-- 000001_init.up.sql

-- Enable UUID extension if needed
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Table: price_ticks (High-frequency tick data)
CREATE TABLE IF NOT EXISTS price_ticks (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    price NUMERIC(18, 8) NOT NULL,
    volume NUMERIC(18, 8) NOT NULL,
    trade_id BIGINT NOT NULL,
    is_buyer_maker BOOLEAN NOT NULL DEFAULT FALSE,
    timestamp TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexing strategy for time-series range queries on price_ticks
CREATE INDEX IF NOT EXISTS idx_price_ticks_symbol_timestamp 
    ON price_ticks (symbol, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_price_ticks_trade_id 
    ON price_ticks (symbol, trade_id);

-- Table: trades (Paper & Live Executed Trades)
CREATE TABLE IF NOT EXISTS trades (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL CHECK (side IN ('BUY', 'SELL')),
    price NUMERIC(18, 8) NOT NULL,
    quantity NUMERIC(18, 8) NOT NULL,
    fee NUMERIC(18, 8) NOT NULL DEFAULT 0,
    slippage NUMERIC(18, 8) NOT NULL DEFAULT 0,
    reason TEXT,
    agent_confidence NUMERIC(5, 4) NOT NULL DEFAULT 0, -- 0.0000 to 1.0000
    pnl NUMERIC(18, 8) DEFAULT 0,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trades_symbol_executed_at 
    ON trades (symbol, executed_at DESC);

-- Table: agent_decisions (Audit trail for LLM & Rule-based decisions)
CREATE TABLE IF NOT EXISTS agent_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    action VARCHAR(10) NOT NULL CHECK (action IN ('BUY', 'SELL', 'HOLD')),
    confidence NUMERIC(5, 4) NOT NULL,
    reasoning TEXT NOT NULL,
    signals_used JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_decisions_symbol_created_at 
    ON agent_decisions (symbol, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_decisions_signals_used 
    ON agent_decisions USING GIN (signals_used);
