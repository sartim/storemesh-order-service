CREATE TABLE IF NOT EXISTS carts (
    customer_id UUID PRIMARY KEY,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cart_lines (
    customer_id UUID NOT NULL REFERENCES carts(customer_id) ON DELETE CASCADE,
    product_id UUID NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (customer_id, product_id)
);
