# Automation Opportunity Identification

The business team keeps asking for the same reports every day, week and month. Most of what's needed to automate them is already in this project, so this document is less about building something new and more about how I'd connect what's there.

Quick context: the project has two programs. `report` (in `cmd/cli`) is the CLI I use to run reports by hand, and `cron` (in `cmd/cron`) is a worker that runs jobs on a schedule. Both call the same report code underneath.

## The recurring requests

The four scheduled jobs are already registered in [`transport/worker/worker.go`](transport/worker/worker.go). I added the daily sales summary because the CLI can already produce it. For each request there's already a report that answers it:

| Request                   | Schedule (Asia/Jakarta) | Existing report it uses                                      | Output                              |
| ------------------------- | ----------------------- | ------------------------------------------------------------ | ----------------------------------- |
| Daily sales summary       | every day               | `daily_sales_summary` (`report export`)                      | JSON sent to an API (`report send`) |
| Low stock alerts          | every day 07:00         | `inventory_turnover`, rows with status Critical / Low        | JSON / xlsx                         |
| Customer churn risk       | every day 06:00         | `rfm_segmentation`, segments At Risk / Lost                  | xlsx / csv                          |
| Weekly sales report       | Monday 07:00            | `sales_trend --days 7`                                       | xlsx                                |
| Monthly revenue breakdown | 1st of month 06:30      | `product_performance` (last month vs previous, per category) | xlsx                                |

## Approach

The main idea is that a report should only be defined once. The SQL lives in `internal/repository/queries`, the logic around it lives in `internal/usecase/report`, and both the CLI and the cron jobs call the same `Generate(...)`. So if someone gets the weekly report by email and then runs it themselves with the CLI, they'll see the same numbers. That alone removes a lot of "why doesn't this match?" conversations.

Every job then does the same few things: generate the report, save it as a file (xlsx, csv or json) with `pkg/export`, send it with `pkg/sender` if it goes to an API, and log what happened. The only things that change from job to job are which report it runs and where the result goes.

I don't want each job to handle its own failures. If something goes wrong, the job just returns an error, and the scheduler takes care of the rest: it retries up to 3 times with a growing wait between attempts, stops the run if it takes too long, and logs the outcome.

The CLI stays for everything that isn't on a schedule. If someone asks "can I see the cohorts for 2025?", I can answer with `./report get --type customer_cohort --year 2025` without writing any new code. And if someone asks how a number was calculated, `--dry-run` prints the exact SQL.

## Architecture

This is how the pieces fit together today:

```
 cmd/cron  ──▶  cronworker.Manager  (transport/worker/cronworker)
                  │  schedules: DailyAt / WeeklyOn / MonthlyOn in Asia/Jakarta
                  │  queue (256) + worker pool (8 workers)
                  │  per job: timeout 30m, 3 attempts, backoff, skip if still running
                  │  panic recovery, graceful shutdown (20s)
                  ▼
             job handlers  (transport/worker/job)
                  │
                  ▼
             report usecase  (internal/usecase/report)  ◀──  also used by cmd/cli
                  │  Generate / GenerateAll / DailySalesSummary
                  ▼
             repository  (internal/repository/postgresql)
                  │  query logging (time, rows, errors) + file cache
                  ▼
             PostgreSQL   (indexes from Part 1.1)

   output:  pkg/export  → reports/*.xlsx | *.csv | *.json
   deliver: pkg/sender  → HTTPS POST with Bearer token, retries, Idempotency-Key
```

To be honest about where things stand: the job handlers are still stubs that only log a line. Filling them in is mostly calling code that already works in the CLI. The one change is that `job.NewJob` currently gets the repository, and it should get the report usecase (`report.New(repo)`) instead. The low stock alert, for example, would look something like this:

```go
func (js *jobStruct) LowStockAlerts(ctx context.Context) error {
    sheet, err := js.report.Generate(ctx, report.Param{
        Type: report.InventoryTurnover,
        Days: 90,
    })
    if err != nil {
        return err // cronworker retries with backoff
    }

    alerts := keepRows(sheet, "Stock Status", "Critical", "Low")
    if len(alerts.Rows) == 0 {
        return nil // nothing to reorder today
    }

    path := filepath.Join("reports", "low_stock_"+today()+".xlsx")
    return lib.WriteFile(path, export.XLSX, alerts)
}
```

The daily sales summary job would do exactly what these two commands already do, just from inside the worker:

```bash
./report export --date 2024-11-29
./report send --url https://api.example.com/v1/reports --file reports/daily_sales_summary_2024-11-29.json
```

## Technology choices

I stuck with what the project already uses, which is basically Go and PostgreSQL and nothing else to run.

For scheduling, I use the small `cronworker` package in this repo. It does everything these jobs need: calendar schedules in the Jakarta time zone, a pool of workers, timeouts, retries with backoff, skipping a run if the last one is still going, recovering from panics, and shutting down cleanly. I did think about something like Airflow, but for five reports it's a lot more to run and look after than it would save.

For the queue, the scheduler's built-in one is enough: a buffered channel that holds up to 256 runs and feeds the workers. If the process restarts, the scheduled jobs just come back on the next tick, so there's nothing that needs to survive a restart and no reason to add Redis or Kafka.

The rest is:

- **Files:** `pkg/export`, which writes xlsx (using excelize), csv, json or a terminal table.
- **Sending reports:** `pkg/sender`. It posts JSON over HTTPS with a Bearer token and retries when the network or the server has a hiccup (timeouts, 429, 5xx). Every attempt carries the same `Idempotency-Key`, so the other side can ignore a duplicate. It also won't send a token over plain http unless the target is localhost. Anything that accepts a JSON POST can receive our reports this way.
- **Logs:** zap. Every query logs how long it took and how many rows it returned, every job logs whether it worked, and errors are logged with the reason.
- **Cache:** `pkg/cache` keeps query results on disk for a while, so two reports that need the same query don't both hit the database.

## Prioritization

The first one I'd automate is the **daily sales summary**. It's the request that comes up most often (every single day), and it's basically done already: `report export` builds the JSON in the right format, and `report send` delivers it with retries. All that's left is scheduling it. It's also easy to sanity-check: if a number looks off, it's one day of data, and I can compare it with `./report get --type daily_sales_summary --format table` in a few seconds.

Second would be **low stock alerts**. This one probably matters most for the business, because running out of a product that sells well is lost money. The `inventory_turnover` report already works out the stock status and how much to reorder, so the job only has to pick the Critical and Low rows and send them.

After that, the weekly sales and monthly revenue reports. The reports are already there, so it's mostly scheduling and naming the files. I'd leave churn risk for last. The RFM segments exist, but what counts as "at risk" should really be agreed with the team that's going to act on it before we start sending them lists.

## Scalability: 10 reports at the same time

Ten reports at once is something the current setup can already handle.

The cron worker runs 8 workers with a queue that holds 256 runs. If 10 reports are due at the same moment, 8 start straight away and the other 2 wait in the queue until a worker frees up. Nothing gets dropped, and if 8 isn't enough, it's a single setting (`cronworker.WithWorkers`).

Slow reports won't pile up either. Jobs use `Overlap: Skip`, so if a report is still running when its next run comes around, the new run is simply skipped. Each run also has a 30-minute limit, and retries wait a bit longer each time with some randomness added, so several failing jobs don't all retry against the database at the same second.

On the report side, `GenerateAll` already runs a list of reports in parallel with a limit. It's what `./report get --type all --concurrency 4` uses (up to 16), and on the seed data all seven reports finish together in under a second. The cache helps too: if two reports need the same query within 10 minutes, only one of them actually runs it. And the date-range reports use the `(status, order_date)` index from Part 1.1, so they only read the days they need.

There are two things I'd fix before running this for real at a bigger scale:

1. **Put a limit on database connections.** Right now the connection pool has no limit (that's sqlx's default), so 10 heavy reports could open 10 or more connections at the same time. Adding `db.SetMaxOpenConns(...)` in `internal/adapter/postgres`, set to about the number of workers, keeps the load on the database predictable.
2. **Turn on the lock if we ever run more than one `cron` process.** The jobs are already marked `Exclusive`, and `cronworker` has a `Locker` hook for exactly this, but no lock is plugged in yet. With one `cron` process, which is how it runs today, that's fine. With two, every job would run twice.
