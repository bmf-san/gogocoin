package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
)

// Balance represents balance for API
type Balance struct {
	Currency  string  `json:"currency"`
	Amount    float64 `json:"amount"`
	Available float64 `json:"available"`
}

// ApplicationService is the application service interface
type ApplicationService interface {
	GetBalances(ctx context.Context) ([]Balance, error)
	GetCurrentStrategy() strategy.Strategy
	GetTradingService() interface{} // Method to return TradingService
	IsTradingEnabled() bool
	SetTradingEnabled(enabled bool) error
}

// Server is the API server for Web UI
type Server struct {
	config    *config.Config
	db        *database.DB
	logger    *logger.Logger
	host      string
	port      int
	webRoot   string
	startTime time.Time
	app       ApplicationService
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, db *database.DB, logger *logger.Logger) *Server {
	// UIget port from configuration, default to8080 as default
	port := cfg.UI.Port
	if port == 0 {
		port = 8080
	}

	// get host from configuration, default tolocalhost as default
	host := cfg.UI.Host
	if host == "" {
		host = "localhost"
	}

	return &Server{
		config:    cfg,
		db:        db,
		logger:    logger,
		host:      host,
		port:      port,
		webRoot:   "web", // fixed value is fine
		startTime: time.Now(),
		app:       nil, // withSetApplication()withconfiguration
	}
}

// SetApplication isアプリケーションservice" "configuration
func (s *Server) SetApplication(app ApplicationService) {
	s.app = app
}

// Start isAPIserver" "開始
func (s *Server) Start() error {
	// 新しいServeMux" "作成
	mux := http.NewServeMux()

	// API エンドポイント（先に登録）
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/balance", s.handleBalance)
	// Positions endpoint removed - not used in spot trading simulation
	mux.HandleFunc("/api/trades", s.handleTrades)
	mux.HandleFunc("/api/orders", s.handleOrders)
	mux.HandleFunc("/api/performance", s.handlePerformance)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/strategy/reset", s.handleStrategyReset)
	mux.HandleFunc("/api/trading/start", s.handleTradingStart)
	mux.HandleFunc("/api/trading/stop", s.handleTradingStop)
	mux.HandleFunc("/api/trading/status", s.handleTradingStatus)

	// 静的file配信（APIパス以外）
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// APIパスof場合is404returns
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// 静的file" "配信
		http.FileServer(http.Dir(s.webRoot)).ServeHTTP(w, r)
	})

	s.logger.Info(fmt.Sprintf("Starting web server on %s:%d", s.host, s.port))

	// セキュリティofためタイムアウト" "configuration
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.host, s.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}

// StatusResponse isステータス情報ofレスポンス
type StatusResponse struct {
	Status         string    `json:"status"`
	Mode           string    `json:"mode"`
	Strategy       string    `json:"strategy"`
	LastUpdate     time.Time `json:"last_update"`
	Uptime         string    `json:"uptime"`
	TotalTrades    int       `json:"total_trades"`
	ActiveOrders   int       `json:"active_orders"`
	TradingEnabled bool      `json:"trading_enabled"`
}

// TradingStatusResponse istrading状態ofレスポンス
type TradingStatusResponse struct {
	Enabled   bool      `json:"enabled"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// handleStatus isシステムステータスreturns
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// mode表示名" "調整
	s.logger.Debug("Current trading mode: " + s.config.Mode)
	var modeDisplay string
	switch s.config.Mode {
	case "paper":
		modeDisplay = "papertrade"
	case "live":
		// livemodewithもAPIcredentialsが不足している場合is警告" "表示
		hasAPIKey := s.config.API.Credentials.APIKey != "" && s.config.API.Credentials.APIKey != "${BITFLYER_API_KEY}"
		hasAPISecret := s.config.API.Credentials.APISecret != "" && s.config.API.Credentials.APISecret != "${BITFLYER_API_SECRET}"

		if hasAPIKey && hasAPISecret {
			modeDisplay = "livetrade"
		} else {
			modeDisplay = "livetrade (⚠️credentials不足 - papermodewith動作中)"
		}
	case "dev":
		modeDisplay = "開発mode"
	default:
		modeDisplay = s.config.Mode
	}

	// 稼働時間" "動的に計算
	uptime := s.calculateUptime()

	// 実際oftrading数とアクティブオーダー数get
	totalTrades, err := s.getTotalTradesCount()
	if err != nil {
		s.logger.Error("Failed to get total trades count: " + err.Error())
		totalTrades = 0
	}

	activeOrders, err := s.getActiveOrdersCount()
	if err != nil {
		s.logger.Error("Failed to get active orders count: " + err.Error())
		activeOrders = 0
	}

	status := StatusResponse{
		Status:         "running",
		Mode:           modeDisplay,
		Strategy:       s.config.Trading.Strategy.Name,
		LastUpdate:     time.Now(),
		Uptime:         uptime,
		TotalTrades:    totalTrades,
		ActiveOrders:   activeOrders,
		TradingEnabled: s.app.IsTradingEnabled(),
	}

	s.writeJSON(w, status)
}

// BalanceResponse represents balance informationofレスポンス
type BalanceResponse struct {
	Currency   string    `json:"currency"`
	Available  float64   `json:"available"`
	Amount     float64   `json:"amount"`
	LastUpdate time.Time `json:"last_update"`
}

// handleBalance represents balance informationreturns
func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// アプリケーションfrom実際ofbalanceget（papertrade対応）
	if s.app != nil {
		balances, err := s.app.GetBalances(r.Context())
		if err != nil {
			s.logger.Error("Failed to get balances from application: " + err.Error())
		} else if len(balances) > 0 {
			// Balance構造体" "BalanceResponseに変換
			var responses []database.Balance
			for _, balance := range balances {
				responses = append(responses, database.Balance{
					Currency:  balance.Currency,
					Amount:    balance.Amount,
					Available: balance.Available,
					Timestamp: time.Now(),
				})
			}
			// balance" "ソート
			s.sortBalances(responses)
			s.writeJSON(w, responses)
			return
		}
	}

	// フォールバック：databasefrom取得
	balances, err := s.db.GetLatestBalances()
	if err != nil {
		s.logger.Error("Failed to get balances from database: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// dataが空of場合isサンプルdatareturns
	if len(balances) == 0 {
		balances = s.generateSampleBalances()
	}

	// balance" "ソート
	s.sortBalances(balances)

	s.writeJSON(w, balances)
}

// Positions functionality removed - not applicable to spot trading simulation

// TradeResponse istradinghistoryofレスポンス
type TradeResponse struct {
	ID          int       `json:"id"`
	ProductCode string    `json:"product_code"`
	Side        string    `json:"side"`
	Size        float64   `json:"size"`
	Price       float64   `json:"price"`
	Fee         float64   `json:"fee"`
	Timestamp   time.Time `json:"timestamp"`
	Strategy    string    `json:"strategy"`
	PnL         float64   `json:"pnl"`
}

// handleTrades istradinghistoryreturns
func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// クエリparametersfrom制限数get
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	trades, err := s.db.GetRecentTrades(limit)
	if err != nil {
		s.logger.Error("Failed to get trades: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 実際oftradingdatareturns（空of場合isempty array）
	s.writeJSON(w, trades)
}

// PerformanceResponse isperformance情報ofレスポンス
type PerformanceResponse struct {
	TotalPnL    float64   `json:"total_pnl"`
	DailyPnL    float64   `json:"daily_pnl"`
	WeeklyPnL   float64   `json:"weekly_pnl"`
	MonthlyPnL  float64   `json:"monthly_pnl"`
	WinRate     float64   `json:"win_rate"`
	MaxDrawdown float64   `json:"max_drawdown"`
	SharpeRatio float64   `json:"sharpe_ratio"`
	LastUpdate  time.Time `json:"last_update"`
}

// handlePerformance isperformance情報returns
func (s *Server) handlePerformance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	performance, err := s.db.GetPerformanceMetrics(30) // 過去30日間
	if err != nil {
		s.logger.Error("Failed to get performance: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 実際ofperformancedatareturns（空of場合isempty array）
	s.writeJSON(w, performance)
}

// ConfigUpdateRequest isconfiguration更新リクエスト
type ConfigUpdateRequest struct {
	Strategy struct {
		Name string `json:"name"`
	} `json:"strategy"`
	Risk struct {
		StopLoss   float64 `json:"stop_loss"`
		TakeProfit float64 `json:"take_profit"`
	} `json:"risk"`
}

// handleConfig isconfiguration情報returns・更新する
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.logger.Info("Returning config",
			"trading_mode", s.config.Mode,
			"strategy", s.config.Trading.Strategy.Name,
			"stop_loss", s.config.Trading.RiskManagement.StopLossPercent,
			"take_profit", s.config.Trading.RiskManagement.TakeProfitPercent)
		s.writeJSON(w, s.config)
	case http.MethodPost:
		s.handleConfigUpdate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConfigUpdate isconfigurationupdates
func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.logger.Info("Updating configuration",
		"strategy", req.Strategy.Name)

	// configuration" "更新（modeis実行時固定ofため更新不可）
	if req.Strategy.Name != "" {
		s.config.Trading.Strategy.Name = req.Strategy.Name
	}
	if req.Risk.StopLoss > 0 {
		s.config.Trading.RiskManagement.StopLossPercent = req.Risk.StopLoss // 既にパーセント値
	}
	if req.Risk.TakeProfit > 0 {
		s.config.Trading.RiskManagement.TakeProfitPercent = req.Risk.TakeProfit // 既にパーセント値
	}

	// configurationfileに保存
	if err := s.saveConfigToFile(); err != nil {
		s.logger.Error("Failed to save config to file: " + err.Error())
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Configuration updated successfully")

	// configurationfilefromreloadして一貫性" "確保
	configPath := "configs/config.yaml"
	reloadedConfig, err := config.Load(configPath)
	if err != nil {
		s.logger.Error("Failed to reload config from file: " + err.Error())
		// errorwithもメモリ上ofconfigurationreturns
	} else {
		s.config = reloadedConfig // reloadしたconfigurationwith更新
		s.logger.Info("Configuration reloaded from file successfully")
	}

	// Return updated configuration
	response := map[string]interface{}{
		"status":  "success",
		"message": "Configuration saved",
		"config":  s.config,
	}

	s.writeJSON(w, response)
}

// saveConfigToFile isconfiguration" "fileに保存する
func (s *Server) saveConfigToFile() error {
	// configurationfileofパスget（通常is configs/config.yaml）
	configPath := "configs/config.yaml"

	// configuration" "YAMLに変換
	data, err := yaml.Marshal(s.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// ディレクトリが存在しない場合is作成
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// fileに書き込み
	// #nosec G306 - configurationfileis読み取り可能withある必要がある
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LogEntry islogentry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Category  string    `json:"category"`
	Message   string    `json:"message"`
}

// handleLogs islog情報returns
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// クエリparametersfrom制限数get
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// クエリparametersfromフィルタget
	levelFilter := r.URL.Query().Get("level")
	categoryFilter := r.URL.Query().Get("category")

	logs, err := s.db.GetRecentLogsWithFilters(limit, levelFilter, categoryFilter)
	if err != nil {
		s.logger.Error("Failed to get logs: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, logs)
}

// calculateUptime is稼働時間" "計算して文字列with返す
func (s *Server) calculateUptime() string {
	duration := time.Since(s.startTime)

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

// OrderResponse isorder情報ofレスポンス
type OrderResponse struct {
	OrderID    string    `json:"order_id"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Type       string    `json:"type"`
	Size       float64   `json:"size"`
	Price      float64   `json:"price"`
	Status     string    `json:"status"`
	ExecutedAt time.Time `json:"executed_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleOrders isorder情報returns
func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// クエリparametersfrom制限数get
	limitStr := r.URL.Query().Get("limit")
	limit := 20 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// 最近oftradingfromorder情報get
	trades, err := s.db.GetRecentTrades(limit)
	if err != nil {
		s.logger.Error("Failed to get recent trades for orders: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Trade" "OrderResponseに変換
	var orders []OrderResponse
	for i := range trades {
		trade := &trades[i]
		if trade.OrderID != "" { // order_idが存在するtradingofみ
			orders = append(orders, OrderResponse{
				OrderID:    trade.OrderID,
				Symbol:     trade.Symbol,
				Side:       trade.Side,
				Type:       trade.Type,
				Size:       trade.Size,
				Price:      trade.Price,
				Status:     trade.Status,
				ExecutedAt: trade.ExecutedAt,
				CreatedAt:  trade.CreatedAt,
			})
		}
	}

	s.writeJSON(w, orders)
}

// sortBalances isbalance" "通貨順にソートする（JPY" "最初、そof後アルファベット順）
func (s *Server) sortBalances(balances []database.Balance) {
	// 通貨of表示順序" "定義
	currencyOrder := map[string]int{
		"JPY":  1,
		"BTC":  2,
		"ETH":  3,
		"XRP":  4,
		"XLM":  5,
		"MONA": 6,
	}

	sort.Slice(balances, func(i, j int) bool {
		orderI, existsI := currencyOrder[balances[i].Currency]
		orderJ, existsJ := currencyOrder[balances[j].Currency]

		// 両方が定義済み通貨of場合is定義順
		if existsI && existsJ {
			return orderI < orderJ
		}
		// iofみ定義済みof場合isi" "先に
		if existsI {
			return true
		}
		// jofみ定義済みof場合isj" "先に
		if existsJ {
			return false
		}
		// どちらも未定義of場合isアルファベット順
		return balances[i].Currency < balances[j].Currency
	})
}

// generateSampleBalances isサンプルbalancedata" "生成
func (s *Server) generateSampleBalances() []database.Balance {
	now := time.Now()
	return []database.Balance{
		{
			ID:        1,
			Currency:  "JPY",
			Available: 500000.0,
			Amount:    1000000.0,
			Timestamp: now,
		},
		{
			ID:        2,
			Currency:  "BTC",
			Available: 0.025,
			Amount:    0.05,
			Timestamp: now,
		},
		{
			ID:        3,
			Currency:  "ETH",
			Available: 0.5,
			Amount:    1.0,
			Timestamp: now,
		},
	}
}

// getTotalTradesCount is総trading数gets
func (s *Server) getTotalTradesCount() (int, error) {
	trades, err := s.db.GetRecentTrades(1000) // 大きな数値with全件取得" "試行
	if err != nil {
		return 0, err
	}
	return len(trades), nil
}

// getActiveOrdersCount isアクティブオーダー数gets
func (s *Server) getActiveOrdersCount() (int, error) {
	// papermodeof場合isTradingServicefromアクティブオーダー数get
	if s.config.Mode == "paper" {
		if s.app != nil && s.app.GetTradingService() != nil {
			// 型アサーションwithTradingServiceに変換
			if tradingService, ok := s.app.GetTradingService().(*bitflyer.TradingService); ok {
				orders, err := tradingService.GetOrders(context.Background())
				if err != nil {
					return 0, err
				}

				// アクティブなorderofみカウント
				activeCount := 0
				for _, order := range orders {
					if order.Status == "ACTIVE" {
						activeCount++
					}
				}
				return activeCount, nil
			}
		}
		return 0, nil
	}

	positions, err := s.db.GetActivePositions()
	if err != nil {
		return 0, err
	}

	// アクティブなposition数" "アクティブオーダー数として返す
	activeCount := 0
	for i := range positions {
		if positions[i].Status == "OPEN" {
			activeCount++
		}
	}

	return activeCount, nil
}

// handleStrategyReset isstrategy" "リセットする
func (s *Server) handleStrategyReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("Strategy reset requested via API")

	// アプリケーションfrom現在ofstrategygetしてリセット
	if s.app != nil && s.app.GetCurrentStrategy() != nil {
		strategy := s.app.GetCurrentStrategy()

		if err := strategy.Reset(); err != nil {
			s.logger.Error("Failed to reset strategy: " + err.Error())
			s.writeJSON(w, map[string]interface{}{
				"status":  "error",
				"message": "Failed to reset strategy: " + err.Error(),
			})
			return
		}

		s.logger.Info("Strategy reset successfully via API")
		s.writeJSON(w, map[string]interface{}{
			"status":  "success",
			"message": "Strategy reset successfully",
		})
	} else {
		s.writeJSON(w, map[string]interface{}{
			"status":  "error",
			"message": "No strategy available to reset",
		})
	}
}

// handleTradingStart istradingstarts
func (s *Server) handleTradingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("Trading start requested via API")

	if err := s.app.SetTradingEnabled(true); err != nil {
		s.logger.Error("Failed to start trading: " + err.Error())
		s.writeJSON(w, TradingStatusResponse{
			Enabled:   false,
			Status:    "error",
			Message:   "Failed to start trading: " + err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	s.logger.Info("Trading started successfully via API")
	s.writeJSON(w, TradingStatusResponse{
		Enabled:   true,
		Status:    "success",
		Message:   "Trading started successfully",
		Timestamp: time.Now(),
	})
}

// handleTradingStop istradingstops
func (s *Server) handleTradingStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("Trading stop requested via API")

	if err := s.app.SetTradingEnabled(false); err != nil {
		s.logger.Error("Failed to stop trading: " + err.Error())
		s.writeJSON(w, TradingStatusResponse{
			Enabled:   true,
			Status:    "error",
			Message:   "Failed to stop trading: " + err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	s.logger.Info("Trading stopped successfully via API")
	s.writeJSON(w, TradingStatusResponse{
		Enabled:   false,
		Status:    "success",
		Message:   "Trading stopped successfully",
		Timestamp: time.Now(),
	})
}

// handleTradingStatus istrading状態returns
func (s *Server) handleTradingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enabled := s.app.IsTradingEnabled()
	status := "stopped"
	message := "Trading is currently stopped"

	if enabled {
		status = "running"
		message = "Trading is currently active"
	}

	s.writeJSON(w, TradingStatusResponse{
		Enabled:   enabled,
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// writeJSON isJSONレスポンス" "書き込む
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON: " + err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
