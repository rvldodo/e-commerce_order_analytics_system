# Screening Test: Data Automation & Retrieval Engineer (PostgreSQL / Go)

## Prerequisites

- [Go](https://go.dev/dl/) 1.26+
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) — `brew install golang-migrate`

---

## Instructions

### Time Allocation

This test is designed to be completed until **friday, 2 October** (1 week).

### Submission Requirements

1. Create a **public GitHub repository** for your solutions
2. Include a comprehensive `README.md` explaining your approach and decisions
3. Organize your code with a clear folder structure
4. Add comments explaining your logic where appropriate
5. Include any SQL scripts, Go code, and documentation
6. Submit the GitHub repository link when complete

### Evaluation Criteria

- **Complex SQL query writing (PRIMARY FOCUS – 45%)**
- PostgreSQL query optimization and performance
- Go programming proficiency
- Problem-solving approach
- Code quality and structure
- Documentation and communication

> [!IMPORTANT]
> This role requires someone who can write complex SQL queries involving multiple tables, advanced aggregations, window functions, CTEs, and optimization. The SQL portion is weighted heavily in this assessment.

---

## Scenario: E-Commerce Order Analytics System

You are working for an e-commerce company that needs help retrieving and automating order data analysis. The company has a **PostgreSQL** database containing **orders**, **customers**, and **products**. Business teams frequently request complex data extracts and analytical reports.

---

## Part 1

### 1.1 Schema Analysis

The original schema is kept (commented out) at the top of [`migrations/000001_initial-tables.up.sql`](migrations/000001_initial-tables.up.sql), and my improved version is right below it.

#### Performance Issues

With 1M+ customers, 10M+ orders and 50M+ order items, these are the problems I'd expect:

- **No indexes on foreign keys.** Postgres doesn't create them automatically, so `orders.customer_id`, `order_items.order_id` and `order_items.product_id` have none. Looking up one customer's orders, or joining orders to their items, ends up scanning millions of rows.
- **No index on `order_date` or `status`.** Most reports ask for "completed orders in this date range", and without an index every one of them reads all 10M orders.
- **`SERIAL` IDs are only 32-bit** (max ~2.1 billion). `order_items` grows fastest, and switching it to `BIGINT` later means rewriting a huge table.
- **`status` is free text.** Nothing stops `'Completed'`, `'complete'` or `NULL`, so reports can silently miss rows.
- **Important columns can be NULL** (`customer_id`, `order_date`, `status`, `total_amount`), so we can end up with orders that have no customer or no date.
- **`TIMESTAMP` has no time zone**, so "which day did this order happen" depends on the server setting.
- **Email is unique only case-sensitively.** `A@x.com` and `a@x.com` can both exist, and a `LOWER(email)` lookup can't use the index.
- **Category is just text on `products`.** Typos split the numbers, and there's no way to model sub-categories.

#### Index Recommendations

1. **`orders (status, order_date) INCLUDE (total_amount, customer_id)`**: most reports filter completed orders by date. This index goes straight to that date range, and because it includes `total_amount`, daily sales totals can be read from the index alone.
2. **`orders (customer_id, order_date DESC)`**: covers the customer foreign key and makes "this customer's latest orders" instant.
3. **`order_items (product_id) INCLUDE (quantity, unit_price, order_id)`**: covers the product foreign key and answers "how much did this product sell" without touching the big table.
4. **`order_items UNIQUE (order_id, product_id)`**: stops the same product being added twice to an order, and because `order_id` comes first it also speeds up every join from orders to items.
5. **`customers UNIQUE (LOWER(email))`**: makes email lookup case-insensitive and fast, and blocks duplicate accounts that differ only in letter case.

One thing to keep in mind: indexes help when a query reads a small slice of a table. Reports over the full history (RFM, all-time product ranking) read almost every completed order anyway, so a full scan is normal there. At a bigger scale, those would benefit more from partitioning or pre-computed summary tables.

#### Schema Improvements

1. **Better types and constraints.** `BIGINT` identity keys, `TIMESTAMPTZ` for dates, an `order_status` enum instead of free text, `NOT NULL` on required columns, and `CHECK` rules such as price, quantity and stock can't be negative.
2. **A proper `categories` table.** Products point to `category_id`, and categories have a `parent_id` so sub-categories (e.g. Smartphones → Electronics) roll up correctly. I also added `is_active` on products, so old products can be hidden without deleting them.
3. **Safer data rules.** Case-insensitive unique email, one row per product per order, and an `updated_at` on orders to make incremental syncs easier.

If the data keeps growing, the next step I'd take is partitioning `orders` and `order_items` by month.

### 1.2 Complex SQL Queries

All six queries live in [`internal/repository/queries/analytics.go`](internal/repository/queries/analytics.go). That's the version the CLI actually runs, with the year or number of days passed in as `$1`.

For reviewing them as plain SQL, [`sql/queries.sql`](sql/queries.sql) has the same six queries with detailed comments (approach, why each technique, optimization notes, assumptions) and the task's values (2024, 90 days) already filled in, so it runs as-is:

```bash
psql -d <database> -f sql/queries.sql
```

Or run them through the CLI:

```bash
./report get --type customer_cohort --format table
./report get --type customer_cohort --dry-run   # print the SQL without running it
```

| #   | Query                    | In `analytics.go`                                                               | CLI `--type`          |
| --- | ------------------------ | ------------------------------------------------------------------------------- | --------------------- |
| 1   | Customer Cohort Analysis | [`CustomerCohortAnalysisQuery`](internal/repository/queries/analytics.go#L4)    | `customer_cohort`     |
| 2   | Product Performance      | [`ProductPerformanceQuery`](internal/repository/queries/analytics.go#L93)       | `product_performance` |
| 3   | RFM Segmentation         | [`CustomerRFMSegmentationQuery`](internal/repository/queries/analytics.go#L176) | `rfm_segmentation`    |
| 4   | Sales Trend Analysis     | [`SalesTrendAnalysisQuery`](internal/repository/queries/analytics.go#L262)      | `sales_trend`         |
| 5   | Inventory Turnover       | [`InventoryTurnoverQuery`](internal/repository/queries/analytics.go#L341)       | `inventory_turnover`  |
| 6   | Purchase Patterns        | [`CustomerPurchasePatternQuery`](internal/repository/queries/analytics.go#L432) | `purchase_patterns`   |

A few things apply to all of them:

- Only **completed** orders count, so "revenue" and "orders" mean the same thing in every report.
- Days and months are in **Asia/Jakarta** time, since `order_date` is stored with a time zone.
- Each query starts with a small `params` CTE holding its inputs (dates, thresholds), then goes step by step with CTEs. Postgres inlines them, so they're as fast as subqueries but much easier to read.
- Date filters compare the raw `order_date` column against a range, so the `(status, order_date)` index can be used.

A short note on each:

1. **Cohort analysis.** A customer's cohort is the month of their _first ever_ order, so someone who started in 2023 isn't counted as new in 2024. `SUM() OVER` gives the running total and `LAG()` the change from the previous cohort. Retention for the December cohort looks at January 2025, since that's the only way to answer "did they come back the next month".
2. **Product performance.** Revenue is summed per product first, then ranked inside its category with `RANK()`. `PERCENT_RANK()` finds the top 20%, and `SUM() OVER (PARTITION BY category)` gives the category share. "Last completed month" means the calendar month before the current one.
3. **RFM.** Recency and frequency use fixed bands with `CASE`, and monetary uses `NTILE(5)` so each score holds exactly 20% of customers. The task only defined the top and bottom bands, so I picked the middle ones (e.g. recency 30/90/180/365 days).
4. **Sales trend.** `generate_series` builds every day in the range so days with no orders still show up as 0. The 7-day averages use `AVG() OVER (ROWS BETWEEN 6 PRECEDING AND CURRENT ROW)`. I use the 90 full days ending yesterday, because today is still in progress and would always look like a drop.
5. **Inventory.** The daily rate is units sold in the last 90 days / 90, and days until stock-out is stock / daily rate. "Dead Stock" is checked first, since with no sales the stock-out date can't be calculated. Reorder quantity is what's needed for 45 days of stock, minus what's on hand.
6. **Purchase patterns.** `LAG()` gives the days between consecutive orders, and `STDDEV_SAMP` of those gaps shows how regular a customer is. `ROW_NUMBER()` picks out the first and last 3 orders. When categories tie for most purchased, `STRING_AGG` lists them all. Customers are picked by ordering in 2024, but their stats use their whole history.

---

## Part 2

### Report CLI

I built a small CLI called `report` so the queries from Part 1 can be run without opening psql. It reads the database connection from `.env`, runs the report you ask for, and saves the result as a file (Excel, CSV or JSON) or prints it in the terminal.

Build it once:

```bash
make build-cli        # or: go build -o ./report ./cmd/cli
./report help
```

There are six commands:

```
report get       run one or more reports
report export    export one day's sales summary (JSON by default)
report send      POST a JSON file to an API
report push      build the daily sales summary and POST it in one step
report types     list the available reports
report help      show all flags
```

#### Getting reports

`report get` runs a report. With no flags it runs the sales trend for the last 90 days and saves it as an Excel file in `reports/`:

```bash
./report get
# -> reports/sales_trend_90d_2026-09-30.xlsx
```

Pick the report with `--type`. These are the ones available (same as `./report types`):

| Type                  | What it shows                                                  | Extra flag                   |
| --------------------- | -------------------------------------------------------------- | ---------------------------- |
| `customer_cohort`     | New customers per month, first-month revenue, retention        | `--year` (default 2024)      |
| `product_performance` | Revenue per product, rank in category, month-over-month change |                              |
| `rfm_segmentation`    | RFM score and segment for each customer                        |                              |
| `sales_trend`         | Daily revenue with 7-day averages and anomaly flags            | `--days` (default 90)        |
| `inventory_turnover`  | Stock level, days until stock-out, reorder quantity            | `--days` (default 90)        |
| `purchase_patterns`   | Order gaps, favourite category, spending trend                 | `--year` (default 2024)      |
| `daily_sales_summary` | Revenue, orders, AOV and top category for one day              | `--date` (default yesterday) |

If you pass a flag that the report doesn't use (like `--year` with `rfm_segmentation`), the CLI tells you instead of silently ignoring it.

Some examples I use a lot:

```bash
# look at something quickly in the terminal
./report get --type product_performance --format table --limit 20

# cohorts for another year
./report get --type customer_cohort --year 2025

# CSV straight to stdout, handy for piping
./report get --type rfm_segmentation --format csv --out - | head

# save to a specific file
./report get --type sales_trend --days 30 --format json --out trend.json
```

`--format` can be `xlsx`, `csv`, `json` or `table`. `--limit` keeps only the first N rows, which is useful with RFM since it has one row per customer.

#### Running several reports at once

`--type` also accepts a comma-separated list, or `all`. The reports run in parallel (4 at a time by default, change it with `--concurrency`) and each one gets its own file:

```bash
./report get --type all --format csv
./report get --type sales_trend,rfm_segmentation --out exports/
```

When you run more than one report, `--out` is treated as a folder. If one report fails, the others still finish, and the command exits with code 1 at the end with a list of what failed.

#### Seeing the SQL without running it

Add `--dry-run` to print the exact SQL and its parameters. It doesn't need a database connection, so it's a quick way to copy a query into psql or check what's going to run:

```bash
./report get --type customer_cohort --year 2025 --dry-run
```

#### Caching

Query results are cached on disk for 10 minutes, so running the same report twice in a row doesn't hit the database again. The cache also resets every day, since some reports depend on today's date (like "last 90 days"). You can change this in `.env`:

```bash
REPORT_CACHE_TTL=10m          # set to 0 to turn caching off
REPORT_CACHE_DIR=.cache/report
```

To force fresh data for one run, use `--no-cache`.

#### Logs

Every command logs what it did to stderr: which query ran, how long it took, how many rows came back, and any errors. The actual report goes to stdout or a file, so piping still works cleanly. A normal run looks like this:

```
INFO  query executed    {"query": "SalesTrendAnalysisQuery", "duration": "54ms", "rows": 90}
INFO  report generated  {"report": "sales_trend", "rows": 90, "duration": "57ms", "output": "reports/sales_trend_90d_2026-09-30.xlsx"}
INFO  command finished  {"command": "get", "duration": "59ms"}
```

When a result comes from the cache, the log says `query served from cache` instead.

#### Daily sales summary as JSON

`report export` builds a small JSON summary for one day. This is the format other systems can consume:

```bash
./report export --date 2024-11-29
# -> reports/daily_sales_summary_2024-11-29.json
```

```json
{
  "report_type": "daily_sales",
  "date": "2024-11-29",
  "data": {
    "total_revenue": 4541.9,
    "total_orders": 15,
    "average_order_value": 302.79,
    "top_category": "Electronics"
  },
  "generated_at": "2026-09-30T02:41:06Z"
}
```

Without `--date` it uses yesterday, because today isn't finished yet. It also supports `--format csv`, `xlsx` or `table` if you'd rather have it that way.

#### Sending a report to an API

`report send` posts a JSON file to an endpoint with a Bearer token:

```bash
export REPORT_API_TOKEN=your-token
./report send --url https://api.example.com/v1/reports \
  --file reports/daily_sales_summary_2024-11-29.json
```

You can pass `--token` directly too, but I'd rather use the env variable so the token doesn't end up in shell history. A few things it does for you:

- checks the file is valid JSON before sending anything
- retries on network errors, timeouts, 429 and 5xx (3 retries by default, `--retries` to change)
- sends an `Idempotency-Key` header, so if a retry reaches the server twice it can tell it's the same report
- refuses plain `http://` unless it's localhost, so the token is never sent unencrypted

A 401 or 404 isn't retried, since trying again won't fix a wrong token or URL.

If you just want to push the daily summary without a file in between, `report push` does both steps at once (more on that in [3.3](#33-rest-api-integration)).

#### How the code is organised

```
cmd/cli/main.go                     entry point, opens the DB only when a command needs it
transport/command_line/command      picks the command from the arguments
transport/command_line/handler      one file per command (get, export, send, types, help)
transport/command_line/lib          flag parsing and file writing
internal/usecase/report             report logic, dry-run plans, concurrent runs
internal/repository/postgresql      runs the queries, logging, cache layer
internal/repository/queries         the SQL itself
pkg/export                          xlsx / csv / json / table writers
pkg/cache                           file cache with expiry
pkg/sender                          HTTP client with retries
```

Tests run without a database (they use fakes), so `go test ./...` is enough to check everything.

---

## Part 3

### 3.1 Automation Design

The full design is in [`AUTOMATION_DESIGN.md`](AUTOMATION_DESIGN.md). In short:

Most of what's needed is already in the project, so the plan is mainly about connecting the pieces.

Every recurring request maps to a report the CLI already has, and the cron jobs call the same report code as the CLI. That way a scheduled report and one I run by hand always show the same numbers. The `cron` program uses the small `cronworker` scheduler in this repo, which already handles schedules, retries, timeouts and a pool of 8 workers. I didn't add anything like Airflow or Redis, because five reports don't need it.

I'd start with the daily sales summary. It's needed every day, and `report export` + `report send` already do the whole job; it just needs to be scheduled. Low stock alerts would come next, since running out of stock costs money.

For 10 reports at once: 8 run right away and 2 wait in the queue, slow runs don't pile up, and the cache stops the same query from running twice. Before using it at a bigger scale, I'd put a limit on database connections and turn on the scheduler's lock if we ever run more than one `cron` process.

### 3.2 Slow Query Investigation

A business user complained that this query takes about 45 seconds. The database has 2 million customers, 15 million orders, and no indexes except the primary keys.

```sql
SELECT
    c.name,
    c.email,
    COUNT(o.id) as order_count,
    SUM(o.total_amount) as total_revenue
FROM customers c
LEFT JOIN orders o ON c.id = o.customer_id
WHERE o.order_date >= '2024-01-01'
GROUP BY c.id, c.name, c.email
HAVING COUNT(o.id) > 5
ORDER BY total_revenue DESC;
```

#### Root cause

The biggest problem is that nothing helps Postgres find the 2024 orders. There's no index on `order_date`, so it has to read all 15 million orders and throw away everything before 2024. That's most of the 45 seconds, and it only gets worse as the table grows.

On top of that, the query makes Postgres do more work than it needs to:

- It joins every 2024 order to its customer first and only then counts. So it drags `name` and `email` along for millions of rows and groups on those text columns, when `customer_id` alone would do. That grouping needs a lot of memory, and when it doesn't fit in `work_mem`, Postgres spills it to disk, which is slow.
- The `LEFT JOIN` doesn't do anything here. The `WHERE` on `o.order_date` already throws away customers without orders, so it's really a normal join. It just makes the query harder to read.
- The `HAVING COUNT > 5` filter comes last, so all the joining and grouping is also done for customers we throw away at the end.
- There's no `LIMIT`, so every matching customer gets sorted by revenue, even if the user only looks at the top of the list.

It's also worth running `ANALYZE orders;`, because outdated statistics can make Postgres pick a bad plan.

#### Quick fixes

These can go in today without changing the schema.

1. **Add an index that fits the query:**

   ```sql
   CREATE INDEX CONCURRENTLY idx_orders_order_date_cover
       ON orders (order_date) INCLUDE (customer_id, total_amount);
   ```

   Postgres can jump straight to 2024-01-01 instead of reading the whole table. And because `customer_id` and `total_amount` are stored in the index too, it doesn't need to touch the table at all. `CONCURRENTLY` means the table isn't locked while the index is being built.

   I'd also add an index on `orders (customer_id)`. It won't speed up this query, but it's a foreign key that every "orders of this customer" lookup needs, and right now it has nothing.

2. **Count first, then join.** Group the orders by `customer_id` only, drop the customers with 5 or fewer orders right away, and only then look up names and emails for the customers who are left. That's the rewrite below.

3. **Give it more memory** if the plan still shows the grouping or the sort spilling to disk: `SET work_mem = '256MB';` for that session.

4. **Ask what the user actually needs.** If they only look at the top 100, `LIMIT 100` saves sorting everyone. And cancelled orders are counted as revenue right now, which probably isn't what they want.

#### Long-term solutions

- **Proper indexes and types.** The improved schema from 1.1 already fixes most of this: indexes on `orders (customer_id, order_date DESC)` and `orders (status, order_date) INCLUDE (total_amount, customer_id)`, `TIMESTAMPTZ` dates, and an enum for `status`.
- **A summary table.** Keep one row per customer per month (order count and revenue), refreshed every night by the cron worker. Then this report reads a small table and answers in milliseconds instead of going through millions of orders every time. A materialized view works too.
- **Partition `orders` by month.** A "since 2024" query then only reads the 2024+ partitions and skips the rest. It also makes archiving old orders much easier.
- **Keep heavy reports off the main database.** Run them on a read replica with a `statement_timeout`, so one slow report can't slow down the shop.
- **Stop running it by hand.** If the business checks this often, it should be a scheduled report like the ones in 3.1, instead of a 45-second query someone runs whenever they need it.

#### Optimized query

```sql
-- Count and sum per customer first, only from orders.
-- With idx_orders_order_date_cover this reads just the 2024+ part of the index.
WITH customer_totals AS (
    SELECT
        customer_id,
        COUNT(*)          AS order_count,
        SUM(total_amount) AS total_revenue
    FROM orders
    WHERE order_date >= DATE '2024-01-01'
    GROUP BY customer_id
    HAVING COUNT(*) > 5          -- drop small customers before joining
)
-- Only the customers left over get their name and email looked up.
SELECT
    c.name,
    c.email,
    t.order_count,
    t.total_revenue
FROM customer_totals t
JOIN customers c ON c.id = t.customer_id
ORDER BY t.total_revenue DESC;
```

What's different:

- It groups on `customer_id` only, one number instead of three columns including two text ones, so it needs much less memory.
- `HAVING` runs before the join, so the join only handles customers with more than 5 orders.
- A plain `JOIN` instead of the `LEFT JOIN`, because that's what the query actually does.
- `COUNT(*)` instead of `COUNT(o.id)`. It's the same number since `id` is never NULL, just slightly cheaper.

The results don't change. I ran both versions on the project's database: both return 4,341 customers, and comparing them row by row shows no differences.

If the business confirms that only completed orders count and they only need the top customers, I'd add `status = 'completed'` to the `WHERE` and a `LIMIT 100`. That version fits the `(status, order_date)` index in our improved schema exactly.

### 3.3 REST API Integration

The BI platform wants the daily sales summary pushed to `POST https://api.bi-platform.com/v1/reports` with a Bearer token. I added a `report push` command that does the whole thing in one go: it runs the daily summary query, builds the JSON, and sends it.

```bash
./report push --date 2024-11-29 \
  --url https://api.bi-platform.com/v1/reports \
  --token <token>
```

It sends exactly the payload the platform asked for:

```json
{
  "report_type": "daily_sales",
  "date": "2024-11-29",
  "data": {
    "total_revenue": 125000.5,
    "total_orders": 450,
    "average_order_value": 277.78,
    "top_category": "Electronics"
  },
  "generated_at": "2024-11-29T08:00:00Z"
}
```

A few things worth knowing:

- Without `--date` it pushes **yesterday**, because today isn't over yet. That's what you want for a daily job.
- The URL and token can come from `.env` instead of flags (`REPORT_API_URL`, `REPORT_API_TOKEN`), so the token never shows up in a crontab line or in shell history.
- Money keeps two decimals (`125000.50`, not `125000.5`), and `generated_at` is always in UTC.
- If the API is down or busy (network error, timeout, 408, 429, 5xx), it retries 3 times with a growing wait. You can change that with `--retries` and `--timeout`.
- If the API says no (401, 403, 404, 422…), it stops right away and tells you why, e.g. "Unauthorized (401, check --token)". Retrying won't fix a wrong token.
- Every attempt sends the same `Idempotency-Key`, so if a retry goes through twice, the platform can tell it's the same report.
- It won't send the token over plain `http://` (except to localhost for testing).
- If anything fails, it exits with code 1 and logs the reason, so whatever scheduled it knows it failed.

To see what it's going to run without touching the database or the API, add `--dry-run`.

#### Scheduling it

The simplest way is a normal crontab entry on the server where the project lives. This pushes yesterday's summary every morning at 08:00 Jakarta time:

```cron
CRON_TZ=Asia/Jakarta
0 8 * * * cd /opt/e-commerce_order_analytics_system && ./report push >> /var/log/report-push.log 2>&1
```

It reads the database settings and `REPORT_API_URL` / `REPORT_API_TOKEN` from `.env` in that folder, so the crontab line doesn't need any secrets. The log file keeps the output of every run: which query ran, how long it took, and the API's status code, with the reason if something went wrong. (`CRON_TZ` works with cronie, the cron on most Linux servers. If your cron doesn't support it, set the server to Asia/Jakarta or convert the time to UTC: 08:00 WIB is 01:00 UTC.)

If a run fails, for example because the API was down for longer than the retries cover, you can push that day again by hand:

```bash
./report push --date 2024-11-29
```

Because of the idempotency key, sending the same day twice is safe.

Other ways to run it, depending on where the project is deployed:

- **The `cron` worker in this project.** It already has a daily schedule in Asia/Jakarta, retries and timeouts (see 3.1). The daily summary would be one more job that calls the same code as `report push`. That's the better option once there are several scheduled reports, because they're all managed in one place.
- **Kubernetes.** A `CronJob` with `schedule: "0 8 * * *"`, `timeZone: Asia/Jakarta` and `./report push` as the command, with the token stored in a Secret.
- **systemd timer.** If you'd rather not use cron, a timer with `OnCalendar=*-*-* 08:00:00 Asia/Jakarta` running the same command does the job. `journalctl` then keeps the logs.

---

## Running Locally

You need Go 1.26+, Docker, and [golang-migrate](https://github.com/golang-migrate/migrate) (`brew install golang-migrate`). Everything below is run from the project root.

### 1. Get the code

```bash
git clone <repo-url>
cd e-commerce_order_analytics_system
go mod download
```

### 2. Set up `.env`

```bash
cp .env.example .env
```

Then fill in the database part. The values have to match the Postgres user and password you create in the next step:

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=ecommerce_order_analytics_system
DB_SSL_MODE=disable

# same database, as a URL, used by the migration tool
DB_MIGRATOR_ADDR="postgresql://postgres:postgres@localhost:5432/ecommerce_order_analytics_system?sslmode=disable"
```

The rest can stay as it is for now. `REPORT_API_URL` and `REPORT_API_TOKEN` are only needed for `report send` and `report push`, and the cache settings already have sensible defaults.

### 3. Create the Docker secrets

The Postgres container reads its user and password from two files in `./secrets/`. Use the same values as `DB_USER` and `DB_PASSWORD` in your `.env`:

```bash
mkdir -p secrets
echo "postgres" > secrets/user.txt
echo "postgres" > secrets/password.txt
```

### 4. Start Postgres

```bash
docker compose up -d
docker compose ps        # wait until it says "healthy"
```

The first time it starts, `sql/init.sql` creates the `ecommerce_order_analytics_system` database. That only happens when the data volume is empty. If you change the user or password later, run `docker compose down -v` to start from a clean volume.

### 5. Create the tables and sample data

```bash
make migration-up
```

This creates the tables and fills them with sample data (about 10,000 customers and two years of orders), so it can take a little while. `make migration-version` should print `2` when it's done.

To start over with fresh data, roll everything back and run it again:

```bash
make migration-down      # answer "y" when it asks
make migration-up
```

### 6. Run the CLI

**Without building anything**, use `go run`. It compiles in the background and runs straight away, which is the quickest way to try things out:

```bash
go run ./cmd/cli help
go run ./cmd/cli types
go run ./cmd/cli get --type sales_trend --format table
go run ./cmd/cli get --type customer_cohort --year 2024 --format csv --out -
go run ./cmd/cli export --date 2024-11-29 --out -
```

Anything you can do with `./report`, you can do with `go run ./cmd/cli` followed by the same command and flags.

**If you're going to use it a lot**, build the binary once. It starts faster and you can copy it to a server:

```bash
make build-cli           # or: go build -o ./report ./cmd/cli
./report help
./report get --type all --format csv
```

There's also `make run-cli`, which builds and runs in one go. Pass the arguments through `ARGS`:

```bash
make run-cli ARGS="get --type rfm_segmentation --format table --limit 10"
```

Run it from the project root either way. It reads `.env` from the current folder and saves reports into `./reports/`.

### 7. Run the scheduler (optional)

```bash
go run ./cmd/cron        # without building
# or
make run-cron            # builds bin/cron, then runs it
```

It keeps running and fires the scheduled jobs at their times (Asia/Jakarta). Stop it with Ctrl+C; jobs that are still running get up to 20 seconds to finish before it exits.

### 8. Run the tests

```bash
go test ./...            # or: make test (verbose)
```

The tests don't need the database, so they work even before step 4.

### If something doesn't work

- **`connection refused`**: Postgres isn't up yet. Check `docker compose ps`, and wait for "healthy".
- **`password authentication failed`**: the values in `.env` don't match `secrets/`. Fix them, then `docker compose down -v && docker compose up -d` (this wipes the local database) and run the migrations again.
- **`relation "orders" does not exist`**: the migrations haven't run yet (step 5).
- **`inventory report needs products.stock_quantity`**: your database was created before the stock column was added. Run `make migration-down` and `make migration-up` to rebuild it.
- **Old numbers after reseeding**: results are cached for 10 minutes. Add `--no-cache`, or delete `.cache/`.
