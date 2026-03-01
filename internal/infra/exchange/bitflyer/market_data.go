package bitflyer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/go-bitflyer-api-client/client/websocket"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// Deprecated: This constant is now configured via config.yaml (worker.max_concurrent_saves)
// It is kept for backward compatibility but will be removed in v2.0
const (
	// MaxConcurrentSaves is deprecated, use config.Worker.MaxConcurrentSaves
	MaxConcurrentSaves = 10
)

// MarketData represents bitFlyer-specific market data with additional fields
// Use ToDomain() to convert to domain.MarketData
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

	// Additional bitFlyer-specific information
	VolumeByProduct float64 `json:"volume_by_product,omitempty"`
	TradeCount      int     `json:"trade_count,omitempty"`
}

// ToDomain converts bitFlyer MarketData to domain.MarketData
func (m *MarketData) ToDomain() domain.MarketData {
	return domain.MarketData{
		Symbol:      m.Symbol,
		ProductCode: m.Symbol,
		Timestamp:   m.Timestamp,
		Price:       m.Price,
		Volume:      m.Volume,
		BestBid:     m.BestBid,
		BestAsk:     m.BestAsk,
		Spread:      m.Spread,
		Open:        m.Open,
		High:        m.High,
		Low:         m.Low,
		Close:       m.Close,
	}
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
	logger             logger.LoggerInterface
	tickerCallbacks    map[string]func(MarketData)
	executionCallbacks map[string]func([]Execution)
	boardCallbacks     map[string]func(OrderBook)
	db                 MarketDataSaver      // database saving interface
	config             *MarketDataConfig    // configuration
	lastSaveTime       map[string]time.Time // Last time data was saved (per symbol)
	mu                 sync.RWMutex         // Protects tickerCallbacks, executionCallbacks, boardCallbacks, callbacksInit, lastSaveTime
	callbacksInit      bool                 // Whether global callbacks have been initialized
	saveSemaphore      chan struct{}        // Limit concurrent database saves (max 10)
	shutdownCtx        context.Context      // Context for graceful shutdown of background goroutines
}

// MarketDataConfig ismarket dataconfiguration
type MarketDataConfig struct {
	HistoryDays int `yaml:"history_days"`
}

// MarketDataSaver is an interface for saving market data
type MarketDataSaver interface {
	SaveMarketData(data *domain.MarketData) error
}

// NewMarketDataService creates a new market data service with all dependencies
func NewMarketDataService(client *Client, log logger.LoggerInterface, db MarketDataSaver, config *MarketDataConfig, maxConcurrentSaves int) *MarketDataService {
	if config == nil {
		config = &MarketDataConfig{
			HistoryDays: 365, // default 1 year
		}
	}

	if maxConcurrentSaves <= 0 {
		maxConcurrentSaves = MaxConcurrentSaves // Use default if invalid
	}

	return &MarketDataService{
		client:        client,
		logger:        log,
		db:            db,
		config:        config,
		lastSaveTime:  make(map[string]time.Time),
		saveSemaphore: make(chan struct{}, maxConcurrentSaves),
		shutdownCtx:   context.Background(), // Initialize with background context (will be updated on first Subscribe)
	}
}

// GetTicker gets ticker information
func (mds *MarketDataService) GetTicker(ctx context.Context, symbol string) (*MarketData, error) {
	// Check rate limiting
	if err := mds.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// API call
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

	// Data conversion
	ticker := resp.JSON200

	// Helper function to safely get value from pointer
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

	// Data validation
	if err := mds.validateMarketData(marketData); err != nil {
		return nil, fmt.Errorf("invalid market data: %w", err)
	}

	return marketData, nil
}

// GetOrderBook gets order book information
func (mds *MarketDataService) GetOrderBook(ctx context.Context, symbol string) (*OrderBook, error) {
	// Check rate limiting
	if err := mds.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// API call
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

	// Data conversion
	board := resp.JSON200
	orderBook := &OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	// Helper function to safely get value from pointer
	safeFloat64 := func(ptr *float32) float64 {
		if ptr == nil {
			return 0
		}
		return float64(*ptr)
	}

	// Convert Bids
	if board.Bids != nil {
		for _, bid := range *board.Bids {
			orderBook.Bids = append(orderBook.Bids, OrderItem{
				Price: safeFloat64(bid.Price),
				Size:  safeFloat64(bid.Size),
			})
		}
	}

	// Convert Asks
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

// GetExecutions gets execution history
func (mds *MarketDataService) GetExecutions(ctx context.Context, symbol string, count int) ([]Execution, error) {
	// Check rate limiting
	if err := mds.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// Configure parameters
	params := &http.GetV1GetexecutionsParams{
		ProductCode: symbol,
	}
	if count > 0 {
		params.Count = &count
	}

	// API call
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

	// Data conversion
	var executions []Execution
	for _, exec := range *resp.JSON200 {
		// Helper function to safely get value from pointer
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

		// Convert ExecDate to time
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

// SubscribeToTicker subscribes to real-time ticker data
func (mds *MarketDataService) SubscribeToTicker(ctx context.Context, symbol string, callback func(MarketData)) error {
	if !mds.client.IsConnected() {
		return fmt.Errorf("websocket client is not connected")
	}

	// Store shutdown context for background goroutines
	mds.shutdownCtx = ctx

	// Set up global handler only on first call
	mds.mu.Lock()
	if !mds.callbacksInit {
		mds.callbacksInit = true
		mds.tickerCallbacks = make(map[string]func(MarketData))
		mds.mu.Unlock() // Unlock before setting up callback to avoid holding lock during WebSocket operations

		// Set up global ticker handler (for all symbols)
		mds.client.wsClient.OnTicker(func(ticker websocket.TickerMessage) {
			mds.logger.Data().WithField("symbol", ticker.ProductCode).WithField("price", ticker.Ltp).Info("Ticker data received from WebSocket")

			// Execute callback for the matching symbol
			mds.mu.RLock()
			cb, exists := mds.tickerCallbacks[ticker.ProductCode]
			mds.mu.RUnlock()
			if exists {
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

				// Data validation
				if err := mds.validateMarketData(&marketData); err != nil {
					mds.logger.Data().WithError(err).Error("Invalid ticker data received")
					return
				}

				mds.logger.Data().WithField("symbol", marketData.Symbol).WithField("price", marketData.Price).Info("Market data processed and calling callback")

				// Save market data to database (asynchronously)
				mds.saveMarketDataToDB(&marketData)

				cb(marketData)
			} else {
				mds.logger.Data().WithField("symbol", ticker.ProductCode).Info("No callback registered for this symbol")
			}
		})

		mds.logger.Data().Info("Global ticker handler initialized")
	} else {
		mds.mu.Unlock()
	}

	// Register callback for the symbol
	mds.mu.Lock()
	mds.tickerCallbacks[symbol] = callback
	mds.mu.Unlock()
	mds.logger.Data().WithField("symbol", symbol).Info("Ticker callback registered")

	// Subscribe to channel
	channel := fmt.Sprintf("lightning_ticker_%s", symbol)
	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Attempting to subscribe to ticker channel")

	if err := mds.client.wsClient.Subscribe(ctx, channel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", channel).WithField("symbol", symbol).Error("Failed to subscribe to ticker channel")
		return fmt.Errorf("failed to subscribe to ticker channel %s: %w", channel, err)
	}

	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Successfully subscribed to ticker channel")
	return nil
}

// SubscribeToExecutions subscribes to real-time execution data
func (mds *MarketDataService) SubscribeToExecutions(ctx context.Context, symbol string, callback func([]Execution)) error {
	if !mds.client.IsConnected() {
		return fmt.Errorf("websocket client is not connected")
	}

	// Set up global handler only on first call
	mds.mu.Lock()
	if mds.executionCallbacks == nil {
		mds.executionCallbacks = make(map[string]func([]Execution))
		mds.mu.Unlock()

		// Set up global execution handler (for all symbols)
		mds.client.wsClient.OnExecutions(func(execMsg websocket.ExecutionsMessage) {
			mds.logger.Data().WithField("symbol", execMsg.ProductCode).Info("Execution data received from WebSocket")

			// Execute callback for the matching symbol
			mds.mu.RLock()
			cb, exists := mds.executionCallbacks[execMsg.ProductCode]
			mds.mu.RUnlock()
			if exists {
				var executions []Execution
				for _, exec := range execMsg.Executions {
					// Convert ExecDate to time
					execTime, err := time.Parse(time.RFC3339, exec.ExecDate)
					if err != nil {
						execTime = time.Now() // Use current time if parsing fails
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
	} else {
		mds.mu.Unlock()
	}

	// Register callback for the symbol
	mds.mu.Lock()
	mds.executionCallbacks[symbol] = callback
	mds.mu.Unlock()
	mds.logger.Data().WithField("symbol", symbol).Info("Execution callback registered")

	// Subscribe to channel
	channel := fmt.Sprintf("lightning_executions_%s", symbol)
	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Attempting to subscribe to executions channel")

	if err := mds.client.wsClient.Subscribe(ctx, channel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", channel).WithField("symbol", symbol).Error("Failed to subscribe to executions channel")
		return fmt.Errorf("failed to subscribe to executions channel %s: %w", channel, err)
	}

	mds.logger.Data().WithField("channel", channel).WithField("symbol", symbol).Info("Successfully subscribed to executions channel")
	return nil
}

// SubscribeToOrderBook subscribes to real-time order book data
func (mds *MarketDataService) SubscribeToOrderBook(ctx context.Context, symbol string, callback func(OrderBook)) error {
	if !mds.client.IsConnected() {
		return fmt.Errorf("websocket client is not connected")
	}

	// Set up global handler only on first call
	mds.mu.Lock()
	if mds.boardCallbacks == nil {
		mds.boardCallbacks = make(map[string]func(OrderBook))
		mds.mu.Unlock() // Unlock before setting up callback to avoid holding lock during WebSocket operations

		// Set up global board handler (for all symbols)
		// Snapshot handler
		mds.client.wsClient.OnBoardSnapshot(func(snapshot websocket.BoardSnapshotMessage) {
			mds.logger.Data().WithField("symbol", snapshot.ProductCode).Info("Board snapshot data received from WebSocket")

			// Execute callback for the matching symbol
			mds.mu.RLock()
			cb, exists := mds.boardCallbacks[snapshot.ProductCode]
			mds.mu.RUnlock()
			if exists {
				orderBook := mds.convertBoardData(snapshot.ProductCode, snapshot.Data)
				cb(orderBook)
			} else {
				mds.logger.Data().WithField("symbol", snapshot.ProductCode).Info("No callback registered for this symbol")
			}
		})

		// Diff handler
		mds.client.wsClient.OnBoard(func(board websocket.BoardMessage) {
			mds.logger.Data().WithField("symbol", board.ProductCode).Debug("Board diff data received from WebSocket")

			// Execute callback for the matching symbol
			mds.mu.RLock()
			cb, exists := mds.boardCallbacks[board.ProductCode]
			mds.mu.RUnlock()
			if exists {
				orderBook := mds.convertBoardData(board.ProductCode, board.Data)
				cb(orderBook)
			}
		})

		mds.logger.Data().Info("Global board handler initialized")
	} else {
		mds.mu.Unlock()
	}

	// Register callback for the symbol
	mds.mu.Lock()
	mds.boardCallbacks[symbol] = callback
	mds.mu.Unlock()
	mds.logger.Data().WithField("symbol", symbol).Info("Board callback registered")

	// Snapshot channel
	snapshotChannel := fmt.Sprintf("lightning_board_snapshot_%s", symbol)
	mds.logger.Data().WithField("channel", snapshotChannel).WithField("symbol", symbol).Info("Attempting to subscribe to board snapshot channel")

	if err := mds.client.wsClient.Subscribe(ctx, snapshotChannel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", snapshotChannel).WithField("symbol", symbol).Error("Failed to subscribe to board snapshot channel")
		return fmt.Errorf("failed to subscribe to board snapshot channel: %w", err)
	}

	// Diff channel
	diffChannel := fmt.Sprintf("lightning_board_%s", symbol)
	mds.logger.Data().WithField("channel", diffChannel).WithField("symbol", symbol).Info("Attempting to subscribe to board diff channel")

	if err := mds.client.wsClient.Subscribe(ctx, diffChannel); err != nil {
		mds.logger.Data().WithError(err).WithField("channel", diffChannel).WithField("symbol", symbol).Error("Failed to subscribe to board diff channel")
		return fmt.Errorf("failed to subscribe to board diff channel: %w", err)
	}

	mds.logger.Data().WithField("symbol", symbol).Info("Successfully subscribed to order book channels")
	return nil
}

// convertBoardData converts board data
func (mds *MarketDataService) convertBoardData(symbol string, data websocket.BoardData) OrderBook {
	orderBook := OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	// Convert Bids
	for _, bid := range data.Bids {
		orderBook.Bids = append(orderBook.Bids, OrderItem{
			Price: bid.Price,
			Size:  bid.Size,
		})
	}

	// Convert Asks
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

// saveMarketDataToDB saves market data to the database
func (mds *MarketDataService) saveMarketDataToDB(marketData *MarketData) {
	if mds.db == nil {
		return
	}

	// History saving is always enabled (history_enabled configuration removed)
	mds.logger.Data().Debug("Saving market data to database")

	now := time.Now()

	// Check last save time (save at 1-minute intervals per symbol)
	mds.mu.Lock()
	lastSave, exists := mds.lastSaveTime[marketData.Symbol]
	if exists && now.Sub(lastSave) < time.Minute {
		// Skip if less than 1 minute since last save
		mds.mu.Unlock()
		return
	}
	// Update save time
	mds.lastSaveTime[marketData.Symbol] = now
	mds.mu.Unlock()

	// Convert MarketData to domain.MarketData
	dbData := domain.MarketData{
		Symbol:      marketData.Symbol,
		ProductCode: marketData.Symbol,
		Timestamp:   marketData.Timestamp,
		Open:        marketData.Price, // For ticker data, all prices are the same
		High:        marketData.Price,
		Low:         marketData.Price,
		Close:       marketData.Price,
		Volume:      marketData.Volume,
	}

	// Save to database asynchronously (with semaphore to limit concurrent saves)
	// Check if shutdown context is still active before spawning goroutine
	if mds.shutdownCtx == nil {
		// Context not initialized, skip save
		return
	}

	select {
	case <-mds.shutdownCtx.Done():
		// Shutdown in progress, skip save
		return
	case mds.saveSemaphore <- struct{}{}:
		go func() {
			defer func() { <-mds.saveSemaphore }()

			// Check context again before database operation
			select {
			case <-mds.shutdownCtx.Done():
				return
			default:
			}

			if err := mds.db.SaveMarketData(&dbData); err != nil {
				mds.logger.Data().WithError(err).Error("Failed to save market data to database")
			} else {
				mds.logger.Data().WithField("symbol", dbData.Symbol).WithField("timestamp", dbData.Timestamp).Debug("Market data saved to database")
			}
		}()
	default:
		mds.logger.Data().Warn("Market data save queue full, dropping data to prevent goroutine leak")
	}
}

// ResetCallbacks clears all callback maps and resets initialization flag
// This should be called when recreating the WebSocket client to prevent callback leaks
func (mds *MarketDataService) ResetCallbacks() {
	mds.mu.Lock()
	defer mds.mu.Unlock()

	mds.tickerCallbacks = nil
	mds.executionCallbacks = nil
	mds.boardCallbacks = nil
	mds.callbacksInit = false
	mds.logger.Data().Info("Market data callbacks reset")
}
