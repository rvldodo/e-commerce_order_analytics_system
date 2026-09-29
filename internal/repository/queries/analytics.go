package queries

var (
	CustomerCohortAnalysisQuery = `
	WITH params AS (
		SELECT
			MAKE_DATE($1::int, 1, 1)     AS period_start,
			MAKE_DATE($1::int + 1, 1, 1) AS period_end,
			'Asia/Jakarta'::text  AS tz
	),
	customer_orders AS (
		SELECT
			o.customer_id,
			DATE_TRUNC('month', o.order_date AT TIME ZONE p.tz)::date AS order_month,
			o.total_amount
		FROM
			orders o
			CROSS JOIN params p
		WHERE
			o.status = 'completed'
	),
	first_orders AS (
		SELECT
			customer_id,
			MIN(order_month) AS cohort_month
		FROM
			customer_orders
		GROUP BY
			customer_id
	),
	cohort_members AS (
		SELECT
			f.customer_id,
			f.cohort_month,
			(f.cohort_month + INTERVAL '1 month')::date AS next_month
		FROM
			first_orders f
			CROSS JOIN params p
		WHERE
			f.cohort_month >= p.period_start
			AND f.cohort_month < p.period_end
	),
	member_activity AS (
		-- One row per customer: what they spent in the cohort month and
		-- whether they came back in the month right after it.
		SELECT
			m.customer_id,
			m.cohort_month,
			SUM(co.total_amount) FILTER (WHERE co.order_month = m.cohort_month) AS first_month_revenue,
			BOOL_OR(co.order_month = m.next_month)                               AS ordered_next_month
		FROM
			cohort_members m
			JOIN customer_orders co ON co.customer_id = m.customer_id
		WHERE
			co.order_month <= m.next_month
		GROUP BY
			m.customer_id,
			m.cohort_month
	),
	cohorts AS (
		SELECT
			cohort_month,
			COUNT(*)                                  AS new_customers,
			SUM(first_month_revenue)                  AS first_month_revenue,
			COUNT(*) FILTER (WHERE ordered_next_month) AS retained_customers
		FROM
			member_activity
		GROUP BY
			cohort_month
	)
	SELECT
		TO_CHAR(cohort_month, 'YYYY-MM')                              AS cohort_month,
		new_customers,
		ROUND(first_month_revenue, 2)                                 AS first_month_revenue,
		SUM(new_customers) OVER (
			ORDER BY cohort_month ROWS UNBOUNDED PRECEDING
		)                                                             AS running_total_customers,
		retained_customers,
		ROUND(100.0 * retained_customers / new_customers, 2)          AS retention_rate_pct,
		new_customers - LAG(new_customers) OVER (ORDER BY cohort_month) AS new_customers_change
	FROM
		cohorts
	ORDER BY
		cohorts.cohort_month
	`

	// Query 2: Product Performance with Ranking and Comparison.
	//
	// "Category" is the category a product is assigned to (a sub-category in
	// the seed data); the top-level parent is returned alongside for context.
	// PERCENT_RANK is 0 for the best product and 1 for the worst, so the top
	// 20% of a category are the rows with percent_rank < 0.2.
	ProductPerformanceQuery = `
	WITH params AS (
		SELECT
			'Asia/Jakarta'::text AS tz,
			(DATE_TRUNC('month', now() AT TIME ZONE 'Asia/Jakarta') - INTERVAL '1 month')::date AS last_month,
			(DATE_TRUNC('month', now() AT TIME ZONE 'Asia/Jakarta') - INTERVAL '2 month')::date AS prev_month
	),
	product_sales AS (
		SELECT
			oi.product_id,
			SUM(oi.quantity)                                                  AS units_sold,
			SUM(oi.quantity * oi.unit_price)                                  AS revenue,
			COALESCE(SUM(oi.quantity * oi.unit_price) FILTER (
				WHERE DATE_TRUNC('month', o.order_date AT TIME ZONE p.tz) = p.last_month
			), 0)                                                             AS last_month_revenue,
			COALESCE(SUM(oi.quantity * oi.unit_price) FILTER (
				WHERE DATE_TRUNC('month', o.order_date AT TIME ZONE p.tz) = p.prev_month
			), 0)                                                             AS prev_month_revenue
		FROM
			order_items oi
			JOIN orders o ON o.id = oi.order_id
			CROSS JOIN params p
		WHERE
			o.status = 'completed'
		GROUP BY
			oi.product_id
	),
	ranked AS (
		SELECT
			pr.id                                   AS product_id,
			pr.name                                 AS product_name,
			COALESCE(c.name, 'Uncategorized')       AS category,
			COALESCE(parent.name, c.name, 'Uncategorized') AS parent_category,
			s.units_sold,
			s.revenue,
			RANK() OVER w_category                  AS category_revenue_rank,
			PERCENT_RANK() OVER w_category          AS category_percent_rank,
			s.revenue * 100
				/ NULLIF(SUM(s.revenue) OVER (PARTITION BY pr.category_id), 0) AS category_revenue_share_pct,
			s.last_month_revenue,
			s.prev_month_revenue
		FROM
			product_sales s
			JOIN products pr ON pr.id = s.product_id
			LEFT JOIN categories c ON c.id = pr.category_id
			LEFT JOIN categories parent ON parent.id = c.parent_id
		WINDOW
			w_category AS (PARTITION BY pr.category_id ORDER BY s.revenue DESC)
	)
	SELECT
		r.product_id,
		r.product_name,
		r.category,
		r.parent_category,
		ROUND(r.revenue, 2)                                AS total_revenue,
		r.units_sold,
		r.category_revenue_rank,
		ROUND(r.category_revenue_share_pct, 2)             AS category_revenue_share_pct,
		TO_CHAR(p.last_month, 'YYYY-MM')                   AS last_month,
		ROUND(r.last_month_revenue, 2)                     AS last_month_revenue,
		ROUND(r.prev_month_revenue, 2)                     AS prev_month_revenue,
		ROUND(r.last_month_revenue - r.prev_month_revenue, 2) AS mom_revenue_change,
		ROUND(
			(r.last_month_revenue - r.prev_month_revenue) * 100
				/ NULLIF(r.prev_month_revenue, 0),
			2
		)                                                  AS mom_revenue_change_pct,
		r.category_percent_rank < 0.2                      AS is_top_20_pct_in_category
	FROM
		ranked r
		CROSS JOIN params p
	ORDER BY
		r.category,
		r.category_revenue_rank,
		r.product_id
	`

	// Query 3: Customer Segmentation with RFM Analysis.
	//
	// Only customers with at least one completed order can be scored. The spec
	// fixes the outer bands of recency and frequency; the inner bands are
	// spread so each score is reachable. Monetary uses NTILE(5), so each score
	// holds 20% of the scored customers.
	CustomerRFMSegmentationQuery = `
	WITH params AS (
		SELECT
			'Asia/Jakarta'::text                    AS tz,
			(now() AT TIME ZONE 'Asia/Jakarta')::date AS today
	),
	customer_stats AS (
		SELECT
			c.id                                       AS customer_id,
			c.name,
			c.email,
			MAX(o.order_date AT TIME ZONE p.tz)::date  AS last_order_day,
			COUNT(*)                                   AS frequency,
			SUM(o.total_amount)                        AS monetary
		FROM
			customers c
			JOIN orders o ON o.customer_id = c.id
			CROSS JOIN params p
		WHERE
			o.status = 'completed'
		GROUP BY
			c.id,
			c.name,
			c.email
	),
	scored AS (
		SELECT
			s.*,
			p.today - s.last_order_day AS recency_days,
			CASE
				WHEN p.today - s.last_order_day <= 30  THEN 5
				WHEN p.today - s.last_order_day <= 90  THEN 4
				WHEN p.today - s.last_order_day <= 180 THEN 3
				WHEN p.today - s.last_order_day <= 365 THEN 2
				ELSE 1
			END AS r_score,
			CASE
				WHEN s.frequency >= 20 THEN 5
				WHEN s.frequency >= 10 THEN 4
				WHEN s.frequency >= 6  THEN 3
				WHEN s.frequency >= 3  THEN 2
				ELSE 1
			END AS f_score,
			NTILE(5) OVER (ORDER BY s.monetary, s.customer_id) AS m_score
		FROM
			customer_stats s
			CROSS JOIN params p
	),
	segmented AS (
		SELECT
			*,
			r_score + f_score + m_score AS rfm_score
		FROM
			scored
	)
	SELECT
		customer_id,
		name,
		email,
		recency_days,
		frequency                 AS frequency_count,
		ROUND(monetary, 2)        AS monetary_value,
		r_score,
		f_score,
		m_score,
		rfm_score,
		CASE
			WHEN rfm_score >= 12 THEN 'Champions'
			WHEN rfm_score >= 9  THEN 'Loyal'
			WHEN rfm_score >= 6  THEN 'At Risk'
			ELSE 'Lost'
		END                       AS segment
	FROM
		segmented
	ORDER BY
		rfm_score DESC,
		monetary DESC,
		customer_id
	`

	// Query 4: Advanced Sales Trend Analysis.
	//
	// Covers the last $1 (spec: 90) full days, ending yesterday, because today's partial
	// revenue would always look like a drop. The calendar starts 6 days
	// earlier so the first reported day already has a full 7-day window; the
	// moving averages are trailing (the day itself plus the 6 days before).
	SalesTrendAnalysisQuery = `
	WITH params AS (
		SELECT
			'Asia/Jakarta'::text                          AS tz,
			(now() AT TIME ZONE 'Asia/Jakarta')::date - $1::int AS from_day,
			(now() AT TIME ZONE 'Asia/Jakarta')::date - 1  AS to_day,
			30.0                                          AS anomaly_threshold_pct
	),
	daily_sales AS (
		SELECT
			(o.order_date AT TIME ZONE p.tz)::date AS day,
			COUNT(*)                                AS total_orders,
			SUM(o.total_amount)                     AS revenue
		FROM
			orders o
			CROSS JOIN params p
		WHERE
			o.status = 'completed'
			AND o.order_date >= ((p.from_day - 6)::timestamp AT TIME ZONE p.tz)
			AND o.order_date <  ((p.to_day + 1)::timestamp AT TIME ZONE p.tz)
		GROUP BY
			1
	),
	calendar AS (
		SELECT
			d::date AS day
		FROM
			params p
			CROSS JOIN LATERAL generate_series(p.from_day - 6, p.to_day, INTERVAL '1 day') AS d
	),
	trend AS (
		SELECT
			c.day,
			COALESCE(ds.total_orders, 0)                  AS total_orders,
			COALESCE(ds.revenue, 0)                       AS revenue,
			AVG(COALESCE(ds.revenue, 0)) OVER w_7d        AS revenue_7d_avg,
			AVG(COALESCE(ds.total_orders, 0)) OVER w_7d   AS orders_7d_avg
		FROM
			calendar c
			LEFT JOIN daily_sales ds ON ds.day = c.day
		WINDOW
			w_7d AS (ORDER BY c.day ROWS BETWEEN 6 PRECEDING AND CURRENT ROW)
	),
	scored AS (
		SELECT
			t.*,
			(t.revenue - t.revenue_7d_avg) * 100 / NULLIF(t.revenue_7d_avg, 0) AS revenue_vs_avg_pct
		FROM
			trend t
	)
	SELECT
		s.day,
		TRIM(TO_CHAR(s.day, 'Day'))            AS day_of_week,
		s.total_orders,
		ROUND(s.revenue, 2)                    AS revenue,
		ROUND(s.revenue_7d_avg, 2)             AS revenue_7d_avg,
		ROUND(s.orders_7d_avg, 2)              AS orders_7d_avg,
		ROUND(s.revenue_vs_avg_pct, 2)         AS revenue_vs_avg_pct,
		CASE
			WHEN s.revenue_vs_avg_pct >  p.anomaly_threshold_pct THEN 'Spike'
			WHEN s.revenue_vs_avg_pct < -p.anomaly_threshold_pct THEN 'Drop'
			ELSE 'Normal'
		END                                    AS anomaly_flag
	FROM
		scored s
		CROSS JOIN params p
	WHERE
		s.day >= p.from_day
	ORDER BY
		s.day
	`

	// Query 5: Inventory Turnover and Stock Analysis.
	//
	// products.stock_quantity is defined in 000001 and seeded in 000002.
	//
	// Out-of-stock products that still sell (stock = 0, rate > 0) have 0 days
	// left and are therefore Critical. Status boundaries are half-open:
	// [0,7) Critical, [7,30) Low, [30,90] Adequate, >90 Overstocked.
	InventoryTurnoverQuery = `
	WITH params AS (
		SELECT
			'Asia/Jakarta'::text AS tz,
			$1::int              AS window_days,
			45                   AS target_cover_days,
			((now() AT TIME ZONE 'Asia/Jakarta')::date - $1::int)::timestamp
				AT TIME ZONE 'Asia/Jakarta' AS window_start_ts
	),
	product_sales AS (
		SELECT
			oi.product_id,
			COALESCE(SUM(oi.quantity) FILTER (WHERE o.order_date >= p.window_start_ts), 0) AS units_sold_90d,
			MAX(o.order_date)                                                            AS last_order_at
		FROM
			order_items oi
			JOIN orders o ON o.id = oi.order_id
			CROSS JOIN params p
		WHERE
			o.status = 'completed'
		GROUP BY
			oi.product_id
	),
	metrics AS (
		SELECT
			pr.id                                          AS product_id,
			pr.name                                        AS product_name,
			COALESCE(c.name, 'Uncategorized')              AS category,
			pr.stock_quantity,
			COALESCE(s.units_sold_90d, 0)                  AS units_sold_90d,
			COALESCE(s.units_sold_90d, 0)::numeric / p.window_days AS daily_sales_rate,
			s.last_order_at,
			p.target_cover_days
		FROM
			products pr
			LEFT JOIN product_sales s ON s.product_id = pr.id
			LEFT JOIN categories c ON c.id = pr.category_id
			CROSS JOIN params p
	),
	classified AS (
		SELECT
			m.*,
			m.stock_quantity / NULLIF(m.daily_sales_rate, 0) AS days_until_stockout,
			CASE
				WHEN m.units_sold_90d = 0 AND m.stock_quantity > 0     THEN 'Dead Stock'
				WHEN m.stock_quantity / NULLIF(m.daily_sales_rate, 0) < 7   THEN 'Critical'
				WHEN m.stock_quantity / NULLIF(m.daily_sales_rate, 0) < 30  THEN 'Low'
				WHEN m.stock_quantity / NULLIF(m.daily_sales_rate, 0) <= 90 THEN 'Adequate'
				ELSE 'Overstocked'
			END AS stock_status
		FROM
			metrics m
		WHERE
			m.stock_quantity > 0
			OR m.units_sold_90d > 0
	)
	SELECT
		product_id,
		product_name,
		category,
		stock_quantity,
		units_sold_90d,
		ROUND(daily_sales_rate, 2)                   AS daily_sales_rate,
		ROUND(days_until_stockout, 1)                AS days_until_stockout,
		last_order_at::date                          AS last_order_date,
		stock_status,
		GREATEST(
			CEIL(daily_sales_rate * target_cover_days) - stock_quantity,
			0
		)::int                                       AS reorder_quantity
	FROM
		classified
	ORDER BY
		CASE stock_status
			WHEN 'Critical'    THEN 1
			WHEN 'Low'         THEN 2
			WHEN 'Adequate'    THEN 3
			WHEN 'Overstocked' THEN 4
			WHEN 'Dead Stock'  THEN 5
		END,
		days_until_stockout NULLS LAST,
		product_id
	`

	// Query 6: Customer Purchase Pattern Analysis.
	//
	// Customers are selected by having a completed order in year $1; their
	// patterns are then measured over all of their completed orders, since
	// "lifetime" and "first vs last 3 orders" only make sense on full history.
	// The favourite category is the top-level category with the most line
	// items; ties are all listed, comma-separated.
	CustomerPurchasePatternQuery = `
	WITH params AS (
		SELECT
			'Asia/Jakarta'::text AS tz,
			MAKE_DATE($1::int, 1, 1)::timestamp     AT TIME ZONE 'Asia/Jakarta' AS year_start_ts,
			MAKE_DATE($1::int + 1, 1, 1)::timestamp AT TIME ZONE 'Asia/Jakarta' AS year_end_ts,
			3                    AS min_orders
	),
	customers_in_year AS (
		SELECT DISTINCT
			o.customer_id
		FROM
			orders o
			CROSS JOIN params p
		WHERE
			o.status = 'completed'
			AND o.order_date >= p.year_start_ts
			AND o.order_date <  p.year_end_ts
	),
	sequenced AS (
		SELECT
			o.id AS order_id,
			o.customer_id,
			o.order_date,
			o.total_amount,
			EXTRACT(EPOCH FROM o.order_date - LAG(o.order_date) OVER w_customer) / 86400.0 AS days_since_prev,
			ROW_NUMBER() OVER w_customer                                                   AS order_no,
			COUNT(*) OVER (PARTITION BY o.customer_id)                                     AS total_orders
		FROM
			orders o
			JOIN customers_in_year c ON c.customer_id = o.customer_id
		WHERE
			o.status = 'completed'
		WINDOW
			w_customer AS (PARTITION BY o.customer_id ORDER BY o.order_date, o.id)
	),
	patterns AS (
		SELECT
			s.customer_id,
			COUNT(*)                                                        AS total_orders,
			AVG(s.days_since_prev)                                          AS avg_days_between_orders,
			STDDEV_SAMP(s.days_since_prev)                                  AS stddev_days_between_orders,
			AVG(s.total_amount)                                             AS avg_order_value,
			AVG(s.total_amount) FILTER (WHERE s.order_no <= 3)              AS first_3_avg,
			AVG(s.total_amount) FILTER (WHERE s.order_no > s.total_orders - 3) AS last_3_avg,
			(MAX(s.order_date AT TIME ZONE p.tz)::date
				- MIN(s.order_date AT TIME ZONE p.tz)::date)                AS lifetime_days
		FROM
			sequenced s
			CROSS JOIN params p
		GROUP BY
			s.customer_id
		HAVING
			COUNT(*) >= MIN(p.min_orders)
	),
	category_counts AS (
		SELECT
			s.customer_id,
			COALESCE(parent.name, c.name, 'Uncategorized') AS category,
			COUNT(*)                                       AS line_items,
			RANK() OVER (
				PARTITION BY s.customer_id ORDER BY COUNT(*) DESC
			)                                              AS category_rank
		FROM
			sequenced s
			JOIN patterns pt ON pt.customer_id = s.customer_id
			JOIN order_items oi ON oi.order_id = s.order_id
			JOIN products pr ON pr.id = oi.product_id
			LEFT JOIN categories c ON c.id = pr.category_id
			LEFT JOIN categories parent ON parent.id = c.parent_id
		GROUP BY
			s.customer_id,
			COALESCE(parent.name, c.name, 'Uncategorized')
	),
	favourite_categories AS (
		SELECT
			customer_id,
			STRING_AGG(category, ', ' ORDER BY category) AS top_categories
		FROM
			category_counts
		WHERE
			category_rank = 1
		GROUP BY
			customer_id
	)
	SELECT
		cu.id                                    AS customer_id,
		cu.name,
		cu.email,
		pt.total_orders,
		ROUND(pt.avg_days_between_orders, 1)     AS avg_days_between_orders,
		ROUND(pt.stddev_days_between_orders, 1)  AS stddev_days_between_orders,
		fc.top_categories                        AS most_purchased_category,
		ROUND(pt.avg_order_value, 2)             AS avg_order_value,
		CASE
			WHEN pt.last_3_avg > pt.first_3_avg THEN 'Increasing'
			ELSE 'Decreasing'
		END                                      AS spending_trend,
		pt.lifetime_days
	FROM
		patterns pt
		JOIN customers cu ON cu.id = pt.customer_id
		LEFT JOIN favourite_categories fc ON fc.customer_id = pt.customer_id
	ORDER BY
		pt.total_orders DESC,
		cu.id
	`
)
