package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogLevel = zapcore.Level

const (
	DEBUG LogLevel = zapcore.DebugLevel
	INFO  LogLevel = zapcore.InfoLevel
	WARN  LogLevel = zapcore.WarnLevel
	ERROR LogLevel = zapcore.ErrorLevel
	FATAL LogLevel = zapcore.FatalLevel
)

var (
	Log   *zap.Logger
	Sugar *zap.SugaredLogger

	level zap.AtomicLevel

	callerLog *zap.Logger

	openLog *zap.Logger

	defaultLogger = &ZapLogger{isDefault: true}
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	l, err := cfg.Build()
	if err != nil {
		setLoggers(zap.NewNop(), zap.NewAtomicLevel())
		return
	}
	setLoggers(l, cfg.Level)
}

func Init(env string) error {
	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	// Errors here are expected outcomes (bad input, missing column), so a Go
	// stack trace on every one only buries the message.
	cfg.DisableStacktrace = true
	l, err := cfg.Build()
	if err != nil {
		return err
	}
	setLoggers(l, cfg.Level)
	return nil
}

func setLoggers(l *zap.Logger, lvl zap.AtomicLevel) {
	Log = l
	Sugar = l.Sugar()
	callerLog = l.WithOptions(zap.AddCallerSkip(2))
	openLog = callerLog.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return allLevelsCore{c}
	}))
	level = lvl
}

func SetLevel(lvl LogLevel) { level.SetLevel(lvl) }

func SetPrefix(prefix string) { defaultLogger.SetPrefix(prefix) }

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

type allLevelsCore struct{ zapcore.Core }

func (c allLevelsCore) Enabled(zapcore.Level) bool { return true }

func (c allLevelsCore) With(fields []zapcore.Field) zapcore.Core {
	return allLevelsCore{c.Core.With(fields)}
}

func (c allLevelsCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, c)
}

type Logger interface {
	Debug(ctx context.Context, args ...interface{})
	Debugf(ctx context.Context, format string, v ...interface{})
	Info(ctx context.Context, args ...interface{})
	Infof(ctx context.Context, format string, v ...interface{})
	Warn(ctx context.Context, args ...interface{})
	Warnf(ctx context.Context, format string, v ...interface{})
	Error(ctx context.Context, args ...interface{})
	Errorf(ctx context.Context, format string, v ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, v ...interface{})
}

var _ Logger = (*ZapLogger)(nil)

type ZapLogger struct {
	isDefault bool
	level     zap.AtomicLevel
	prefix    atomic.Pointer[string]
}

func Default() *ZapLogger { return defaultLogger }

func New() *ZapLogger {
	return &ZapLogger{level: zap.NewAtomicLevelAt(INFO)}
}

func (l *ZapLogger) SetLevel(lvl LogLevel) {
	if l.isDefault {
		SetLevel(lvl)
		return
	}
	l.level.SetLevel(lvl)
}

func (l *ZapLogger) Level() LogLevel {
	if l.isDefault {
		return level.Level()
	}
	return l.level.Level()
}

func (l *ZapLogger) SetPrefix(prefix string) { l.prefix.Store(&prefix) }

func (l *ZapLogger) Enabled(lvl LogLevel) bool {
	if l.isDefault {
		return callerLog.Core().Enabled(lvl)
	}
	return l.level.Enabled(lvl)
}

func (l *ZapLogger) Debug(ctx context.Context, args ...interface{}) {
	write(l, ctx, zapcore.DebugLevel, "", args)
}
func (l *ZapLogger) Debugf(ctx context.Context, format string, v ...interface{}) {
	write(l, ctx, zapcore.DebugLevel, format, v)
}
func (l *ZapLogger) Info(ctx context.Context, args ...interface{}) {
	write(l, ctx, zapcore.InfoLevel, "", args)
}
func (l *ZapLogger) Infof(ctx context.Context, format string, v ...interface{}) {
	write(l, ctx, zapcore.InfoLevel, format, v)
}
func (l *ZapLogger) Warn(ctx context.Context, args ...interface{}) {
	write(l, ctx, zapcore.WarnLevel, "", args)
}
func (l *ZapLogger) Warnf(ctx context.Context, format string, v ...interface{}) {
	write(l, ctx, zapcore.WarnLevel, format, v)
}
func (l *ZapLogger) Error(ctx context.Context, args ...interface{}) {
	write(l, ctx, zapcore.ErrorLevel, "", args)
}
func (l *ZapLogger) Errorf(ctx context.Context, format string, v ...interface{}) {
	write(l, ctx, zapcore.ErrorLevel, format, v)
}

func (l *ZapLogger) Fatal(args ...interface{}) {
	write(l, context.Background(), zapcore.FatalLevel, "", args)
}

func (l *ZapLogger) Fatalf(format string, v ...interface{}) {
	write(l, context.Background(), zapcore.FatalLevel, format, v)
}

func (l *ZapLogger) Print(v ...interface{}) {
	write(l, context.Background(), zapcore.InfoLevel, "", v)
}
func (l *ZapLogger) Printf(format string, v ...interface{}) {
	write(l, context.Background(), zapcore.InfoLevel, format, v)
}
func (l *ZapLogger) Println(v ...interface{}) {
	write(l, context.Background(), zapcore.InfoLevel, "%s",
		[]interface{}{strings.TrimSuffix(fmt.Sprintln(v...), "\n")})
}

func Debug(ctx context.Context, args ...interface{}) {
	write(defaultLogger, ctx, zapcore.DebugLevel, "", args)
}
func Debugf(ctx context.Context, format string, v ...interface{}) {
	write(defaultLogger, ctx, zapcore.DebugLevel, format, v)
}
func Info(ctx context.Context, args ...interface{}) {
	write(defaultLogger, ctx, zapcore.InfoLevel, "", args)
}
func Infof(ctx context.Context, format string, v ...interface{}) {
	write(defaultLogger, ctx, zapcore.InfoLevel, format, v)
}
func Warn(ctx context.Context, args ...interface{}) {
	write(defaultLogger, ctx, zapcore.WarnLevel, "", args)
}
func Warnf(ctx context.Context, format string, v ...interface{}) {
	write(defaultLogger, ctx, zapcore.WarnLevel, format, v)
}
func Error(ctx context.Context, args ...interface{}) {
	write(defaultLogger, ctx, zapcore.ErrorLevel, "", args)
}
func Errorf(ctx context.Context, format string, v ...interface{}) {
	write(defaultLogger, ctx, zapcore.ErrorLevel, format, v)
}
func Fatal(args ...interface{}) {
	write(defaultLogger, context.Background(), zapcore.FatalLevel, "", args)
}
func Fatalf(format string, v ...interface{}) {
	write(defaultLogger, context.Background(), zapcore.FatalLevel, format, v)
}

func write(
	l *ZapLogger,
	ctx context.Context,
	lvl zapcore.Level,
	template string,
	args []interface{},
) {
	out := callerLog
	if !l.isDefault {
		out = openLog
	}
	if lvl < zapcore.FatalLevel && !l.Enabled(lvl) {
		return
	}

	var msg string
	switch {
	case template == "":
		msg = fmt.Sprint(args...)
	case len(args) == 0:
		msg = template
	default:
		msg = fmt.Sprintf(template, args...)
	}
	if p := l.prefix.Load(); p != nil && *p != "" {
		msg = *p + msg
	}

	if ce := out.Check(lvl, msg); ce != nil {
		ce.Write(contextFields(ctx)...)
	}
}

type ctxKey string

const (
	RequestID  ctxKey = "requestID"
	UserID     ctxKey = "userID"
	partnerID  ctxKey = "partnerID"
	FuncName   ctxKey = "funcName"
	Device     ctxKey = "device"
	AppVersion ctxKey = "appVersion"
)

func SetCtx(ctx context.Context, key ctxKey, value string) context.Context {
	return context.WithValue(ctx, key, value)
}

func GetCtx(ctx context.Context, key ctxKey) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(key).(string)
	return v
}

func SetRequestIDCtx(ctx context.Context) context.Context {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return SetCtx(ctx, RequestID, hex.EncodeToString(b))
}

func GetRequestIDCtx(ctx context.Context) string { return GetCtx(ctx, RequestID) }

func SetUserIDCtx(ctx context.Context, uid int64) context.Context {
	return SetCtx(ctx, UserID, fmt.Sprintf("%d", uid))
}

func GetUserIDCtx(ctx context.Context) string { return GetCtx(ctx, UserID) }

func SetPartnerIDCtx(ctx context.Context, pid string) context.Context {
	return SetCtx(ctx, partnerID, pid)
}

func GetPartnerIDCtx(ctx context.Context) string { return GetCtx(ctx, partnerID) }

func SetDeviceCtx(ctx context.Context, device string) context.Context {
	return SetCtx(ctx, Device, device)
}

func GetDeviceCtx(ctx context.Context) string { return GetCtx(ctx, Device) }

func SetAppVersionCtx(ctx context.Context, appVersion string) context.Context {
	return SetCtx(ctx, AppVersion, appVersion)
}

func GetAppVersionCtx(ctx context.Context) string { return GetCtx(ctx, AppVersion) }

func SetFuncNameCtx(ctx context.Context) context.Context {
	return SetCtx(ctx, FuncName, GetCallerFuncName(2))
}

func SetFuncNameHandlerCtx(ctx context.Context) context.Context {
	return SetCtx(ctx, FuncName, GetCallerFuncName(3))
}

func GetFuncNameCtx(ctx context.Context) string { return GetCtx(ctx, FuncName) }

func GetCallerFuncName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	details := runtime.FuncForPC(pc)
	if details == nil {
		return ""
	}
	parts := strings.Split(details.Name(), ".")
	return parts[len(parts)-1]
}

func contextFields(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}
	fields := make([]zap.Field, 0, 6)

	if v := GetRequestIDCtx(ctx); v != "" {
		fields = append(fields, zap.String("requestID", v))
	}
	if v := GetUserIDCtx(ctx); v != "" {
		fields = append(fields, zap.String("userID", v))
	}
	if v := GetPartnerIDCtx(ctx); v != "" {
		fields = append(fields, zap.String("partnerID", v))
	}
	if v := GetFuncNameCtx(ctx); v != "" {
		fields = append(fields, zap.String("funcName", v))
	}
	if v := GetDeviceCtx(ctx); v != "" {
		fields = append(fields, zap.String("device", v))
	}
	if v := GetAppVersionCtx(ctx); v != "" {
		fields = append(fields, zap.String("appVersion", v))
	}
	return fields
}

func WithContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Log
	}
	return Log.With(contextFields(ctx)...)
}

func SugarWithContext(ctx context.Context) *zap.SugaredLogger {
	return WithContext(ctx).Sugar()
}
