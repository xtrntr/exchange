-- Creates the database schema for users, orders, and trades
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    type VARCHAR(4) NOT NULL CHECK (type IN ('buy', 'sell')),
    price DECIMAL(10, 2) NOT NULL,
    quantity DECIMAL(10, 8) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'filled', 'canceled')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE trades (
    id SERIAL PRIMARY KEY,
    buy_order_id INT REFERENCES orders(id),
    sell_order_id INT REFERENCES orders(id),
    price DECIMAL(10, 2) NOT NULL,
    quantity DECIMAL(10, 8) NOT NULL,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
); 