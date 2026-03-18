-- Deduplicate performance_metrics: keep only the most recent row per calendar day.
-- This fixes the bug where UpdateMetrics (called after every trade) inserted a new row
-- each time, resulting in hundreds of rows per day instead of one daily summary.
DELETE FROM performance_metrics
WHERE id NOT IN (
    SELECT max(id) FROM performance_metrics GROUP BY date(date)
);

-- Add unique index on the date portion so future INSERTs are upserts (one row per day).
CREATE UNIQUE INDEX IF NOT EXISTS idx_perf_date_day
    ON performance_metrics (date(date));
