-- NOTE: BEFORE Initial Tables Given to Improve

-- =================================================================================

-- -- Customers table
-- CREATE TABLE customers (
--     id SERIAL PRIMARY KEY,
--     email VARCHAR(255) UNIQUE NOT NULL,
--     name VARCHAR(255) NOT NULL,
--     country VARCHAR(100),
--     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
-- );
--
-- -- Products table
-- CREATE TABLE products (
--     id SERIAL PRIMARY KEY,
--     name VARCHAR(255) NOT NULL,
--     category VARCHAR(100),
--     price DECIMAL(10,2) NOT NULL,
--     stock_quantity INT DEFAULT 0
-- );
--
-- -- Orders table
-- CREATE TABLE orders (
--     id SERIAL PRIMARY KEY,
--     customer_id INT REFERENCES customers(id),
--     order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
--     status VARCHAR(50), -- 'pending', 'completed', 'cancelled'
--     total_amount DECIMAL(10,2)
-- );
--
-- -- Order items table
-- CREATE TABLE order_items (
--     id SERIAL PRIMARY KEY,
--     order_id INT REFERENCES orders(id),
--     product_id INT REFERENCES products(id),
--     quantity INT NOT NULL,
--     unit_price DECIMAL(10,2) NOT NULL
-- );

-- =================================================================================

-- TODO: AFTER Tables Improved

CREATE TYPE order_status AS ENUM ('pending', 'completed', 'cancelled');

CREATE TABLE customers (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email       VARCHAR(255) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    country     CHAR(2), 
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_customers_email   CHECK (email LIKE '%_@_%'),
    CONSTRAINT chk_customers_country CHECK (country ~ '^[A-Z]{2}$')
);

CREATE UNIQUE INDEX idx_customers_email_lower ON customers (LOWER(email));

CREATE TABLE categories (
    id         INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    parent_id  INT REFERENCES categories(id),

    CONSTRAINT uq_categories_name UNIQUE (name)
);
 
CREATE INDEX idx_categories_parent ON categories (parent_id);

CREATE TABLE products (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         VARCHAR(255)  NOT NULL,
    category_id  INT REFERENCES categories(id),
    price        NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    is_active    BOOLEAN       NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_category ON products (category_id) WHERE is_active;

CREATE TABLE orders (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id   BIGINT        NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    order_date    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    status        order_status  NOT NULL DEFAULT 'pending',
    total_amount  NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
 
CREATE INDEX idx_orders_customer_date
    ON orders (customer_id, order_date DESC);
 
CREATE INDEX idx_orders_status_date
    ON orders (status, order_date) INCLUDE (total_amount, customer_id);
 
CREATE INDEX idx_orders_updated_at ON orders (updated_at);
 
CREATE INDEX idx_orders_pending
    ON orders (order_date) WHERE status = 'pending';

CREATE TABLE order_items (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id    BIGINT        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id  BIGINT        NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    quantity    INT           NOT NULL CHECK (quantity > 0),
    unit_price  NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),

    CONSTRAINT uq_order_items_order_product UNIQUE (order_id, product_id)
);
 
CREATE INDEX idx_order_items_product
    ON order_items (product_id) INCLUDE (quantity, unit_price, order_id);
