package cronworker

import (
	"context"
	"e-commerce_order_analytics_system/pkg/logger"
	"errors"
	"fmt"
	"math/rand"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

type Logger interface {
	Debugf(ctx context.Context, format string, v ...interface{})
	Infof(ctx context.Context, format string, v ...interface{})
	Warnf(ctx context.Context, format string, v ...interface{})
	Errorf(ctx context.Context, format string, v ...interface{})
}

type HandlerFunc func(ctx context.Context) error

type Middleware func(HandlerFunc) HandlerFunc

type Schedule interface {
	Next(after time.Time) time.Time
}

type ScheduleFunc func(time.Time) time.Time

func (f ScheduleFunc) Next(t time.Time) time.Time { return f(t) }

func Every(d time.Duration) Schedule {
	return ScheduleFunc(func(t time.Time) time.Time { return t.Add(d) })
}

func DailyAt(hour, min int, loc *time.Location) Schedule {
	return ScheduleFunc(func(t time.Time) time.Time {
		t = t.In(loc)
		next := time.Date(t.Year(), t.Month(), t.Day(), hour, min, 0, 0, loc)
		if !next.After(t) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	})
}

func MonthlyOn(day, hour, min int, loc *time.Location) Schedule {
	return ScheduleFunc(func(t time.Time) time.Time {
		t = t.In(loc)
		next := monthlyCandidate(t.Year(), t.Month(), day, hour, min, loc)
		if !next.After(t) {
			next = monthlyCandidate(t.Year(), t.Month()+1, day, hour, min, loc)
		}
		return next
	})
}

func monthlyCandidate(
	year int,
	month time.Month,
	day, hour, min int,
	loc *time.Location,
) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if day > lastDay {
		day = lastDay
	}
	if day < 1 {
		day = 1
	}
	return time.Date(year, month, day, hour, min, 0, 0, loc)
}

func WeeklyOn(wd time.Weekday, hour, min int, loc *time.Location) Schedule {
	return ScheduleFunc(func(t time.Time) time.Time {
		t = t.In(loc)
		next := time.Date(t.Year(), t.Month(), t.Day(), hour, min, 0, 0, loc)
		delta := (int(wd) - int(next.Weekday()) + 7) % 7
		next = next.AddDate(0, 0, delta)
		if !next.After(t) {
			next = next.AddDate(0, 0, 7)
		}
		return next
	})
}

type OverlapPolicy int

const (
	Skip OverlapPolicy = iota
	Allow
)

var ErrPermanent = errors.New("permanent error")

type Job struct {
	Name     string
	Schedule Schedule
	Handler  HandlerFunc

	Timeout     time.Duration
	MaxAttempts int
	Backoff     time.Duration
	Overlap     OverlapPolicy

	Exclusive bool
	LockTTL   time.Duration
}

type Locker interface {
	Acquire(
		ctx context.Context,
		key string,
		ttl time.Duration,
	) (release func(), acquired bool, err error)
}

type RunInfo struct {
	Job   string
	RunID string
	Tick  time.Time
}

type runCtxKey struct{}

func WithRun(ctx context.Context, ri RunInfo) context.Context {
	return context.WithValue(ctx, runCtxKey{}, ri)
}

func RunFromContext(ctx context.Context) (RunInfo, bool) {
	ri, ok := ctx.Value(runCtxKey{}).(RunInfo)
	return ri, ok
}

func (r RunInfo) tag() string { return fmt.Sprintf("job=%s run=%s", r.Job, r.RunID) }

type jobState struct {
	Job
	inFlight atomic.Bool
	seq      atomic.Uint64
}

type run struct {
	job  *jobState
	info RunInfo
}

type options struct {
	workers        int
	queueSize      int
	log            Logger
	locker         Locker
	mw             []Middleware
	forceStopGrace time.Duration
	tickInterval   time.Duration
	now            func() time.Time
}

type Option func(*options)

func WithWorkers(n int) Option   { return func(o *options) { o.workers = n } }
func WithQueueSize(n int) Option { return func(o *options) { o.queueSize = n } }
func WithLogger(l Logger) Option { return func(o *options) { o.log = l } }
func WithLocker(l Locker) Option { return func(o *options) { o.locker = l } }

func WithMiddleware(m ...Middleware) Option {
	return func(o *options) { o.mw = append(o.mw, m...) }
}

func WithTickInterval(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.tickInterval = d
		}
	}
}

func WithClock(now func() time.Time) Option { return func(o *options) { o.now = now } }

func WithForceStopGrace(d time.Duration) Option {
	return func(o *options) { o.forceStopGrace = d }
}

type Manager struct {
	opts options

	mu      sync.Mutex
	jobs    map[string]*jobState
	order   []*jobState
	started bool
	stopped bool

	queue    chan run
	stopCh   chan struct{}
	jobCtx   context.Context
	cancel   context.CancelFunc
	schedWG  sync.WaitGroup
	workerWG sync.WaitGroup
}

func New(opts ...Option) *Manager {
	o := options{
		workers:        8,
		queueSize:      256,
		log:            logger.Default(),
		forceStopGrace: 5 * time.Second,
		tickInterval:   30 * time.Second,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if o.workers < 1 {
		o.workers = 1
	}
	if o.queueSize < 1 {
		o.queueSize = 1
	}
	return &Manager{opts: o, jobs: map[string]*jobState{}}
}

func (m *Manager) Register(j Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errors.New("cronworker: Register after Start")
	}
	if j.Name == "" || j.Schedule == nil || j.Handler == nil {
		return errors.New("cronworker: Name, Schedule and Handler are required")
	}
	if _, dup := m.jobs[j.Name]; dup {
		return fmt.Errorf("cronworker: duplicate job %q", j.Name)
	}
	if j.MaxAttempts < 1 {
		j.MaxAttempts = 1
	}
	st := &jobState{Job: j}
	m.jobs[j.Name] = st
	m.order = append(m.order, st)
	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return errors.New("cronworker: already started")
	}
	m.started = true
	m.queue = make(chan run, m.opts.queueSize)
	m.stopCh = make(chan struct{})
	m.jobCtx, m.cancel = context.WithCancel(ctx)
	jobs := append([]*jobState(nil), m.order...)
	m.mu.Unlock()

	for i := 0; i < m.opts.workers; i++ {
		m.workerWG.Add(1)
		go m.worker()
	}
	for _, j := range jobs {
		m.schedWG.Add(1)
		go m.schedule(j)
	}
	m.opts.log.Infof(m.jobCtx, "cronworker started workers=%d queue=%d jobs=%d",
		m.opts.workers, m.opts.queueSize, len(jobs))
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started || m.stopped {
		m.mu.Unlock()
		return nil
	}
	m.stopped = true
	m.mu.Unlock()

	close(m.stopCh)
	m.schedWG.Wait()
	close(m.queue)

	done := make(chan struct{})
	go func() { m.workerWG.Wait(); close(done) }()

	select {
	case <-done:
		m.cancel()
		m.opts.log.Infof(ctx, "cronworker stopped cleanly")
		return nil
	case <-ctx.Done():
	}

	m.opts.log.Warnf(ctx, "cronworker shutdown deadline hit, cancelling in-flight jobs")
	m.cancel()
	select {
	case <-done:
		return ctx.Err()
	case <-time.After(m.opts.forceStopGrace):
		return fmt.Errorf("cronworker: workers did not exit within grace period: %w", ctx.Err())
	}
}

func (m *Manager) Trigger(name string) (bool, error) {
	m.mu.Lock()
	j, ok := m.jobs[name]
	started, stopped := m.started, m.stopped
	m.mu.Unlock()
	if !ok {
		return false, fmt.Errorf("cronworker: unknown job %q", name)
	}
	if !started || stopped {
		return false, errors.New("cronworker: not running")
	}
	return m.enqueue(j, time.Now()), nil
}

func (m *Manager) schedule(j *jobState) {
	defer m.schedWG.Done()
	next := j.Schedule.Next(m.clock())

	for {
		now := m.clock()
		if !next.After(now) {
			m.enqueue(j, next)
			n := j.Schedule.Next(now)
			if !n.After(now) {
				n = now.Add(time.Second)
			}
			next = n
			continue
		}

		wait := next.Sub(now)
		if wait > m.opts.tickInterval {
			wait = m.opts.tickInterval
		}
		timer := time.NewTimer(wait)
		select {
		case <-m.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (m *Manager) enqueue(j *jobState, tick time.Time) bool {
	if j.Overlap == Skip && !j.inFlight.CompareAndSwap(false, true) {
		m.opts.log.Warnf(m.jobCtx, "job=%s tick skipped, previous run still in flight", j.Name)
		return false
	}
	r := run{job: j, info: RunInfo{
		Job:   j.Name,
		RunID: fmt.Sprintf("%s-%d", j.Name, j.seq.Add(1)),
		Tick:  tick,
	}}
	select {
	case m.queue <- r:
		return true
	default:
		if j.Overlap == Skip {
			j.inFlight.Store(false)
		}
		m.opts.log.Errorf(m.jobCtx, "job=%s tick dropped, queue full cap=%d", j.Name, cap(m.queue))
		return false
	}
}

func (m *Manager) worker() {
	defer m.workerWG.Done()
	for r := range m.queue {
		m.execute(r)
	}
}

func (m *Manager) execute(r run) {
	j := r.job
	if j.Overlap == Skip {
		defer j.inFlight.Store(false)
	}

	ctx := WithRun(m.jobCtx, r.info)
	tag := r.info.tag()

	if j.Exclusive && m.opts.locker != nil {
		release, ok, err := m.opts.locker.Acquire(ctx, "cronlock:"+j.Name, j.lockTTL())
		if err != nil {
			m.opts.log.Errorf(ctx, "%s lock acquisition failed: %v", tag, err)
			return
		}
		if !ok {
			m.opts.log.Debugf(ctx, "%s lock held by another replica, skipping", tag)
			return
		}
		defer release()
	}

	h := chain(recovery(), m.opts.mw...)(j.Handler)

	start := time.Now()
	err := m.runWithRetry(ctx, j, h)
	dur := time.Since(start)

	switch {
	case err == nil:
		m.opts.log.Infof(ctx, "%s ok dur=%s lag=%s", tag, dur, start.Sub(r.info.Tick))
	case errors.Is(err, context.Canceled):
		m.opts.log.Warnf(ctx, "%s aborted by shutdown dur=%s", tag, dur)
	default:
		m.opts.log.Errorf(ctx, "%s failed dur=%s: %v", tag, dur, err)
	}
}

func (m *Manager) runWithRetry(ctx context.Context, j *jobState, h HandlerFunc) error {
	var err error
	for attempt := 1; attempt <= j.MaxAttempts; attempt++ {
		attemptCtx := ctx
		cancel := context.CancelFunc(func() {})
		if j.Timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, j.Timeout)
		}
		err = h(attemptCtx)
		cancel()

		if err == nil {
			return nil
		}
		if errors.Is(err, ErrPermanent) || ctx.Err() != nil || attempt == j.MaxAttempts {
			return err
		}
		wait := backoff(j.Backoff, attempt)
		ri, _ := RunFromContext(ctx)
		m.opts.log.Warnf(ctx, "%s attempt %d/%d failed, retrying in %s: %v",
			ri.tag(), attempt, j.MaxAttempts, wait, err)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (m *Manager) clock() time.Time {
	if m.opts.now != nil {
		return m.opts.now()
	}
	return time.Now()
}

func (j *jobState) lockTTL() time.Duration {
	if j.LockTTL > 0 {
		return j.LockTTL
	}
	if j.Timeout > 0 {
		return j.Timeout*time.Duration(j.MaxAttempts) + 30*time.Second
	}
	return 5 * time.Minute
}

func backoff(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if attempt > 20 {
		attempt = 20
	}
	d := base << (attempt - 1)
	if d > 5*time.Minute || d <= 0 {
		d = 5 * time.Minute
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

func chain(inner Middleware, mws ...Middleware) Middleware {
	return func(h HandlerFunc) HandlerFunc {
		h = inner(h)
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}

func recovery() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
				}
			}()
			return next(ctx)
		}
	}
}

func Observe(record func(job string, dur time.Duration, err error)) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context) error {
			start := time.Now()
			err := next(ctx)
			ri, _ := RunFromContext(ctx)
			record(ri.Job, time.Since(start), err)
			return err
		}
	}
}
