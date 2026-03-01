-- Initial schema for gogocoin trading bot
-- Creates core tables for balances, trades, positions, performance metrics, market data, and logs

-- Balances table: stores account balance snapshots
CREATE TABLE IF NOT EXISTS balances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    currency TEXT NOT NULL,
    available REAL NOT NULL,
    amount REAL NOT NULL,
    timestamp DATETIME NOT NULL
);

-- Trades table: stores executed trade records
CREATE TABLE IF NOT EXISTS trades (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    type TEXT NOT NULL,
    size REAL NOT NULL,
    price REAL NOT NULL,
    fee REAL NOT NULL,
    status TEXT NOT NULL,
    order_id TEXT UNIQUE,
    executed_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    strategy_name TEXT,
    pnl REAL DEFAULT 0
);

-- Positions table: tracks open and closed positions
CREATE TABLE IF NOT EXISTS positions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    size REAL NOT NULL,
    used_size REAL DEFAULT 0,
    remaining_size REAL NOT NULL,
    entry_price REAL NOT NULL,
    current_price REAL NOT NULL,
    unrealized_pl REAL DEFAULT 0,
    pnl REAL DEFAULT 0,
    status TEXT NOT NULL,
    order_id TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Performance metrics table: stores daily performance statistics
CREATE TABLE IF NOT EXISTS performance_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATETIME NOT NULL,
    total_return REAL DEFAULT 0,
    daily_return REAL DEFAULT 0,
    win_rate REAL DEFAULT 0,
    max_drawdown REAL DEFAULT 0,
    sharpe_ratio REAL DEFAULT 0,
    profit_factor REAL DEFAULT 0,
    total_trades INTEGER DEFAULT 0,
    winning_trades INTEGER DEFAULT 0,
    losing_trades INTEGER DEFAULT 0,
    average_win REAL DEFAULT 0,
    average_loss REAL DEFAULT 0,
    largest_win REAL DEFAULT 0,
    largest_loss REAL DEFAULT 0,
    consecutive_wins INTEGER DEFAULT 0,
    consecutive_loss INTEGER DEFAULT 0,
    total_pnl REAL DEFAULT 0
);

-- Market data table: stores OHLCV market data
CREATE TABLE IF NOT EXISTS market_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    open REAL NOT NULL,
    high REAL NOT NULL,
    low REAL NOT NULL,
    close REAL NOT NULL,
    volume REAL NOT NULL,
    created_at DATETIME NOT NULL,
    UNIQUE(symbol, timestamp)
);

-- Logs table: stores application logs
CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    level TEXT NOT NULL,
    category TEXT NOT NULL,
    message TEXT NOT NULL,
    fields TEXT,
    timestamp DATETIME NOT NULL
);
