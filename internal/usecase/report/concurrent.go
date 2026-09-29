package report

import (
	"context"
	"e-commerce_order_analytics_system/pkg/export"
	"sync"
	"time"
)

const MaxConcurrency = 16

type Result struct {
	Param    Param
	Sheet    export.Sheet
	Err      error
	Duration time.Duration
}

// GenerateAll runs the reports with at most concurrency queries in flight.
// Results keep the order of params, and one failing report does not cancel
// the others.
func (rs *reportStruct) GenerateAll(ctx context.Context, params []Param, concurrency int) []Result {
	concurrency = min(max(concurrency, 1), MaxConcurrency)
	results := make([]Result, len(params))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for i, p := range params {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[i] = Result{Param: p, Err: ctx.Err()}
				return
			}
			defer func() { <-sem }()

			start := time.Now()
			sheet, err := rs.Generate(ctx, p)
			results[i] = Result{Param: p, Sheet: sheet, Err: err, Duration: time.Since(start)}
		}()
	}
	wg.Wait()
	return results
}
