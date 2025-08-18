CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    number TEXT NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'NEW',
    accrual DECIMAL(10,2) DEFAULT 0,
    uploaded_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED', 'WITHDRAWN'))
    );

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);