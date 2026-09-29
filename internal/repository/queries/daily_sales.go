package queries

// DailySalesSummaryQuery returns exactly one row summarising a single calendar
// day in Asia/Jakarta, even when the day has no orders.
//
//	$1 day (YYYY-MM-DD)
//
// Only completed orders count, like every other report. The top category is
// the top-level category (sub-categories rolled up to their parent) with the
// highest line-item revenue that day; ties go to the alphabetically first.
// Money columns are cast to text after ROUND so the caller keeps the exact
// two-decimal value (125000.50, not 125000.5).
var DailySalesSummaryQuery = `
	WITH params AS (
		SELECT
			$1::date                                                AS day,
			($1::date::timestamp       AT TIME ZONE 'Asia/Jakarta') AS start_ts,
			(($1::date + 1)::timestamp AT TIME ZONE 'Asia/Jakarta') AS end_ts
	),
	day_orders AS (
		SELECT
			o.id,
			o.total_amount
		FROM
			orders o
			CROSS JOIN params p
		WHERE
			o.status = 'completed'
			AND o.order_date >= p.start_ts
			AND o.order_date < p.end_ts
	),
	totals AS (
		SELECT
			COUNT(*)                          AS total_orders,
			COALESCE(SUM(total_amount), 0)    AS total_revenue
		FROM
			day_orders
	),
	category_revenue AS (
		SELECT
			COALESCE(parent.name, c.name, 'Uncategorized') AS category,
			SUM(oi.quantity * oi.unit_price)               AS revenue
		FROM
			day_orders d
			JOIN order_items oi ON oi.order_id = d.id
			JOIN products pr ON pr.id = oi.product_id
			LEFT JOIN categories c ON c.id = pr.category_id
			LEFT JOIN categories parent ON parent.id = c.parent_id
		GROUP BY
			1
	)
	SELECT
		t.total_orders,
		ROUND(t.total_revenue, 2)::text                                        AS total_revenue,
		ROUND(COALESCE(t.total_revenue / NULLIF(t.total_orders, 0), 0), 2)::text AS average_order_value,
		(
			SELECT category
			FROM category_revenue
			ORDER BY revenue DESC, category
			LIMIT 1
		)                                                                      AS top_category
	FROM
		totals t
	`
