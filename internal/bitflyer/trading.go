package bitflyer

import (
	"context"
	"fmt"
	"time"

	"github.com/bmf-san/go-bitflyer-api-client/client/http"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// OrderRequest represents an order request
type OrderRequest struct {
	Symbol         string  `json:"symbol"`
	Side           string  `json:"side"` // "BUY" or "SELL"
	Type           string  `json:"type"` // "MARKET" or "LIMIT"
	Size           float64 `json:"size"`
	Price          float64 `json:"price,omitempty"`
	TimeInForce    string  `json:"time_in_force,omitempty"` // "GTC", "IOC", "FOK"
	MinuteToExpire int     `json:"minute_to_expire,omitempty"`
}

// OrderResult represents an order result
type OrderResult struct {
	OrderID         string    `json:"order_id"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	Type            string    `json:"type"`
	Size            float64   `json:"size"`
	Price           float64   `json:"price"`
	Status          string    `json:"status"`
	FilledSize      float64   `json:"filled_size"`
	RemainingSize   float64   `json:"remaining_size"`
	AveragePrice    float64   `json:"average_price"`
	TotalCommission float64   `json:"total_commission"`
	Fee             float64   `json:"fee"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Balance represents balance information
type Balance struct {
	Currency  string    `json:"currency"`
	Amount    float64   `json:"amount"`
	Available float64   `json:"available"`
	Timestamp time.Time `json:"timestamp"`
}

// Position isposition情報represents
type Position struct {
	Symbol              string    `json:"symbol"`
	Side                string    `json:"side"`
	Size                float64   `json:"size"`
	Price               float64   `json:"price"`
	Commission          float64   `json:"commission"`
	SwapPointAccumulate float64   `json:"swap_point_accumulate"`
	RequireCollateral   float64   `json:"require_collateral"`
	OpenDate            time.Time `json:"open_date"`
	Leverage            float64   `json:"leverage"`
	Pnl                 float64   `json:"pnl"`
}

// TradingService is the trading service
type TradingService struct {
	client       *Client
	logger       *logger.Logger
	paperTrade   bool
	paperOrders  map[string]*OrderResult
	paperBalance map[string]*Balance
	db           DatabaseSaver // database saving interface
	strategyName string        // 現在ofstrategy名
}

// DatabaseSaver isdatabase保存ofインターフェース
type DatabaseSaver interface {
	SaveTrade(trade *database.Trade) error
	SavePosition(position *database.Position) error
	SaveBalance(balance database.Balance) error
	GetRecentTrades(limit int) ([]database.Trade, error)
}

// NewTradingService is新しいtradingservicecreates
func NewTradingService(client *Client, log *logger.Logger, paperTrade bool) *TradingService {
	ts := &TradingService{
		client:     client,
		logger:     log,
		paperTrade: paperTrade,
	}

	if paperTrade {
		ts.paperOrders = make(map[string]*OrderResult)
		ts.paperBalance = make(map[string]*Balance)
		ts.initializePaperBalance()
	}

	return ts
}

// initializePaperBalance ispapertrade用of初期balancesets
func (ts *TradingService) initializePaperBalance() {
	// configurationfrom初期balanceget
	initialBalance := ts.client.config.InitialBalance
	if initialBalance <= 0 {
		// default値: 100万円
		initialBalance = 1000000.0
	}

	ts.paperBalance["JPY"] = &Balance{
		Currency:  "JPY",
		Amount:    initialBalance,
		Available: initialBalance,
	}

	// 暗号通貨of初期balanceis0from始める（実際oftradingフローに近い）
	cryptoCurrencies := []string{"BTC", "ETH", "XRP", "XLM", "MONA"}
	for _, currency := range cryptoCurrencies {
		ts.paperBalance[currency] = &Balance{
			Currency:  currency,
			Amount:    0.0,
			Available: 0.0,
		}
	}

	// タイムスタンプ" "configuration
	now := time.Now()
	for _, balance := range ts.paperBalance {
		balance.Timestamp = now
	}

	ts.logger.Trading().Info("Paper trading balances initialized",
		"jpy_balance", initialBalance,
		"crypto_currencies", cryptoCurrencies)

	// 初期balance" "databaseに保存
	ts.savePaperBalanceToDB()
}

// PlaceOrder isorderexecutes
func (ts *TradingService) PlaceOrder(ctx context.Context, order *OrderRequest) (*OrderResult, error) {
	// 入力検証
	if err := ts.validateOrder(order); err != nil {
		return nil, fmt.Errorf("invalid order: %w", err)
	}

	if ts.paperTrade {
		return ts.placePaperOrder(ctx, order)
	}

	return ts.placeLiveOrder(ctx, order)
}

// placeLiveOrder is実際oforderexecutes
func (ts *TradingService) placeLiveOrder(ctx context.Context, order *OrderRequest) (*OrderResult, error) {
	// rate limitingチェック
	if err := ts.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// balanceチェック
	balances, err := ts.getLiveBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	if err := ts.checkLiveBalance(order, balances); err != nil {
		return nil, fmt.Errorf("insufficient balance: %w", err)
	}

	start := time.Now()

	// orderタイプ" "変換
	var childOrderType http.NewOrderRequestChildOrderType
	switch order.Type {
	case "LIMIT":
		childOrderType = http.NewOrderRequestChildOrderTypeLIMIT
	case "MARKET":
		childOrderType = http.NewOrderRequestChildOrderTypeMARKET
	default:
		childOrderType = http.NewOrderRequestChildOrderTypeMARKET
	}

	// 売買方向" "変換
	var side http.NewOrderRequestSide
	switch order.Side {
	case "BUY":
		side = http.NewOrderRequestSideBUY
	case "SELL":
		side = http.NewOrderRequestSideSELL
	default:
		side = http.NewOrderRequestSideBUY
	}

	// orderリクエスト" "構築
	requestBody := http.PostV1MeSendchildorderJSONRequestBody{
		ProductCode:    order.Symbol,
		ChildOrderType: childOrderType,
		Side:           side,
		Size:           float32(order.Size),
	}

	if order.Type == "LIMIT" {
		price := float32(order.Price)
		requestBody.Price = &price
	}

	if order.TimeInForce != "" {
		var timeInForce http.NewOrderRequestTimeInForce
		switch order.TimeInForce {
		case "GTC":
			timeInForce = http.NewOrderRequestTimeInForceGTC
		case "IOC":
			timeInForce = http.NewOrderRequestTimeInForceIOC
		case "FOK":
			timeInForce = http.NewOrderRequestTimeInForceFOK
		default:
			timeInForce = http.NewOrderRequestTimeInForceIOC
		}
		requestBody.TimeInForce = &timeInForce
	}

	if order.MinuteToExpire > 0 {
		requestBody.MinuteToExpire = &order.MinuteToExpire
	}

	// API呼び出し
	resp, err := ts.client.httpClient.PostV1MeSendchildorderWithResponse(ctx, requestBody)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		ts.logger.LogAPICall("POST", "/v1/me/sendchildorder", duration, 0, err)
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	ts.logger.LogAPICall("POST", "/v1/me/sendchildorder", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

	// order結果" "構築
	var orderID string
	if resp.JSON200.ChildOrderAcceptanceId != nil {
		orderID = *resp.JSON200.ChildOrderAcceptanceId
	}

	result := &OrderResult{
		OrderID:       orderID,
		Symbol:        order.Symbol,
		Side:          order.Side,
		Type:          order.Type,
		Size:          order.Size,
		Price:         order.Price,
		Status:        "ACTIVE",
		FilledSize:    0,
		RemainingSize: order.Size,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	ts.logger.LogTrade("ORDER_PLACED", order.Symbol, order.Price, order.Size, map[string]interface{}{
		"order_id": result.OrderID,
		"side":     order.Side,
		"type":     order.Type,
	})

	// 約定確認と状態更新" "非同期with実行
	go ts.monitorOrderExecution(ctx, result)

	return result, nil
}

// placePaperOrder ispapertradeoforderexecutes
func (ts *TradingService) placePaperOrder(ctx context.Context, order *OrderRequest) (*OrderResult, error) {
	// balanceチェック
	if err := ts.checkPaperBalance(order); err != nil {
		return nil, fmt.Errorf("insufficient balance: %w", err)
	}

	// orderID" "生成
	orderID := fmt.Sprintf("paper_%d", time.Now().UnixNano())

	// orderタイプに応じて処理" "分岐
	var status string
	var filledSize, remainingSize float64

	if order.Type == "MARKET" {
		// マーケットorderis即座に約定
		status = "COMPLETED"
		filledSize = order.Size
		remainingSize = 0
	} else {
		// リミットorderis一定時間アクティブ状態" "保つ
		status = "ACTIVE"
		filledSize = 0
		remainingSize = order.Size
	}

	// 手数料" "計算（約定時ofみ）
	var fee float64
	if status == "COMPLETED" {
		feeRate := ts.client.config.FeeRate
		if feeRate <= 0 {
			feeRate = 0.0015 // default: bitFlyer標準手数料0.15%
		}
		fee = filledSize * order.Price * feeRate
	}

	// order結果" "作成
	result := &OrderResult{
		OrderID:         orderID,
		Symbol:          order.Symbol,
		Side:            order.Side,
		Type:            order.Type,
		Size:            order.Size,
		Price:           order.Price,
		Status:          status,
		FilledSize:      filledSize,
		RemainingSize:   remainingSize,
		AveragePrice:    order.Price,
		TotalCommission: fee,
		Fee:             fee,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// order" "記録
	ts.paperOrders[orderID] = result

	// マーケットorderof場合is即座にbalance更新とtrading記録
	if status == "COMPLETED" {
		// paperbalance" "更新（手数料" "考慮）
		ts.updatePaperBalance(order, result.Fee)

		// balance" "databaseに保存
		ts.savePaperBalanceToDB()

		// databaseにtrading" "保存
		ts.saveTradeToDB(result)
	} else {
		// リミットorderof場合is非同期with約定処理
		go ts.monitorPaperOrderExecution(ctx, result)
	}

	ts.logger.LogTrade("PAPER_ORDER", order.Symbol, order.Price, order.Size, map[string]interface{}{
		"order_id": orderID,
		"side":     order.Side,
		"type":     order.Type,
		"status":   status,
		"fee":      fee,
	})

	return result, nil
}

// SetDatabase isdatabase saving interfacesets
func (ts *TradingService) SetDatabase(db DatabaseSaver) {
	ts.db = db
}

// SetStrategyName isstrategy名sets
func (ts *TradingService) SetStrategyName(name string) {
	ts.strategyName = name
}

// saveTradeToDB istrading" "databaseに保存する
func (ts *TradingService) saveTradeToDB(result *OrderResult) {
	if ts.db == nil {
		return
	}

	// Strategy名" "動的に取得（未configurationof場合isdefault）
	strategyName := ts.strategyName
	if strategyName == "" {
		strategyName = "unknown"
	}

	// 手数料get（OrderResultfrom、またisconfigurationfrom計算）
	// livemodeと同じ計算式: 約定size × 平均約定price × 手数料率
	fee := result.Fee
	if fee <= 0 {
		// OrderResultに手数料がない場合isconfigurationfrom計算
		feeRate := ts.client.config.FeeRate
		if feeRate <= 0 {
			feeRate = 0.0015 // default: 0.15%
		}
		// 約定priceと約定size" "使用
		avgPrice := result.AveragePrice
		if avgPrice <= 0 {
			avgPrice = result.Price
		}
		filledSize := result.FilledSize
		if filledSize <= 0 {
			filledSize = result.Size
		}
		fee = filledSize * avgPrice * feeRate
	}

	// PnL計算（papermodeofみ、SELL時に計算）
	pnl := 0.0
	if ts.paperTrade && result.Side == "SELL" {
		pnl = ts.calculatePaperPnL(result)
	}

	trade := database.Trade{
		Symbol:       result.Symbol,
		ProductCode:  result.Symbol,
		Side:         result.Side,
		Type:         result.Type,
		Size:         result.Size,
		Price:        result.Price,
		Fee:          fee,
		Status:       result.Status,
		OrderID:      result.OrderID,
		ExecutedAt:   result.UpdatedAt,
		CreatedAt:    result.CreatedAt,
		UpdatedAt:    result.UpdatedAt,
		StrategyName: strategyName,
		Strategy:     strategyName,
		PnL:          pnl,
	}

	// 非同期withdatabaseに保存
	go func() {
		if err := ts.db.SaveTrade(&trade); err != nil {
			ts.logger.Trading().WithError(err).Error("Failed to save trade to database")
		} else {
			ts.logger.Trading().WithField("trade_id", trade.ID).Info("Trade saved to database")
		}
	}()
}

// calculatePaperPnL ispapermodewithofPnLcalculates
// SELL時に直前ofBUYtrading" "参照してPnL" "計算
func (ts *TradingService) calculatePaperPnL(sellOrder *OrderResult) float64 {
	if ts.db == nil {
		return 0
	}

	// 同じシンボルof直前ofBUYtradingget
	trades, err := ts.db.GetRecentTrades(100) // 最近100件get
	if err != nil {
		ts.logger.Trading().WithError(err).Error("Failed to get recent trades for PnL calculation")
		return 0
	}

	// 直前ofBUYtrading" "探す（同じシンボルwith、SELL前oftrading）
	var buyPrice float64
	var buyFee float64
	found := false

	for i := range trades {
		trade := &trades[i]
		// 同じシンボルofBUYtrading" "探す（SELLtradingより前of時刻）
		if trade.Symbol == sellOrder.Symbol && trade.Side == "BUY" &&
			trade.ExecutedAt.Before(sellOrder.CreatedAt) {
			buyPrice = trade.Price
			buyFee = trade.Fee
			found = true
			break
		}
	}

	if !found {
		// 対応するBUYtradingが見つfromない場合isPnL=0
		ts.logger.Trading().
			WithField("symbol", sellOrder.Symbol).
			Warn("No matching BUY trade found for PnL calculation")
		return 0
	}

	// PnL計算
	// PnL = (売値 - 買値) × size - (買い手数料 + 売り手数料)
	sellPrice := sellOrder.AveragePrice
	if sellPrice <= 0 {
		sellPrice = sellOrder.Price
	}
	size := sellOrder.FilledSize
	if size <= 0 {
		size = sellOrder.Size
	}
	sellFee := sellOrder.Fee

	priceDiff := (sellPrice - buyPrice) * size
	totalFee := buyFee + sellFee
	pnl := priceDiff - totalFee

	ts.logger.Trading().
		WithField("symbol", sellOrder.Symbol).
		WithField("buy_price", buyPrice).
		WithField("sell_price", sellPrice).
		WithField("size", size).
		WithField("buy_fee", buyFee).
		WithField("sell_fee", sellFee).
		WithField("price_diff", priceDiff).
		WithField("total_fee", totalFee).
		WithField("pnl", pnl).
		Info("Calculated paper trade PnL")

	return pnl
}

// monitorPaperOrderExecution ispaperorderof約定" "監視する
func (ts *TradingService) monitorPaperOrderExecution(ctx context.Context, result *OrderResult) {
	// 5-30秒後にランダムに約定させる（リアルなtrading所" "模倣）
	executionDelay := time.Duration(5+time.Now().UnixNano()%25) * time.Second

	select {
	case <-ctx.Done():
		// コンテキストがキャンセルされた場合
		ts.logger.Trading().WithField("order_id", result.OrderID).Info("Paper order monitoring canceled")
		return
	case <-time.After(executionDelay):
		// 約定処理
		ts.executePaperOrder(result)
	}
}

// executePaperOrder ispaperorder" "約定させる
func (ts *TradingService) executePaperOrder(result *OrderResult) {
	// order状態" "更新
	result.Status = "COMPLETED"
	result.FilledSize = result.Size
	result.RemainingSize = 0
	result.UpdatedAt = time.Now()

	// 手数料" "計算
	feeRate := ts.client.config.FeeRate
	if feeRate <= 0 {
		feeRate = 0.0015 // default: bitFlyer標準手数料0.15%
	}
	fee := result.FilledSize * result.AveragePrice * feeRate
	result.Fee = fee
	result.TotalCommission = fee

	// orderリクエスト" "再構築（balance更新用）
	order := &OrderRequest{
		Symbol: result.Symbol,
		Side:   result.Side,
		Type:   result.Type,
		Size:   result.Size,
		Price:  result.Price,
	}

	// paperbalance" "更新
	ts.updatePaperBalance(order, fee)

	// balance" "databaseに保存
	ts.savePaperBalanceToDB()

	// databaseにtrading" "保存
	ts.saveTradeToDB(result)

	ts.logger.Trading().WithField("order_id", result.OrderID).
		WithField("execution_delay", time.Since(result.CreatedAt)).
		Info("Paper order executed")
}

// CancelOrder isorder" "キャンセルする
func (ts *TradingService) CancelOrder(ctx context.Context, orderID string) error {
	if ts.paperTrade {
		return ts.cancelPaperOrder(orderID)
	}

	return ts.cancelLiveOrder(ctx, orderID)
}

// cancelLiveOrder is実際oforder" "キャンセルする
func (ts *TradingService) cancelLiveOrder(ctx context.Context, orderID string) error {
	// rate limitingチェック
	if err := ts.client.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// キャンセルリクエスト
	requestBody := http.PostV1MeCancelchildorderJSONRequestBody{
		ChildOrderAcceptanceId: &orderID,
	}

	resp, err := ts.client.httpClient.PostV1MeCancelchildorderWithResponse(ctx, requestBody)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		ts.logger.LogAPICall("POST", "/v1/me/cancelchildorder", duration, 0, err)
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	ts.logger.LogAPICall("POST", "/v1/me/cancelchildorder", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	ts.logger.Trading().WithField("order_id", orderID).Info("Order canceled successfully")
	return nil
}

// cancelPaperOrder ispapertradeoforder" "キャンセルする
func (ts *TradingService) cancelPaperOrder(orderID string) error {
	if order, exists := ts.paperOrders[orderID]; exists {
		if order.Status == "ACTIVE" {
			order.Status = "CANCELED"
			order.UpdatedAt = time.Now()
			ts.logger.Trading().WithField("order_id", orderID).Info("Paper order canceled")
		}
		return nil
	}

	return fmt.Errorf("order not found: %s", orderID)
}

// GetBalance represents balance informationgets
func (ts *TradingService) GetBalance(ctx context.Context) ([]Balance, error) {
	if ts.paperTrade {
		return ts.getPaperBalance(), nil
	}

	return ts.getLiveBalance(ctx)
}

// GetOrders isorder一覧gets
func (ts *TradingService) GetOrders(ctx context.Context) ([]*OrderResult, error) {
	if ts.paperTrade {
		return ts.getPaperOrders(), nil
	}

	return ts.getLiveOrders(ctx)
}

// getPaperOrders ispaperorder一覧gets
func (ts *TradingService) getPaperOrders() []*OrderResult {
	var orders []*OrderResult
	for _, order := range ts.paperOrders {
		orders = append(orders, order)
	}
	return orders
}

// getLiveOrders is実際oforder一覧gets
func (ts *TradingService) getLiveOrders(ctx context.Context) ([]*OrderResult, error) {
	// rate limitingチェック
	if err := ts.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// 最新100件oforderget
	count := http.Count(100)
	params := &http.GetV1MeGetchildordersParams{
		Count: &count,
	}

	resp, err := ts.client.httpClient.GetV1MeGetchildordersWithResponse(ctx, params)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		ts.logger.LogAPICall("GET", "/v1/me/getchildorders", duration, 0, err)
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	ts.logger.LogAPICall("GET", "/v1/me/getchildorders", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return []*OrderResult{}, nil
	}

	// レスポンス" "OrderResultに変換
	var orders []*OrderResult
	for i := range *resp.JSON200 {
		order := &(*resp.JSON200)[i]
		if order.ChildOrderAcceptanceId == nil {
			continue
		}

		// 安全にポインタfrom値get
		orderID := *order.ChildOrderAcceptanceId

		var symbol, side, orderType, status string
		var size, price, filledSize, remainingSize, avgPrice, commission float64
		var createdAt, updatedAt time.Time

		if order.ProductCode != nil {
			symbol = string(*order.ProductCode)
		}
		if order.Side != nil {
			side = string(*order.Side)
		}
		if order.ChildOrderType != nil {
			orderType = string(*order.ChildOrderType)
		}
		if order.ChildOrderState != nil {
			switch *order.ChildOrderState {
			case "ACTIVE":
				status = "ACTIVE"
			case "COMPLETED":
				status = "COMPLETED"
			case "CANCELED":
				status = "CANCELED"
			case "EXPIRED":
				status = "EXPIRED"
			case "REJECTED":
				status = "REJECTED"
			default:
				status = "UNKNOWN"
			}
		}
		if order.Size != nil {
			size = float64(*order.Size)
		}
		if order.Price != nil {
			price = float64(*order.Price)
		}
		if order.ExecutedSize != nil {
			filledSize = float64(*order.ExecutedSize)
		}
		if order.Size != nil && order.ExecutedSize != nil {
			remainingSize = float64(*order.Size - *order.ExecutedSize)
		}
		if order.AveragePrice != nil {
			avgPrice = float64(*order.AveragePrice)
		}
		if order.TotalCommission != nil {
			commission = float64(*order.TotalCommission)
		}
		if order.ChildOrderDate != nil {
			createdAt = *order.ChildOrderDate
			updatedAt = *order.ChildOrderDate
		}

		orderResult := &OrderResult{
			OrderID:         orderID,
			Symbol:          symbol,
			Side:            side,
			Type:            orderType,
			Size:            size,
			Price:           price,
			Status:          status,
			FilledSize:      filledSize,
			RemainingSize:   remainingSize,
			AveragePrice:    avgPrice,
			TotalCommission: commission,
			Fee:             commission,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		}

		orders = append(orders, orderResult)
	}

	ts.logger.Trading().Info("Retrieved live orders", "count", len(orders))
	return orders, nil
}

// getLiveBalance is実際ofbalancegets
func (ts *TradingService) getLiveBalance(ctx context.Context) ([]Balance, error) {
	// rate limitingチェック
	if err := ts.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	resp, err := ts.client.httpClient.GetV1MeGetbalanceWithResponse(ctx)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		ts.logger.LogAPICall("GET", "/v1/me/getbalance", duration, 0, err)
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	ts.logger.LogAPICall("GET", "/v1/me/getbalance", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response body")
	}

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

	var balances []Balance
	for _, bal := range *resp.JSON200 {
		balance := Balance{
			Currency:  safeString(bal.CurrencyCode),
			Amount:    safeFloat64(bal.Amount),
			Available: safeFloat64(bal.Available),
		}
		balances = append(balances, balance)
	}

	return balances, nil
}

// getPaperBalance ispapertradeofbalancegets
func (ts *TradingService) getPaperBalance() []Balance {
	var balances []Balance
	for _, balance := range ts.paperBalance {
		balances = append(balances, *balance)
	}
	return balances
}

// validateOrder validates order for basic requirements only
func (ts *TradingService) validateOrder(order *OrderRequest) error {
	if order.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if order.Side != "BUY" && order.Side != "SELL" {
		return fmt.Errorf("invalid side: %s", order.Side)
	}

	if order.Type != "MARKET" && order.Type != "LIMIT" {
		return fmt.Errorf("invalid order type: %s", order.Type)
	}

	if order.Size <= 0 {
		return fmt.Errorf("size must be positive: %f", order.Size)
	}

	if order.Type == "LIMIT" && order.Price <= 0 {
		return fmt.Errorf("price is required for limit order")
	}

	// Price range validation removed - market prices can vary widely
	// API will reject invalid orders anyway

	return nil
}

// checkPaperBalance checks paper trading balance
func (ts *TradingService) checkPaperBalance(order *OrderRequest) error {
	ts.logger.Trading().Debug("Checking paper balance",
		"symbol", order.Symbol,
		"side", order.Side,
		"size", order.Size,
		"price", order.Price)

	if order.Side == "BUY" {
		// 買いorder：JPYbalance" "チェック
		jpyBalance := ts.paperBalance["JPY"]
		requiredAmount := order.Size * order.Price
		ts.logger.Trading().Debug("Buy order balance check",
			"required_amount", requiredAmount,
			"available_jpy", jpyBalance.Available)
		if jpyBalance.Available < requiredAmount {
			return fmt.Errorf("insufficient JPY balance: required %f, available %f",
				requiredAmount, jpyBalance.Available)
		}
	} else {
		// 売りorder：通貨balance" "チェック
		currency := getCurrencyFromSymbol(order.Symbol)
		ts.logger.Trading().Debug("Sell order balance check",
			"currency", currency,
			"symbol", order.Symbol)

		// 利用可能なbalance" "すべてlog出力
		ts.logger.Trading().Debug("Available paper balances")
		for curr, bal := range ts.paperBalance {
			ts.logger.Trading().Debug("Balance entry",
				"currency", curr,
				"amount", bal.Amount,
				"available", bal.Available)
		}

		if balance, exists := ts.paperBalance[currency]; exists {
			ts.logger.Trading().Debug("Currency balance found",
				"currency", currency,
				"required", order.Size,
				"available", balance.Available)
			if balance.Available < order.Size {
				return fmt.Errorf("insufficient %s balance: required %f, available %f",
					currency, order.Size, balance.Available)
			}
		} else {
			return fmt.Errorf("no %s balance found", currency)
		}
	}

	return nil
}

// checkLiveBalance islivemodeofbalance" "チェックする
func (ts *TradingService) checkLiveBalance(order *OrderRequest, balances []Balance) error {
	ts.logger.Trading().Debug("Checking live balance",
		"symbol", order.Symbol,
		"side", order.Side,
		"size", order.Size,
		"price", order.Price)

	if order.Side == "BUY" {
		// 買いorder：JPYbalance" "チェック
		var jpyBalance *Balance
		for _, bal := range balances {
			if bal.Currency == "JPY" {
				jpyBalance = &bal
				break
			}
		}
		if jpyBalance == nil {
			return fmt.Errorf("JPY balance not found")
		}

		requiredAmount := order.Size * order.Price
		ts.logger.Trading().Debug("Buy order balance check",
			"required_amount", requiredAmount,
			"available_jpy", jpyBalance.Available)
		if jpyBalance.Available < requiredAmount {
			return fmt.Errorf("insufficient JPY balance: required %f, available %f",
				requiredAmount, jpyBalance.Available)
		}
	} else {
		// 売りorder：通貨balance" "チェック
		currency := getCurrencyFromSymbol(order.Symbol)
		ts.logger.Trading().Debug("Sell order balance check",
			"currency", currency,
			"symbol", order.Symbol)

		var currencyBalance *Balance
		for _, bal := range balances {
			if bal.Currency == currency {
				currencyBalance = &bal
				break
			}
		}
		if currencyBalance == nil {
			return fmt.Errorf("%s balance not found", currency)
		}

		ts.logger.Trading().Debug("Currency balance found",
			"currency", currency,
			"required", order.Size,
			"available", currencyBalance.Available)
		if currencyBalance.Available < order.Size {
			return fmt.Errorf("insufficient %s balance: required %f, available %f",
				currency, order.Size, currencyBalance.Available)
		}
	}

	return nil
}

// updatePaperBalance ispapertradeofbalanceupdates（手数料考慮）
func (ts *TradingService) updatePaperBalance(order *OrderRequest, fee float64) {
	currency := getCurrencyFromSymbol(order.Symbol)

	if order.Side == "BUY" {
		// 買いorder：JPY" "減らし、通貨" "増やす
		jpyBalance := ts.paperBalance["JPY"]
		currencyBalance := ts.paperBalance[currency]

		amount := order.Size * order.Price
		totalCost := amount + fee // 手数料" "追加

		jpyBalance.Amount -= totalCost
		jpyBalance.Available -= totalCost

		currencyBalance.Amount += order.Size
		currencyBalance.Available += order.Size
	} else {
		// 売りorder：通貨" "減らし、JPY" "増やす
		jpyBalance := ts.paperBalance["JPY"]
		currencyBalance := ts.paperBalance[currency]

		amount := order.Size * order.Price
		netAmount := amount - fee // 手数料" "差し引き

		currencyBalance.Amount -= order.Size
		currencyBalance.Available -= order.Size

		jpyBalance.Amount += netAmount
		jpyBalance.Available += netAmount
	}

	// タイムスタンプ" "更新
	now := time.Now()
	for _, balance := range ts.paperBalance {
		balance.Timestamp = now
	}
}

// savePaperBalanceToDB ispapertradeofbalance" "databaseに保存する
func (ts *TradingService) savePaperBalanceToDB() {
	if ts.db == nil {
		return
	}

	for _, balance := range ts.paperBalance {
		dbBalance := database.Balance{
			Currency:  balance.Currency,
			Available: balance.Available,
			Amount:    balance.Amount,
			Timestamp: balance.Timestamp,
		}
		if err := ts.db.SaveBalance(dbBalance); err != nil {
			ts.logger.Trading().WithField("currency", balance.Currency).WithField("error", err).Error("Failed to save paper balance to database")
		}
	}
}

// getCurrencyFromSymbol isシンボルfrom通貨gets
func getCurrencyFromSymbol(symbol string) string {
	switch symbol {
	case "BTC_JPY":
		return "BTC"
	case "ETH_JPY":
		return "ETH"
	case "XRP_JPY":
		return "XRP"
	default:
		// "_JPY"" "除去して通貨コードget
		if len(symbol) > 4 && symbol[len(symbol)-4:] == "_JPY" {
			return symbol[:len(symbol)-4]
		}
		return symbol
	}
}

// monitorOrderExecution isorderof約定状態" "監視する
func (ts *TradingService) monitorOrderExecution(ctx context.Context, result *OrderResult) {
	// 最大10回、3秒ごとにポーリング（合計30秒）
	maxAttempts := 10
	interval := 3 * time.Second

	for i := 0; i < maxAttempts; i++ {
		select {
		case <-ctx.Done():
			ts.logger.Info("Order monitoring canceled", "order_id", result.OrderID)
			return
		case <-time.After(interval):
			// order状態get
			updated, err := ts.getOrderStatus(ctx, result.OrderID, result.Symbol)
			if err != nil {
				ts.logger.Error(fmt.Sprintf("Failed to get order status: %v", err))
				continue
			}

			// 状態" "更新
			result.Status = updated.Status
			result.FilledSize = updated.FilledSize
			result.RemainingSize = updated.RemainingSize
			result.AveragePrice = updated.AveragePrice
			result.TotalCommission = updated.TotalCommission
			result.Fee = updated.Fee
			result.UpdatedAt = time.Now()

			ts.logger.Info("Order status updated",
				"order_id", result.OrderID,
				"status", result.Status,
				"filled_size", result.FilledSize)

			// 部分約定も含めてtrading" "保存
			if result.FilledSize > 0 {
				if err := ts.saveLiveTrade(result); err != nil {
					ts.logger.Error(fmt.Sprintf("Failed to save trade: %v", err))
				}
			}

			// 完全約定of場合ofみposition更新とbalance更新
			if result.Status == "COMPLETED" || result.FilledSize >= result.Size {
				if err := ts.updateLivePosition(result); err != nil {
					ts.logger.Error(fmt.Sprintf("Failed to update position: %v", err))
				}
				if err := ts.updateLiveBalance(ctx); err != nil {
					ts.logger.Error(fmt.Sprintf("Failed to update balance: %v", err))
				}
				ts.logger.Info("Order fully executed and saved", "order_id", result.OrderID)
				return
			}

			// キャンセルされたら終了
			if result.Status == "CANCELED" || result.Status == "EXPIRED" {
				ts.logger.Info("Order terminated",
					"order_id", result.OrderID,
					"status", result.Status)
				return
			}
		}
	}

	ts.logger.Warn("Order monitoring timeout",
		"order_id", result.OrderID,
		"status", result.Status,
		"filled_size", result.FilledSize)
}

// getOrderStatus isorderof現在of状態gets
func (ts *TradingService) getOrderStatus(ctx context.Context, orderID, symbol string) (*OrderResult, error) {
	if err := ts.client.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()

	// order一覧getしてorderIDwith検索
	productCode := http.ProductCode(symbol)
	params := &http.GetV1MeGetchildordersParams{
		ProductCode:            productCode,
		ChildOrderAcceptanceId: &orderID,
	}

	resp, err := ts.client.httpClient.GetV1MeGetchildordersWithResponse(ctx, params)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		ts.logger.LogAPICall("GET", "/v1/me/getchildorders", duration, 0, err)
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	ts.logger.LogAPICall("GET", "/v1/me/getchildorders", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		// orderが見つfromない場合is、まだ処理中と仮定
		return &OrderResult{
			OrderID:       orderID,
			Status:        "ACTIVE",
			FilledSize:    0,
			RemainingSize: 0,
		}, nil
	}

	// 最初ofマッチするorderget
	order := (*resp.JSON200)[0]

	// 状態" "マッピング
	var status string
	if order.ChildOrderState != nil {
		switch *order.ChildOrderState {
		case "ACTIVE":
			status = "ACTIVE"
		case "COMPLETED":
			status = "COMPLETED"
		case "CANCELED":
			status = "CANCELED"
		case "EXPIRED":
			status = "EXPIRED"
		case "REJECTED":
			status = "REJECTED"
		default:
			status = "ACTIVE"
		}
	} else {
		status = "ACTIVE"
	}

	var price, filledSize, remainingSize, avgPrice, commission float64

	if order.Price != nil {
		price = float64(*order.Price)
	}
	if order.ExecutedSize != nil {
		filledSize = float64(*order.ExecutedSize)
	}
	if order.Size != nil && order.ExecutedSize != nil {
		remainingSize = float64(*order.Size - *order.ExecutedSize)
	}
	if order.AveragePrice != nil {
		avgPrice = float64(*order.AveragePrice)
	}
	if order.TotalCommission != nil {
		commission = float64(*order.TotalCommission)
	}

	return &OrderResult{
		OrderID:         orderID,
		Symbol:          symbol,
		Status:          status,
		Price:           price,
		FilledSize:      filledSize,
		RemainingSize:   remainingSize,
		AveragePrice:    avgPrice,
		TotalCommission: commission,
		Fee:             commission,
		UpdatedAt:       time.Now(),
	}, nil
}

// saveLiveTrade islivetrading" "databaseに保存する
func (ts *TradingService) saveLiveTrade(result *OrderResult) error {
	if ts.db == nil {
		return fmt.Errorf("database not configured")
	}

	trade := &database.Trade{
		OrderID:    result.OrderID,
		Symbol:     result.Symbol,
		Side:       result.Side,
		Type:       result.Type,
		Size:       result.FilledSize,
		Price:      result.AveragePrice,
		Status:     result.Status,
		Fee:        result.TotalCommission,
		ExecutedAt: result.UpdatedAt,
		CreatedAt:  result.CreatedAt,
	}

	if err := ts.db.SaveTrade(trade); err != nil {
		return fmt.Errorf("failed to save trade: %w", err)
	}

	ts.logger.Info("Trade saved to database",
		"order_id", trade.OrderID,
		"symbol", trade.Symbol,
		"side", trade.Side,
		"size", trade.Size,
		"price", trade.Price)

	return nil
}

// updateLivePosition islivepositionupdates
func (ts *TradingService) updateLivePosition(result *OrderResult) error {
	if ts.db == nil {
		return fmt.Errorf("database not configured")
	}

	// position" "作成/更新
	position := &database.Position{
		Symbol:     result.Symbol,
		Side:       result.Side,
		Size:       result.FilledSize,
		EntryPrice: result.AveragePrice,
		Status:     "OPEN",
		CreatedAt:  result.UpdatedAt,
	}

	if err := ts.db.SavePosition(position); err != nil {
		return fmt.Errorf("failed to save position: %w", err)
	}

	ts.logger.Info("Position updated",
		"symbol", position.Symbol,
		"side", position.Side,
		"size", position.Size,
		"entry_price", position.EntryPrice)

	return nil
}

// updateLiveBalance islivebalanceupdates
func (ts *TradingService) updateLiveBalance(ctx context.Context) error {
	if ts.db == nil {
		return fmt.Errorf("database not configured")
	}

	if err := ts.client.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	start := time.Now()
	resp, err := ts.client.httpClient.GetV1MeGetbalanceWithResponse(ctx)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		ts.logger.LogAPICall("GET", "/v1/me/getbalance", duration, 0, err)
		return fmt.Errorf("failed to get balance: %w", err)
	}

	ts.logger.LogAPICall("GET", "/v1/me/getbalance", duration, resp.HTTPResponse.StatusCode, nil)

	if resp.HTTPResponse.StatusCode != 200 {
		return fmt.Errorf("API error: status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return fmt.Errorf("empty response body")
	}

	// balance" "databaseに保存
	for _, bal := range *resp.JSON200 {
		if bal.CurrencyCode == nil {
			continue
		}

		var amount, available float64
		if bal.Amount != nil {
			amount = float64(*bal.Amount)
		}
		if bal.Available != nil {
			available = float64(*bal.Available)
		}

		balance := database.Balance{
			Currency:  *bal.CurrencyCode,
			Amount:    amount,
			Available: available,
			Timestamp: time.Now(),
		}

		if err := ts.db.SaveBalance(balance); err != nil {
			ts.logger.Error(fmt.Sprintf("Failed to save balance for %s: %v", balance.Currency, err))
			continue
		}
	}

	ts.logger.Info("Balance updated from live API")
	return nil
}
