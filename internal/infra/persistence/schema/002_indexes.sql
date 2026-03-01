-- Indexes for improved query performance

-- Logs indexes for efficient filtering and sorting
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_logs_category ON logs(category);

-- Trades: executed_at is used heavily for recent-trade queries and cleanup
CREATE INDEX IF NOT EXISTS idx_trades_executed_at ON trades(executed_at DESC);

-- Market data: symbol + timestamp narrowly scopes symbol-specific lookups
CREATE INDEX IF NOT EXISTS idx_market_data_symbol_timestamp ON market_data(symbol, timestamp DESC);

-- Positions: status + created_at supports active-position listing and cleanup
CREATE INDEX IF NOT EXISTS idx_positions_status_created_at ON positions(status, created_at DESC);
