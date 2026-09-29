package lib

const USAGE = `report - export e-commerce order analytics reports from PostgreSQL

Usage:
  report get [flags]       Generate one or more reports (database connection is read from .env)
  report export [flags]    Export one day's sales summary (JSON by default)
  report send [flags]      POST a JSON file to an API endpoint
  report types             List available report types and the flags they use
  report help              Show this help

Flags for get:
  --type         Report type, comma-separated list or "all" (default: sales_trend, see "report types")
  --year         Year for customer_cohort and purchase_patterns (default: 2024)
  --days         Window in days for sales_trend and inventory_turnover (default: 90)
  --date         Day for daily_sales_summary, YYYY-MM-DD (default: yesterday)
  --format       xlsx, csv, json or table (default: xlsx; table prints to the terminal)
  --out          Output file, "-" for stdout; a directory when several reports run
                 (default: reports/<type>[_<year|days|date>]_<today>.<format>)
  --limit        Keep only the first N rows, 0 = all (default: 0)
  --concurrency  Reports generated in parallel (default: 4, max: 16)
  --dry-run      Print the SQL and its parameters without executing it
  --no-cache     Skip cached results and refresh them

Flags for export:
  --date         Day to summarise, YYYY-MM-DD (default: yesterday)
  --format       json, csv, xlsx or table (default: json)
  --out          Output file, "-" for stdout (default: reports/daily_sales_summary_<date>.<format>)
  --dry-run      Print the SQL and its parameters without executing it
  --no-cache     Skip cached results and refresh them

Flags for send:
  --url          API endpoint (https; plain http only for localhost)
  --token        Bearer token (default: $REPORT_API_TOKEN)
  --file         JSON file to send, e.g. one written by "report export"
  --retries      Extra attempts on network errors, 408, 429 and 5xx (default: 3)
  --timeout      Per-attempt timeout (default: 30s)

Environment:
  REPORT_CACHE_TTL   How long query results are cached, 0 disables (default: 10m)
  REPORT_CACHE_DIR   Where cached results are stored (default: .cache/report)

Logs (query time, rows returned, errors) go to stderr; report output goes to stdout or files.
Days and months are evaluated in Asia/Jakarta. Only completed orders count.

Examples:
  report get
  report get --type customer_cohort --year 2025
  report get --type rfm_segmentation --format csv --out - | head
  report get --type product_performance --format json --limit 20
  report get --type all --concurrency 4
  report get --type sales_trend,rfm_segmentation --format table --limit 5
  report get --type customer_cohort --dry-run

  report export --date 2024-11-29
  report export --date 2024-11-29 --format csv --out -

  report send --url https://api.example.com/v1/reports --file reports/daily_sales_summary_2024-11-29.json
`
