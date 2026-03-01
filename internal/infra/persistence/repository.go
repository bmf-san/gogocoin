package persistence

import (
	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// Repository is a composite struct that aggregates all individual repositories
// and satisfies the adapter/http.DatabaseService interface.
// It is used in bootstrap to wire the HTTP server without importing old database package.
type Repository struct {
	trade       *TradeRepository
	position    *PositionRepository
	balance     *BalanceRepository
	marketData  *MarketDataRepository
	performance *PerformanceRepository
	log         *LogRepository
	appState    *AppStateRepository
	maintenance *MaintenanceRepository
}

// NewRepository creates a composite Repository backed by a single *DB.
func NewRepository(db *DB) *Repository {
	return &Repository{
		trade:       NewTradeRepository(db),
		position:    NewPositionRepository(db),
		balance:     NewBalanceRepository(db),
		marketData:  NewMarketDataRepository(db),
		performance: NewPerformanceRepository(db),
		log:         NewLogRepository(db),
		appState:    NewAppStateRepository(db),
		maintenance: NewMaintenanceRepository(db),
	}
}

// --- domain.TradeRepository ---

func (r *Repository) SaveTrade(trade *domain.Trade) error {
	return r.trade.SaveTrade(trade)
}
func (r *Repository) GetRecentTrades(limit int) ([]domain.Trade, error) {
	return r.trade.GetRecentTrades(limit)
}
func (r *Repository) GetTradesCount() (int, error) {
	return r.trade.GetTradesCount()
}

// --- domain.PositionRepository ---

func (r *Repository) SavePosition(position *domain.Position) error {
	return r.position.SavePosition(position)
}
func (r *Repository) GetOpenPositions(symbol string, side string) ([]domain.Position, error) {
	return r.position.GetOpenPositions(symbol, side)
}
func (r *Repository) UpdatePosition(position *domain.Position) error {
	return r.position.UpdatePosition(position)
}
func (r *Repository) GetActivePositions() ([]domain.Position, error) {
	return r.position.GetActivePositions()
}
func (r *Repository) GetActivePositionsCount() (int, error) {
	return r.position.GetActivePositionsCount()
}

// --- domain.BalanceRepository ---

func (r *Repository) SaveBalance(balance domain.Balance) error {
	return r.balance.SaveBalance(balance)
}
func (r *Repository) GetLatestBalances() ([]domain.Balance, error) {
	return r.balance.GetLatestBalances()
}

// --- domain.MarketDataRepository ---

func (r *Repository) SaveMarketData(data *domain.MarketData) error {
	return r.marketData.SaveMarketData(data)
}
func (r *Repository) SaveTicker(ticker *domain.MarketData) error {
	return r.marketData.SaveTicker(ticker)
}
func (r *Repository) GetLatestMarketData(symbol string, limit int) ([]domain.MarketData, error) {
	return r.marketData.GetLatestMarketData(symbol, limit)
}
func (r *Repository) GetLatestMarketDataForSymbols(symbols []string) (map[string]domain.MarketData, error) {
	return r.marketData.GetLatestMarketDataForSymbols(symbols)
}

// --- domain.PerformanceRepository + AnalyticsRepository ---

func (r *Repository) SavePerformanceMetric(metric *domain.PerformanceMetric) error {
	return r.performance.SavePerformanceMetric(metric)
}
func (r *Repository) GetLatestPerformanceMetric() (*domain.PerformanceMetric, error) {
	return r.performance.GetLatestPerformanceMetric()
}
func (r *Repository) GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error) {
	return r.performance.GetPerformanceMetrics(days)
}

// --- domain.LogRepository ---

func (r *Repository) SaveLog(entry *domain.LogEntry) error {
	return r.log.SaveLog(entry)
}
func (r *Repository) GetRecentLogsWithFilters(limit int, level, category string) ([]domain.LogEntry, error) {
	return r.log.GetRecentLogsWithFilters(limit, level, category)
}

// --- domain.AppStateRepository ---

func (r *Repository) SaveAppState(key, value string) error {
	return r.appState.SaveAppState(key, value)
}
func (r *Repository) GetAppState(key string) (string, error) {
	return r.appState.GetAppState(key)
}

// --- domain.MaintenanceRepository ---

func (r *Repository) GetDatabaseSize() (int64, error) {
	return r.maintenance.GetDatabaseSize()
}
func (r *Repository) CleanupOldData(retentionDays int) error {
	return r.maintenance.CleanupOldData(retentionDays)
}
func (r *Repository) GetTableStats() (map[string]int, error) {
	return r.maintenance.GetTableStats()
}

// --- domain.TransactionManager ---

func (r *Repository) BeginTx() (domain.Transaction, error) {
	return r.trade.db.BeginTx()
}
