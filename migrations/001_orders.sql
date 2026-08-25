CREATE TABLE IF NOT EXISTS orders (
    order_id UUID PRIMARY KEY,
    customer_id UUID NOT NULL,
    total_minor BIGINT NOT NULL CHECK (total_minor >= 0),
    currency TEXT NOT NULL,
    status SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS order_lines (
    order_id UUID NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL,
    product_id UUID NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    unit_price_minor BIGINT NOT NULL CHECK (unit_price_minor >= 0),
    PRIMARY KEY (order_id, line_number)
);

CREATE INDEX IF NOT EXISTS orders_customer_idx ON orders (customer_id, created_at DESC);
