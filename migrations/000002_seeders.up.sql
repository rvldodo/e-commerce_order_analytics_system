
CREATE TEMP TABLE _seed_config ON COMMIT DROP AS
SELECT 10000 AS customers,                      
       10    AS avg_orders_per_customer,       
       0.15  AS pct_customers_without_orders, 
       0.05  AS pct_inactive_products,       
       730   AS history_days;               

TRUNCATE order_items, orders, products, categories, customers RESTART IDENTITY;


INSERT INTO categories (name)
VALUES ('Electronics'),
       ('Fashion'),
       ('Home & Kitchen'),
       ('Books'),
       ('Sports & Outdoors'),
       ('Toys & Games'),
       ('Beauty');

INSERT INTO categories (name, parent_id)
SELECT sub.name, parent.id
FROM  (VALUES ('Smartphones',        'Electronics'),
              ('Laptops',            'Electronics'),
              ('Audio',              'Electronics'),
              ('Men''s Clothing',    'Fashion'),
              ('Women''s Clothing',  'Fashion'),
              ('Shoes',              'Fashion'),
              ('Kitchen Appliances', 'Home & Kitchen'),
              ('Furniture',          'Home & Kitchen'),
              ('Fiction',            'Books'),
              ('Non-Fiction',        'Books'),
              ('Fitness',            'Sports & Outdoors'),
              ('Racket Sports',      'Sports & Outdoors'),
              ('Board Games',        'Toys & Games'),
              ('Skincare',           'Beauty')
      ) AS sub (name, parent_name)
JOIN   categories parent ON parent.name = sub.parent_name;

INSERT INTO products (name, category_id, price, is_active, created_at)
SELECT b.base || ' ' || v.variant                                    AS name,
       c.id                                                          AS category_id,
       FLOOR(pc.min_price
             + (pc.max_price - pc.min_price) * (0.1 + 0.6 * random())
               * (1 + 0.2 * (v.variant_no - 1))) + 0.99              AS price,
       random() >= cfg.pct_inactive_products                         AS is_active,
       now() - (cfg.history_days + 30) * INTERVAL '1 day'
             + random() * INTERVAL '30 days'                         AS created_at
FROM  (VALUES
        ('Smartphones',        199, 1199, ARRAY['Galaxy A55','Redmi Note 13','iPhone 15','Pixel 8a','Reno 11','Nord CE4'],            ARRAY['128GB','256GB']),
        ('Laptops',            499, 1899, ARRAY['ThinkPad E14','MacBook Air 13','IdeaPad Slim 5','Vivobook 15','Aspire 7'],          ARRAY['8GB/256GB','16GB/512GB']),
        ('Audio',               19,  349, ARRAY['Wireless Earbuds','Over-Ear Headphones','Bluetooth Speaker','Soundbar'],           ARRAY['Basic','Pro']),
        ('Men''s Clothing',     15,   89, ARRAY['Oxford Shirt','Chino Pants','Polo Shirt','Denim Jacket','Hoodie'],                  ARRAY['S','M','L','XL']),
        ('Women''s Clothing',   15,   99, ARRAY['Midi Dress','Linen Blouse','Wide-Leg Trousers','Cardigan','Batik Kebaya'],          ARRAY['S','M','L']),
        ('Shoes',               25,  149, ARRAY['Running Shoes','Leather Sneakers','Sandals','Loafers'],                             ARRAY['Size 40','Size 41','Size 42','Size 43']),
        ('Kitchen Appliances',  19,  249, ARRAY['Rice Cooker','Air Fryer','Blender','Electric Kettle','Coffee Maker'],               ARRAY['Standard','Deluxe']),
        ('Furniture',           39,  399, ARRAY['Office Chair','Bookshelf','Coffee Table','Floor Lamp'],                             ARRAY['Oak','Walnut','Black']),
        ('Fiction',              8,   35, ARRAY['Laskar Pelangi','Bumi Manusia','Cantik Itu Luka','Pulang','Ronggeng Dukuh Paruk'],  ARRAY['Paperback','Hardcover']),
        ('Non-Fiction',         10,   59, ARRAY['Atomic Habits','Clean Code','Sapiens','Filosofi Teras','The Pragmatic Programmer'], ARRAY['Paperback','Hardcover']),
        ('Fitness',              9,  199, ARRAY['Yoga Mat','Dumbbell Set','Resistance Bands','Jump Rope','Kettlebell'],              ARRAY['Standard','Pro']),
        ('Racket Sports',       12,  229, ARRAY['Badminton Racket','Shuttlecock Tube','Tennis Racket','Table Tennis Paddle'],        ARRAY['Standard','Pro']),
        ('Board Games',          9,  129, ARRAY['Building Blocks Set','Puzzle 1000pcs','Strategy Board Game','Card Game'],           ARRAY['Standard','Deluxe']),
        ('Skincare',             5,   49, ARRAY['Sunscreen SPF50','Facial Cleanser','Moisturizer','Vitamin C Serum','Lip Tint'],     ARRAY['30ml','50ml'])
      ) AS pc (category, min_price, max_price, bases, variants)
JOIN   categories c ON c.name = pc.category
CROSS  JOIN LATERAL unnest(pc.bases)    AS b (base)
CROSS  JOIN LATERAL unnest(pc.variants) WITH ORDINALITY AS v (variant, variant_no)
CROSS  JOIN _seed_config cfg;

CREATE TEMP TABLE _product_weight ON COMMIT DROP AS
SELECT id AS product_id,
       EXP(random() * 3) AS weight
FROM   products;


INSERT INTO customers (email, name, country, created_at)
SELECT LOWER(n.first_name || '.' || n.last_name) || g || '@example.com' AS email,
       n.first_name || ' ' || n.last_name                                AS name,
       n.country,
       now() - random() * cfg.history_days * INTERVAL '1 day'            AS created_at
FROM   _seed_config cfg
CROSS  JOIN LATERAL generate_series(1, cfg.customers) AS g
CROSS  JOIN LATERAL (
    SELECT (ARRAY['Budi','Siti','Agus','Dewi','Rizky','Putri','Andi','Ayu','Fajar','Nur',
                  'Wahyu','Rina','Dimas','Intan','James','Emma','Wei','Mei','Kenji','Yuki',
                  'Liam','Olivia','Arjun','Priya'])[1 + FLOOR(random() * 24)::int] AS first_name,
           (ARRAY['Santoso','Wijaya','Pratama','Saputra','Hidayat','Kusuma','Nugroho','Lestari',
                  'Tan','Lim','Wong','Smith','Johnson','Brown','Tanaka','Sato','Kumar',
                  'Sharma','Nguyen','Garcia'])[1 + FLOOR(random() * 20)::int]         AS last_name,
           CASE WHEN r < 0.50 THEN 'ID'
                WHEN r < 0.62 THEN 'SG'
                WHEN r < 0.74 THEN 'MY'
                WHEN r < 0.84 THEN 'US'
                WHEN r < 0.90 THEN 'AU'
                WHEN r < 0.95 THEN 'JP'
                ELSE NULL
           END AS country
    FROM  (SELECT random() AS r WHERE g > 0) AS rnd
) AS n;


INSERT INTO orders (customer_id, order_date, status, total_amount, updated_at)
SELECT c.id,
       d.order_date,
       s.status,
       0,                 
       CASE WHEN s.status = 'pending' THEN d.order_date
            ELSE LEAST(now(), d.order_date + random() * INTERVAL '3 days')
       END                                                AS updated_at
FROM   customers c
CROSS  JOIN _seed_config cfg
CROSS  JOIN LATERAL (
    SELECT CASE
               WHEN random() < cfg.pct_customers_without_orders THEN 0
               ELSE 1 + FLOOR(-LN(1 - random())
                              * (cfg.avg_orders_per_customer - 1)
                              * CASE WHEN random() < 0.05 THEN 4 ELSE 1 END
                              * 2 * EXTRACT(EPOCH FROM now() - c.created_at)
                                  / (cfg.history_days * 86400.0))::int
           END AS n_orders
    WHERE  c.id > 0
) AS k
CROSS  JOIN LATERAL generate_series(1, k.n_orders) AS g
CROSS  JOIN LATERAL (
    SELECT c.created_at + random() * (now() - c.created_at) AS order_date
    WHERE  g > 0
) AS d
CROSS  JOIN LATERAL (
    SELECT (CASE
                WHEN d.order_date > now() - INTERVAL '2 days' THEN
                    CASE WHEN r < 0.60 THEN 'pending'
                         WHEN r < 0.93 THEN 'completed'
                         ELSE 'cancelled' END
                ELSE
                    CASE WHEN r < 0.87 THEN 'completed'
                         WHEN r < 0.97 THEN 'cancelled'
                         ELSE 'pending' END
            END)::order_status AS status
    FROM  (SELECT random() AS r WHERE d.order_date IS NOT NULL) AS rnd
) AS s
ORDER  BY d.order_date;


INSERT INTO order_items (order_id, product_id, quantity, unit_price)
SELECT o.id,
       p.id,
       x.quantity,
       x.unit_price
FROM   orders o
CROSS  JOIN LATERAL (
    SELECT 1 + FLOOR(POWER(random(), 2) * 5)::int AS n_items
    WHERE  o.id > 0
) AS n
CROSS  JOIN LATERAL (
    SELECT p.id, p.price
    FROM   products p
    JOIN   _product_weight w ON w.product_id = p.id
    WHERE  p.is_active
       OR  o.order_date < now() - INTERVAL '180 days'
    ORDER  BY -LN(1 - random()) / w.weight
    LIMIT  n.n_items
) AS p
CROSS  JOIN LATERAL (
    SELECT CASE WHEN r1 < 0.80 THEN 1
                WHEN r1 < 0.95 THEN 2
                ELSE 3 END                                        AS quantity,
           CASE WHEN r2 < 0.70 THEN p.price
                ELSE ROUND(p.price * (0.80 + 0.15 * r3)::numeric, 2) END   AS unit_price
    FROM  (SELECT random() AS r1, random() AS r2, random() AS r3
           WHERE  p.id > 0) AS rnd
) AS x;


UPDATE orders o
SET    total_amount = t.total
FROM  (
    SELECT order_id, SUM(quantity * unit_price) AS total
    FROM   order_items
    GROUP  BY order_id
) t
WHERE  o.id = t.order_id;

ANALYZE customers;
ANALYZE categories;
ANALYZE products;
ANALYZE orders;
ANALYZE order_items;


SELECT 'categories'  AS table_name, COUNT(*) AS rows FROM categories
UNION ALL SELECT 'products',    COUNT(*) FROM products
UNION ALL SELECT 'customers',   COUNT(*) FROM customers
UNION ALL SELECT 'orders',      COUNT(*) FROM orders
UNION ALL SELECT 'order_items', COUNT(*) FROM order_items;
