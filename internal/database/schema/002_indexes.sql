-- Indexes for improved query performance

-- Logs indexes for efficient filtering and sorting
CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_logs_category ON logs(category);
