package bitflyer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/go-bitflyer-api-client/client/websocket"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// MarketData represents market data
type MarketData struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
	BestBid   float64   `json:"best_bid"`
	BestAsk   float64   `json:"best_ask"`
	Spread    float64   `json:"spread"`
	Timestamp time.Time `json:"timestamp"`

	// OHLCV data
	Open  float64 `json:"open,omitempty"`
	High  float64 `json:"high,omitempty"`
	Low   float64 `json:"low,omitempty"`
	Close float64 `json:"close,omitempty"`

	// Additional information
	VolumeByProduct float64 `json:"volume_by_product,omitempty"`
	TradeCount      int     `json:"trade_count,omitempty"`
}

// Execution represents execution data
type Execution struct {
	ID                         string    `json:"id"`
	Symbol                     string    `json:"symbol"`
	Side                       string    `json:"side"`
	Price                      float64   `json:"price"`
	Size                       float64   `json:"size"`
	ExecDate                   time.Time `json:"exec_date"`
	BuyChildOrderAcceptanceId  string    `json:"buy_child_order_acceptance_id,omitempty"`
	SellChildOrderAcceptanceId string    `json:"sell_child_order_acceptance_id,omitempty"`
}

// OrderBook represents order book data
type OrderBook struct {
	Symbol    string      `json:"symbol"`
	Bids      []OrderItem `json:"bids"`
	Asks      []OrderItem `json:"asks"`
	Timestamp time.Time   `json:"timestamp"`
}

// OrderItem represents an individual order book item
type OrderItem struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

// MarketDataService ismarket data retrieval service
type MarketDataService struct {
	client             *Client
	logger             *logger.Logger
	tickerCallbacks    map[string]func(MarketData)
	executionCallbacks map[string]func([]Execution)
	boardCallbacks     map[string]func(OrderBook)
	db                 MarketDataSaver   // database saving interface
	config             *MarketDataConfig // configuration
	lastSaveTime       map[string]time.Time // 最後にデータを保存した時刻（シンボルごと）
	mu                 sync.RWMutex         // lastSaveTime保護用
}

// MarketDataConfig ismarket dataconfiguration
type MarketDataConfig struct {
	HistoryDays int `yaml:"history_days"`
}

// MarketDataSaver ismarket data保存ofインターフェース
type MarketDataSaver interface {
	SaveMarketData(data *database.MarketData) error
}

// NewMarketDataService is新しいmarket dataservicecreates
func NewMarketDataService(client *Client, log *logger.Logger) *MarketDataService {
	return &MarketDataService{
		client:       client,
		logger:       log,
		lastSaveTime: make(map[string]time.Time),
		config: &MarketDataConfig{
			HistoryDays: 365, // default1年
		},
	}
}

// SetDatabase isdatabase saving interfacesets
func (mds *MarketDataService) SetDatabase(db MarketDataSaver) {
	mds.db = db
}

// SetConfig ismarket dataconfigurationsets
func (mds *MarketDataService) SetConfig(historyDays int) {
	mds.config = &MarketDataConfig{
		HistoryDays: historyDays,
	}
}

// GetTicker isティッカー情報gets
func (mds *MarketDataService) GetTicker(ctx context.Context, symbol string) (*MarketData, error) {
	// rate limitingチェック
	if err := mds.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// API呼び出し
	resp, err := mds.client.httpClient.GetV1GettickerWithResponse(ctx, &http.GetV1GettickerParams{
		ProductCode: symbol,
	})

	duration := time.Since(start).Milliseconds()

	if err != nil {
		mds.logger.LogAPICall("GET", "/v1/getticker", duration, 0, err)
		return nil, fmt.Errorf("failed to get ticker: %w", err)
	}

	mds.logger.LogAPICall("GET", "/v1/getticker", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	// data変換
	ticker := resp.JSON200

	// ポインタfrom値" "安全に取得するヘルパー関数
	safeFloat64 := func(ptr *float32) float64 {
		if ptr == nil {
			return 0
		}
		return float64(*ptr)
	}

	bestBid := safeFloat64(ticker.BestBid)
	bestAsk := safeFloat64(ticker.BestAsk)

	marketData := &MarketData{
		Symbol:          symbol,
		Price:           safeFloat64(ticker.Ltp),
		Volume:          safeFloat64(ticker.Volume),
		BestBid:         bestBid,
		BestAsk:         bestAsk,
		Spread:          bestAsk - bestBid,
		VolumeByProduct: safeFloat64(ticker.VolumeByProduct),
		Timestamp:       time.Now(),
	}

	// data検証
	if err := mds.validateMarketData(marketData); err != nil {
		return nil, fmt.Errorf("invalid market data: %w", err)
	}

	return marketData, nil
}

// GetOrderBook is板情報gets
func (mds *MarketDataService) GetOrderBook(ctx context.Context, symbol string) (*OrderBook, error) {
	// rate limitingチェック
	if err := mds.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// API呼び出し
	resp, err := mds.client.httpClient.GetV1GetboardWithResponse(ctx, &http.GetV1GetboardParams{
		ProductCode: symbol,
	})

	duration := time.Since(start).Milliseconds()

	if err != nil {
		mds.logger.LogAPICall("GET", "/v1/getboard", duration, 0, err)
		return nil, fmt.Errorf("failed to get order book: %w", err)
	}

	mds.logger.LogAPICall("GET", "/v1/getboard", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	// data変換
	board := resp.JSON200
	orderBook := &OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	// ポインタfrom値" "安全に取得するヘルパー関数
	safeFloat64 := func(ptr *float32) float64 {
		if ptr == nil {
			return 0
		}
		return float64(*ptr)
	}

	// Bids変換
	if board.Bids != nil {
		for _, bid := range *board.Bids {
			orderBook.Bids = append(orderBook.Bids, OrderItem{
				Price: safeFloat64(bid.Price),
				Size:  safeFloat64(bid.Size),
			})
		}
	}

	// Asks変換
	if board.Asks != nil {
		for _, ask := range *board.Asks {
			orderBook.Asks = append(orderBook.Asks, OrderItem{
				Price: safeFloat64(ask.Price),
				Size:  safeFloat64(ask.Size),
			})
		}
	}

	return orderBook, nil
}

// GetExecutions is約定historygets
func (mds *MarketDataService) GetExecutions(ctx context.Context, symbol string, count int) ([]Execution, error) {
	// rate limitingチェック
	if err := mds.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// parametersconfiguration
	params := &http.GetV1GetexecutionsParams{
		ProductCode: symbol,
	}
	if count > 0 {
		params.Count = &count
	}

	// API呼び出し
	resp, err := mds.client.httpClient.GetV1GetexecutionsWithResponse(ctx, params)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		mds.logger.LogAPICall("GET", "/v1/getexecutions", duration, 0, err)
		return nil, fmt.Errorf("failed to get executions: %w", err)
	}

	mds.logger.LogAPICall("GET", "/v1/getexecutions", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	// data変換
	var executions []Execution
	for _, exec := range *resp.JSON200 {
		// ポインタfrom値" "安全に取得するヘルパー関数
		safeString := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		safeFloat64 := func(ptr *float32) float64 {
			if ptr == nil {
				return 0
			}
			return float64(*ptr)
		}

		// ExecDate" "時刻に変換
		var execTime time.Time
		if exec.ExecDate != nil {
			execTime = *exec.ExecDate
		} else {
			execTime = time.Now()
		}

		execution := Execution{
			ID:       fmt.Sprintf("%d", exec.Id),
			Symbol:   symbol,
			Side:     safeString(exec.Side),
			Price:    safeFloat64(exec.Price),
			Size:     safeFloat64(exec.Size),
			ExecDate: execTime,
		}

		if exec.BuyChildOrderAcceptanceId != nil && *exec.BuyChildOrderAcceptanceId != "" {
			execution.BuyChildOrderAcceptanceId = *exec.BuyChildOrderAcceptanceId
		}
		if exec.SellChildOrderAcceptanceId != nil && *exec.SellChildOrderAcceptanceId != "" {
			execution.SellChildOrderAcceptanceId = *exec.SellChildOrderAcceptanceId
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

// SubscribeToTicker isティッカーdata" "リアルタイム購読する
func (mds *MarketDataService) SubscribeToTicker(ctx context.Context, symbol string, callback func(MarketData)) error {
	if !mds.client.IsConnected() {
		return fmt.Errorf("websocket client is not connected")
	}

	// 初回ofみグローバルハンドラー" "configuration
	if mds.tickerCallbacks == nil {
		mds.tickerCallbacks = make(map[string]func(MarketData))

		// グローバルティッカーハンドラー" "configuration（全シンボル対応）
		mds.client.wsClient.OnTicker(func(ticker websocket.TickerMessage) {
			mds.logger.Data().WithField("symbol", ticker.ProductCode).WithField("price", ticker.Ltp).Info("Ticker data received from WebSocket")

			// 該当するシンボルofコールバック" "実行
			if cb, exists := mds.tickerCallbacks[ticker.ProductCode]; exists {
				marketData := MarketData{
					Symbol:          ticker.ProductCode,
					Price:           ticker.Ltp,
					Volume:          ticker.Volume,
					BestBid:         ticker.BestBid,
					BestAsk:         ticker.BestAsk,
					Spread:          ticker.BestAsk - ticker.BestBid,
					VolumeByProduct: ticker.VolumeByProduct,
					Timestamp:       time.Now(),
				}

				// data検証
				if err := mds.validateMarketData(&marketData); err != nil {
					mds.logger.Data().WithError(err).Error("Invalid ticker data received")
					return
				}

				mds.logger.Data().WithField("symbol", marketData.Symbol).WithField("price", marketData.Price).Info("Market data processed and calling callback")

				// databaseにmarket data" "保存（非同期）
				mds.saveMarketDataToDB(&marketData)

				cb(marketData)
			} else {
				mds.logger.Data().WithField("symbol", ticker.ProductCode).Info("No callback registered for this symbol")
			}
		})

		mds.logger.Data().Info("Global ticker handler initialized")
	}

	// シンボル別ofコールバック" "登録
	mds.tickerCallbacks[symbol] = callback
	mds.logger.Data().WithField("symbol", symbol).Info("Ticker callback registered")

	// channel購読
	channel := fmt.Sprintf("lightning_ticker_%s", symbol)
	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Attempting to subscribe to ticker channel")

	if err := mds.client.wsClient.Subscribe(ctx, channel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", channel).WithField("symbol", symbol).Error("Failed to subscribe to ticker channel")
		return fmt.Errorf("failed to subscribe to ticker channel %s: %w", channel, err)
	}

	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Successfully subscribed to ticker channel")
	return nil
}

// SubscribeToExecutions is約定data" "リアルタイム購読する
func (mds *MarketDataService) SubscribeToExecutions(ctx context.Context, symbol string, callback func([]Execution)) error {
	if !mds.client.IsConnected() {
		return fmt.Errorf("websocket client is not connected")
	}

	// 初回ofみグローバルハンドラー" "configuration
	if mds.executionCallbacks == nil {
		mds.executionCallbacks = make(map[string]func([]Execution))

		// グローバル約定ハンドラー" "configuration（全シンボル対応）
		mds.client.wsClient.OnExecutions(func(execMsg websocket.ExecutionsMessage) {
			mds.logger.Data().WithField("symbol", execMsg.ProductCode).Info("Execution data received from WebSocket")

			// 該当するシンボルofコールバック" "実行
			if cb, exists := mds.executionCallbacks[execMsg.ProductCode]; exists {
				var executions []Execution
				for _, exec := range execMsg.Executions {
					// ExecDate" "時刻に変換
					execTime, err := time.Parse(time.RFC3339, exec.ExecDate)
					if err != nil {
						execTime = time.Now() // パースに失敗した場合is現在時刻
					}

					execution := Execution{
						ID:       fmt.Sprintf("%d", exec.ID),
						Symbol:   execMsg.ProductCode,
						Side:     exec.Side,
						Price:    exec.Price,
						Size:     exec.Size,
						ExecDate: execTime,
					}

					if exec.BuyChildOrderAcceptanceID != "" {
						execution.BuyChildOrderAcceptanceId = exec.BuyChildOrderAcceptanceID
					}
					if exec.SellChildOrderAcceptanceID != "" {
						execution.SellChildOrderAcceptanceId = exec.SellChildOrderAcceptanceID
					}

					executions = append(executions, execution)
				}

				cb(executions)
			} else {
				mds.logger.Data().WithField("symbol", execMsg.ProductCode).Info("No callback registered for this symbol")
			}
		})

		mds.logger.Data().Info("Global execution handler initialized")
	}

	// シンボル別ofコールバック" "登録
	mds.executionCallbacks[symbol] = callback
	mds.logger.Data().WithField("symbol", symbol).Info("Execution callback registered")

	// channel購読
	channel := fmt.Sprintf("lightning_executions_%s", symbol)
	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Attempting to subscribe to executions channel")

	if err := mds.client.wsClient.Subscribe(ctx, channel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", channel).WithField("symbol", symbol).Error("Failed to subscribe to executions channel")
		return fmt.Errorf("failed to subscribe to executions channel %s: %w", channel, err)
	}

	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Successfully subscribed to executions channel")
	return nil
}

// SubscribeToOrderBook is板情報" "リアルタイム購読する
func (mds *MarketDataService) SubscribeToOrderBook(ctx context.Context, symbol string, callback func(OrderBook)) error {
	if !mds.client.IsConnected() {
		return fmt.Errorf("websocket client is not connected")
	}

	// 初回ofみグローバルハンドラー" "configuration
	if mds.boardCallbacks == nil {
		mds.boardCallbacks = make(map[string]func(OrderBook))

		// グローバル板情報ハンドラー" "configuration（全シンボル対応）
		// スナップショットハンドラー
		mds.client.wsClient.OnBoardSnapshot(func(snapshot websocket.BoardSnapshotMessage) {
			mds.logger.Data().WithField("symbol", snapshot.ProductCode).Info("Board snapshot data received from WebSocket")

			// 該当するシンボルofコールバック" "実行
			if cb, exists := mds.boardCallbacks[snapshot.ProductCode]; exists {
				orderBook := mds.convertBoardData(snapshot.ProductCode, snapshot.Data)
				cb(orderBook)
			} else {
				mds.logger.Data().WithField("symbol", snapshot.ProductCode).Info("No callback registered for this symbol")
			}
		})

		// 差分ハンドラー
		mds.client.wsClient.OnBoard(func(board websocket.BoardMessage) {
			mds.logger.Data().WithField("symbol", board.ProductCode).Debug("Board diff data received from WebSocket")

			// 該当するシンボルofコールバック" "実行
			if cb, exists := mds.boardCallbacks[board.ProductCode]; exists {
				orderBook := mds.convertBoardData(board.ProductCode, board.Data)
				cb(orderBook)
			}
		})

		mds.logger.Data().Info("Global board handler initialized")
	}

	// シンボル別ofコールバック" "登録
	mds.boardCallbacks[symbol] = callback
	mds.logger.Data().WithField("symbol", symbol).Info("Board callback registered")

	// スナップショットchannel
	snapshotChannel := fmt.Sprintf("lightning_board_snapshot_%s", symbol)
	mds.logger.Data().WithField("channel", snapshotChannel).WithField("symbol", symbol).Info("Attempting to subscribe to board snapshot channel")

	if err := mds.client.wsClient.Subscribe(ctx, snapshotChannel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", snapshotChannel).WithField("symbol", symbol).Error("Failed to subscribe to board snapshot channel")
		return fmt.Errorf("failed to subscribe to board snapshot channel: %w", err)
	}

	// 差分channel
	diffChannel := fmt.Sprintf("lightning_board_%s", symbol)
	mds.logger.Data().WithField("channel", diffChannel).WithField("symbol", symbol).Info("Attempting to subscribe to board diff channel")

	if err := mds.client.wsClient.Subscribe(ctx, diffChannel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", diffChannel).WithField("symbol", symbol).Error("Failed to subscribe to board diff channel")
		return fmt.Errorf("failed to subscribe to board diff channel: %w", err)
	}

	mds.logger.Data().WithField("symbol", symbol).Info("Successfully subscribed to order book channels")
	return nil
}

// convertBoardData is板data" "変換する
func (mds *MarketDataService) convertBoardData(symbol string, data websocket.BoardData) OrderBook {
	orderBook := OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	// Bids変換
	for _, bid := range data.Bids {
		orderBook.Bids = append(orderBook.Bids, OrderItem{
			Price: bid.Price,
			Size:  bid.Size,
		})
	}

	// Asks変換
	for _, ask := range data.Asks {
		orderBook.Asks = append(orderBook.Asks, OrderItem{
			Price: ask.Price,
			Size:  ask.Size,
		})
	}

	return orderBook
}

// validateMarketData validates market data for obvious errors only
func (mds *MarketDataService) validateMarketData(data *MarketData) error {
	// Price validation - only check for obviously invalid values
	if err := mds.client.validator.ValidatePrice(data.Symbol, data.Price); err != nil {
		return err
	}

	// Volume validation - only check for negative values
	if err := mds.client.validator.ValidateVolume(data.Symbol, data.Volume); err != nil {
		return err
	}

	// Spread validation - only check if both bid and ask are present
	// Allow cases where one or both are zero (data may not always be available)
	if data.BestBid > 0 && data.BestAsk > 0 && data.BestBid >= data.BestAsk {
		return fmt.Errorf("invalid spread: bid=%f >= ask=%f", data.BestBid, data.BestAsk)
	}

	// Timestamp validation
	if data.Timestamp.IsZero() {
		return fmt.Errorf("invalid timestamp")
	}

	return nil
}

// saveMarketDataToDB ismarket data" "databaseに保存する
func (mds *MarketDataService) saveMarketDataToDB(marketData *MarketData) {
	if mds.db == nil {
		return
	}

	// history保存is常に有効（history_enabledconfiguration" "削除）
	mds.logger.Data().Debug("Saving market data to database")

	now := time.Now()

	// 最後の保存時刻をチェック（シンボルごとに1分間隔で保存）
	mds.mu.Lock()
	lastSave, exists := mds.lastSaveTime[marketData.Symbol]
	if exists && now.Sub(lastSave) < time.Minute {
		// 前回の保存から1分未満の場合はスキップ
		mds.mu.Unlock()
		return
	}
	// 保存時刻を更新
	mds.lastSaveTime[marketData.Symbol] = now
	mds.mu.Unlock()

	// MarketData " " database.MarketData に変換
	dbData := database.MarketData{
		Symbol:      marketData.Symbol,
		ProductCode: marketData.Symbol,
		Timestamp:   marketData.Timestamp,
		Open:        marketData.Price, // ティッカーdataなofwith全て同じprice
		High:        marketData.Price,
		Low:         marketData.Price,
		Close:       marketData.Price,
		Volume:      marketData.Volume,
		CreatedAt:   now,
	}

	// 非同期withdatabaseに保存
	go func() {
		if err := mds.db.SaveMarketData(&dbData); err != nil {
			mds.logger.Data().WithError(err).Error("Failed to save market data to database")
		} else {
			mds.logger.Data().WithField("symbol", dbData.Symbol).WithField("timestamp", dbData.Timestamp).Debug("Market data saved to database")
		}
	}()
}
