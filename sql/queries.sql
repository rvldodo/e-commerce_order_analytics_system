-- =============================================================================
-- Query 1: Customer Cohort Analysis with Running Totals
-- =============================================================================
--
-- Approach
--   1. customer_orders  - completed orders with their Jakarta order month.
--   2. first_orders     - each customer's first-ever order month = cohort.
--   3. cohort_members   - keep only 2024 cohorts; precompute next_month.
--   4. member_activity  - one row per customer: revenue in the cohort month
--                         and whether they ordered again in next_month.
--   5. cohorts          - roll up to one row per cohort month.
--   6. final SELECT     - running total with SUM() OVER, retention %, and
--                         month-over-month change in new customers with LAG().
--
-- Why these techniques
--   * The cohort is MIN(order_month) over ALL history, not just 2024, so a
--     customer who first ordered in 2023 is never counted as "new" in 2024.
--   * SUM(...) OVER (ORDER BY cohort_month ROWS UNBOUNDED PRECEDING) gives the
--     running total in one pass; ROWS (not the default RANGE) is explicit and
--     cheaper.
--   * FILTER (WHERE ...) and BOOL_OR compute "first-month revenue" and
--     "came back next month" in the same GROUP BY instead of two extra joins.
--
-- Optimization
--   * member_activity only reads orders up to next_month
--     (co.order_month <= m.next_month), not the customer's whole history.
--
-- Assumptions
--   * "Only include data from 2024" = cohorts whose first order is in 2024.
--     Retention for the December 2024 cohort necessarily looks at
--     January 2025 orders.
--   * Retention = % of the cohort with at least one completed order in the
--     calendar month right after the cohort month.

WITH params AS (
	SELECT
		MAKE_DATE(2024::int /* year */, 1, 1)     AS period_start,
		MAKE_DATE(2024::int /* year */ + 1, 1, 1) AS period_end,
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
	cohorts.cohort_month;


-- =============================================================================
-- Query 2: Product Performance with Ranking and Comparison
-- =============================================================================
--
-- Approach
--   1. params         - last completed month and the month before it,
--                       derived from now() in Asia/Jakarta.
--   2. product_sales  - one pass over completed order items: all-time units
--                       and revenue, plus last/previous month revenue using
--                       conditional aggregation (FILTER).
--   3. ranked         - join products/categories and apply the window
--                       functions per category.
--   4. final SELECT   - MoM change and %, top-20% flag, sorted by category
--                       and rank.
--
-- Why these techniques
--   * RANK() gives 1 = highest revenue; ties share a rank, which is fair
--     for equal revenue.
--   * PERCENT_RANK() is (rank - 1) / (rows - 1): 0 for the best product and
--     1 for the worst, so "top 20% of its category" is percent_rank < 0.2.
--   * SUM(revenue) OVER (PARTITION BY category_id) gives each category's
--     total without a second GROUP BY and join.
--   * A named WINDOW (w_category) keeps RANK and PERCENT_RANK on exactly
--     the same ordering.
--   * FILTER computes three revenue figures in one scan instead of three
--     separate subqueries.
--
-- Optimization
--   * Aggregate order_items first (to one row per product) and only then
--     join products and categories.
--
-- Assumptions
--   * "Category" is the category the product is assigned to (a
--     sub-category such as Smartphones); the parent (Electronics) is shown
--     too. Ranking is within the assigned category.
--   * "Last completed month" = the calendar month before the current one.
--     MoM % is NULL when the previous month had no revenue.
--   * Products with no completed sales are excluded by the inner join from
--     product_sales, as required.

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
	r.product_id;


-- =============================================================================
-- Query 3: Customer Segmentation with RFM Analysis
-- =============================================================================
--
-- Approach
--   1. customer_stats - per customer: last order day, completed order count,
--                       lifetime revenue.
--   2. scored         - R and F scores with CASE bands, M score with NTILE(5).
--   3. segmented      - rfm_score = R + F + M (3..15).
--   4. final SELECT   - segment label from the combined score.
--
-- Why these techniques
--   * Recency and frequency use fixed business thresholds, so CASE is the
--     right tool. Monetary is relative ("top 20%"), so NTILE(5) over revenue
--     puts exactly a fifth of customers in each score.
--   * The customer_id tie-breaker in NTILE's ORDER BY makes the result
--     deterministic when customers have the same revenue.
--   * Separate CTEs keep each step readable, and rfm_score is computed once
--     and reused by both the label and the ORDER BY.
--
-- Optimization
--   * One GROUP BY over completed orders; customers is joined by primary key.
--
-- Assumptions
--   * The task only fixes the outer bands, so the middle ones are mine:
--       Recency (days):  <=30 -> 5, <=90 -> 4, <=180 -> 3, <=365 -> 2, else 1
--       Frequency:       >=20 -> 5, >=10 -> 4, >=6 -> 3, >=3 -> 2, 1-2 -> 1
--   * Recency is measured from the last COMPLETED order, like frequency and
--     monetary, so all three describe the same set of orders.
--   * Customers without any completed order cannot be scored and are left
--     out.

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
	customer_id;


-- =============================================================================
-- Query 4: Advanced Sales Trend Analysis
-- =============================================================================
--
-- Approach
--   1. params      - the 90-day window (ending yesterday) and the 30% threshold.
--   2. daily_sales - completed orders and revenue per Jakarta day.
--   3. calendar    - generate_series() of every day, so days with no orders
--                    still appear (with 0).
--   4. trend       - LEFT JOIN calendar -> daily_sales, then 7-day moving
--                    averages with AVG() OVER (ROWS BETWEEN 6 PRECEDING
--                    AND CURRENT ROW).
--   5. scored      - % difference between the day and its moving average.
--   6. final SELECT - day of week and Spike / Drop / Normal flag.
--
-- Why these techniques
--   * generate_series + LEFT JOIN is the standard way to keep empty days.
--     COALESCE(..., 0) happens BEFORE the window function, so empty days
--     pull the average down as they should.
--   * ROWS BETWEEN 6 PRECEDING counts rows, and because the calendar has
--     exactly one row per day, 7 rows = 7 days.
--   * The calendar starts 6 days before the window, so the first reported
--     day already has a full 7-day average; those extra days are filtered
--     out at the end.
--
-- Optimization
--   * The filter status = 'completed' AND order_date in [start, end) matches
--     idx_orders_status_date (status, order_date) INCLUDE (total_amount,
--     customer_id), so this can run as an index-only scan over ~96 days.
--
-- Assumptions
--   * "Last 90 days" = the 90 full days ending yesterday. Today is still in
--     progress and would always be flagged as a drop.
--   * The moving average includes the day itself (a trailing 7-day window).

WITH params AS (
	SELECT
		'Asia/Jakarta'::text                          AS tz,
		(now() AT TIME ZONE 'Asia/Jakarta')::date - 90::int /* days */ AS from_day,
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
	s.day;


-- =============================================================================
-- Query 5: Inventory Turnover and Stock Analysis
-- =============================================================================
--
-- Approach
--   1. params         - 90-day window and 45-day target cover.
--   2. product_sales  - per product: units sold in the window (FILTER) and
--                       the last order time (all history).
--   3. metrics        - LEFT JOIN from products so unsold products are kept;
--                       daily sales rate = units / 90.
--   4. classified     - days until stock-out and the stock status CASE.
--   5. final SELECT   - reorder quantity and priority sort.
--
-- Why these techniques
--   * The CASE checks "Dead Stock" first, because with zero sales the days
--     until stock-out is undefined (NULLIF turns the division into NULL
--     instead of an error).
--   * One aggregation returns both the 90-day units and the all-time last
--     order date, instead of two passes over order_items.
--   * Sorting by a CASE priority puts the urgent statuses first; NULLS LAST
--     keeps Dead Stock (no stock-out date) at the bottom.
--
-- Optimization
--   * Aggregate order_items once per product before joining products.
--
-- Assumptions
--   * products.stock_quantity is the current stock (defined in migration
--     000001, seeded in 000002).
--   * Status bands: < 7 Critical, 7 to < 30 Low, 30 to 90 Adequate,
--     > 90 Overstocked. A product with 0 stock that still sells has 0 days
--     left, so it is Critical.
--   * Reorder quantity = units needed for 45 days at the current rate minus
--     the stock on hand, never below 0.
--   * Last order date = the last completed order that contained the product.

WITH params AS (
	SELECT
		'Asia/Jakarta'::text AS tz,
		90::int /* days */              AS window_days,
		45                   AS target_cover_days,
		((now() AT TIME ZONE 'Asia/Jakarta')::date - 90::int /* days */)::timestamp
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
	product_id;


-- =============================================================================
-- Query 6: Customer Purchase Pattern Analysis
-- =============================================================================
--
-- Approach
--   1. customers_in_year    - customers with a completed order in 2024.
--   2. sequenced            - their completed orders in time order with LAG()
--                             (days since previous order), ROW_NUMBER() and
--                             the order count per customer.
--   3. patterns             - averages, STDDEV_SAMP of the gaps, first-3 and
--                             last-3 order averages, lifetime in days;
--                             HAVING keeps customers with >= 3 orders.
--   4. category_counts      - line items per top-level category, ranked per
--                             customer.
--   5. favourite_categories - STRING_AGG of the top category (ties included).
--   6. final SELECT         - trend label and sort by order count.
--
-- Why these techniques
--   * LAG() over (PARTITION BY customer ORDER BY order_date) gives the gap
--     between consecutive orders in one pass; a self-join on
--     "the next order" would be far more expensive.
--   * ROW_NUMBER() plus the per-customer COUNT(*) OVER makes "first 3" and
--     "last 3" simple FILTERs on order_no.
--   * STDDEV_SAMP (sample standard deviation) measures how regular the
--     customer is; a low value means they order on a steady rhythm.
--   * RANK() instead of ROW_NUMBER() for categories, so ties are kept and
--     STRING_AGG lists them all ("Electronics, Fashion").
--
-- Optimization
--   * The expensive category join (orders -> order_items -> products ->
--     categories) runs only for customers that passed the >= 3 orders
--     filter, by joining category_counts to patterns.
--
-- Assumptions
--   * Customers are selected by ordering in 2024, but their patterns use all
--     their completed orders: "lifetime" and "first vs last 3 orders" only
--     make sense over the full history.
--   * "Most frequently purchased category" = top-level category with the
--     most order lines.
--   * The task's rule is used as-is: if the last-3 average is not greater
--     than the first-3 average, the trend is "Decreasing" (this includes
--     equal values).

WITH params AS (
	SELECT
		'Asia/Jakarta'::text AS tz,
		MAKE_DATE(2024::int /* year */, 1, 1)::timestamp     AT TIME ZONE 'Asia/Jakarta' AS year_start_ts,
		MAKE_DATE(2024::int /* year */ + 1, 1, 1)::timestamp AT TIME ZONE 'Asia/Jakarta' AS year_end_ts,
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
	cu.id;
