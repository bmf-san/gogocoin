package main

import (
	"context"
	"fmt"
	"time"

	adapterhttp "github.com/bmf-san/gogocoin/v1/internal/adapter/http"
	adapterworker "github.com/bmf-san/gogocoin/v1/internal/adapter/worker"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/infra/exchange/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/infra/persistence"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/usecase/analytics"
	"github.com/bmf-san/gogocoin/v1/internal/usecase/risk"
	"github.com/bmf-san/gogocoin/v1/internal/usecase/strategy"
	trading "github.com/bmf-san/gogocoin/v1/internal/usecase/trading"
	"github.com/bmf-san/gogocoin/v1/internal/utils"
)

// appServiceAdapter wraps TradingController + a trader to satisfy adapter/http.ApplicationService.
type appServiceAdapter struct {
	tc     *TradingController
	trader trading.Trader
	strat  strategy.Strategy
}

func (a *appServiceAdapter) GetBalances(ctx context.Context) ([]domain.Balance, error) {
	return a.trader.GetBalance(ctx)
}
func (a *appServiceAdapter) GetCurrentStrategy() strategy.Strategy { return a.strat }
func (a *appServiceAdapter) IsTradingEnabled() bool                { return a.tc.IsTradingEnabled() }
func (a *appServiceAdapter) SetTradingEnabled(enabled bool) error {
	return a.tc.SetTradingEnabled(enabled)
}

// marketDataAdapter adapts infra bitflyer.MarketDataService to adapterworker.ClientFactory.
type marketDataAdapter struct {
	client        *bitflyer.Client
	marketDataSvc *bitflyer.MarketDataService
}

func (m *marketDataAdapter) IsConnected() bool { return m.client.IsConnected() }

func (m *marketDataAdapter) ReconnectClient() error {
	// Minimal reconnect: the market data service stays the same; in a full
	// implementation this would recreate the client as in ConnectionManager.
	return nil
}

func (m *marketDataAdapter) SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error {
	return m.marketDataSvc.SubscribeToTicker(ctx, symbol, func(bfd bitflyer.MarketData) {
		callback(domain.MarketData{
			Symbol:      bfd.Symbol,
			ProductCode: bfd.Symbol,
			Timestamp:   bfd.Timestamp,
			Price:       bfd.Price,
			Volume:      bfd.Volume,
			BestBid:     bfd.BestBid,
			BestAsk:     bfd.BestAsk,
			Spread:      bfd.Spread,
			Open:        bfd.Open,
			High:        bfd.High,
			Low:         bfd.Low,
			Close:       bfd.Close,
		})
	})
}

// Run wires up and runs the full application until ctx is cancelled.
func Run(ctx context.Context, cfg *config.Config, log logger.LoggerInterface) error {
	// usecase/trading still requires *logger.Logger concretely; assert once here.
	concreteLog, ok := log.(*logger.Logger)
	if !ok {
		return fmt.Errorf("logger must be *logger.Logger for trading wiring")
	}
	// ── 1. Database ──────────────────────────────────────────────────────────
	dbPath := "./data/gogocoin.db"
	db, err := persistence.NewDB(dbPath, log)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	repo := persistence.NewRepository(db)
	log.SetDatabase(repo)
	defer func() {
		log.SetDatabase(nil)
		if err := db.Close(); err != nil {
			log.System().WithError(err).Error("Failed to close database")
		}
	}()

	// ── 2. TradingController ──────────────────────────────────────────────────
	tradingCtrl, err := newTradingController(repo, log)
	if err != nil {
		return fmt.Errorf("failed to create trading controller: %w", err)
	}

	// ── 3. bitFlyer client + services ────────────────────────────────────────
	bfClient, err := bitflyer.NewClient(&bitflyer.Config{
		APIKey:            cfg.API.Credentials.APIKey,
		APISecret:         cfg.API.Credentials.APISecret,
		Endpoint:          cfg.API.Endpoint,
		WebSocketEndpoint: cfg.API.WebSocketEndpoint,
		Timeout:           cfg.API.Timeout,
		RetryCount:        cfg.API.RetryCount,
		RequestsPerMinute: cfg.API.RateLimit.RequestsPerMinute,
		InitialBalance:    cfg.Trading.InitialBalance,
		FeeRate:           cfg.Trading.FeeRate,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to create bitFlyer client: %w", err)
	}
	defer func() { _ = bfClient.Close(context.Background()) }()

	bfMarketDataSvc := bitflyer.NewMarketDataService(
		bfClient, log, repo,
		&bitflyer.MarketDataConfig{HistoryDays: cfg.Data.MarketData.HistoryDays},
		cfg.Worker.MaxConcurrentSaves,
	)
	bfMarketSpecSvc := bitflyer.NewMarketSpecificationService(bfClient)

	// ── 4. Trader ─────────────────────────────────────────────────────────────
	if len(cfg.Trading.Symbols) == 0 {
		return fmt.Errorf("trading.symbols must contain at least one symbol in config")
	}
	primarySymbol := cfg.Trading.Symbols[0]
	trader := trading.NewTraderWithDependencies(bfClient, concreteLog, repo, bfMarketSpecSvc, cfg.Trading.Strategy.Name, primarySymbol)

	// ── 5. Strategy ───────────────────────────────────────────────────────────
	strat, err := buildStrategy(cfg)
	if err != nil {
		return fmt.Errorf("failed to build strategy: %w", err)
	}
	if err := strat.Start(ctx); err != nil {
		return fmt.Errorf("failed to start strategy: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = strat.Stop(stopCtx)
	}()

	// Initialise daily trade count from recent trades
	initDailyTradeCount(repo, strat, log)

	// ── 6. Risk manager ───────────────────────────────────────────────────────
	minInterval, _ := time.ParseDuration(cfg.Trading.RiskManagement.MinTradeInterval)
	riskMgr := risk.NewRiskManager(
		risk.ManagerConfig{
			MaxTotalLossPercent:   cfg.Trading.RiskManagement.MaxTotalLossPercent,
			MaxTradeLossPercent:   cfg.Trading.RiskManagement.MaxTradeLossPercent,
			MaxDailyLossPercent:   cfg.Trading.RiskManagement.MaxDailyLossPercent,
			MaxTradeAmountPercent: cfg.Trading.RiskManagement.MaxTradeAmountPercent,
			MaxDailyTrades:        cfg.Trading.RiskManagement.MaxDailyTrades,
			MinTradeInterval:      minInterval,
			FeeRate:               cfg.Trading.FeeRate,
			InitialBalance:        cfg.Trading.InitialBalance,
		},
		repo, repo, trader, log,
	)

	// ── 7. Performance analytics ──────────────────────────────────────────────
	perfAnalytics := analytics.NewPerformanceAnalytics(repo, repo, log, cfg.Trading.InitialBalance)

	// ── 8. HTTP server ────────────────────────────────────────────────────────
	appSvc := &appServiceAdapter{tc: tradingCtrl, trader: trader, strat: strat}
	httpServer := adapterhttp.NewServer(cfg, repo, log)
	httpServer.SetApplication(appSvc)
	go func() {
		if err := httpServer.Start(); err != nil {
			log.System().Error("HTTP server error", "error", err)
		}
	}()

	// ── 9. Channels ───────────────────────────────────────────────────────────
	marketDataCh := make(chan domain.MarketData, cfg.Worker.MarketDataChannelBuffer)
	signalCh := make(chan *strategy.Signal, cfg.Worker.SignalChannelBuffer)

	// ── 10. Workers ───────────────────────────────────────────────────────────
	clientFactory := &marketDataAdapter{client: bfClient, marketDataSvc: bfMarketDataSvc}

	marketDataWorker := adapterworker.NewMarketDataWorker(
		log,
		cfg.Trading.Symbols,
		marketDataCh,
		clientFactory,
		cfg.Worker.ReconnectIntervalSeconds,
		cfg.Worker.MaxReconnectIntervalSeconds,
		cfg.Worker.ConnectionCheckIntervalSeconds,
		cfg.Worker.StaleDataTimeoutSeconds,
	)
	strategyWorker := adapterworker.NewStrategyWorker(log, strat, marketDataCh, signalCh)
	signalWorker := adapterworker.NewSignalWorker(log, signalCh, tradingCtrl, riskMgr, trader, strat, perfAnalytics, cfg.Runtime.SellSizePercentage)
	strategyMonitor := adapterworker.NewStrategyMonitorWorker(log, &strategyGetter{strat: strat})
	maintenanceWorker := adapterworker.NewMaintenanceWorker(log, repo, cfg.DataRetention.RetentionDays, repo)

	wm := adapterworker.NewWorkerManager(log)

	_ = wm.Register(marketDataWorker.Name(), marketDataWorker)
	_ = wm.Register(strategyWorker.Name(), strategyWorker)
	_ = wm.Register(signalWorker.Name(), signalWorker)
	_ = wm.Register(strategyMonitor.Name(), strategyMonitor)
	_ = wm.Register(maintenanceWorker.Name(), maintenanceWorker)

	if err := wm.StartAll(ctx); err != nil {
		return fmt.Errorf("failed to start workers: %w", err)
	}

	log.System().Info("gogocoin started", "strategy", cfg.Trading.Strategy.Name, "port", cfg.UI.Port)

	// ── 11. Wait for shutdown ─────────────────────────────────────────────────
	<-ctx.Done()
	log.System().Info("Shutdown signal received, stopping...")

	// Shutdown HTTP server first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.System().WithError(err).Error("HTTP server shutdown error")
	}

	// Wait for all workers to exit
	if err := wm.StopAll(); err != nil {
		log.System().WithError(err).Warn("Worker manager shutdown error")
	}

	// Close channels after all workers have stopped
	close(marketDataCh)
	close(signalCh)

	log.System().Info("gogocoin shut down successfully")
	return nil
}

// buildStrategy creates the configured strategy.
func buildStrategy(cfg *config.Config) (strategy.Strategy, error) {
	factory := strategy.NewStrategyFactory()
	params, err := cfg.GetStrategyParams(cfg.Trading.Strategy.Name)
	if err != nil {
		params = nil
	}
	// config.ScalpingParams and strategy.ScalpingParams are structurally identical
	// but different Go types; convert explicitly so the factory type assertion succeeds.
	if cfg.Trading.Strategy.Name == "scalping" {
		if cp, ok := params.(config.ScalpingParams); ok {
			params = strategy.ScalpingParams{
				EMAFastPeriod:  cp.EMAFastPeriod,
				EMASlowPeriod:  cp.EMASlowPeriod,
				TakeProfitPct:  cp.TakeProfitPct,
				StopLossPct:    cp.StopLossPct,
				CooldownSec:    cp.CooldownSec,
				MaxDailyTrades: cp.MaxDailyTrades,
				MinNotional:    cp.MinNotional,
				FeeRate:        cp.FeeRate,
			}
		}
	}
	return factory.CreateStrategy(cfg.Trading.Strategy.Name, params)
}

// initDailyTradeCount restores today's trade count into the strategy.
func initDailyTradeCount(repo *persistence.Repository, strat strategy.Strategy, log logger.LoggerInterface) {
	trades, err := repo.GetRecentTrades(200)
	if err != nil {
		log.System().WithError(err).Warn("Failed to load recent trades for daily count init")
		return
	}
	now := utils.NowInJST()
	todayY, todayM, todayD := now.Date()
	for i := range trades {
		t := utils.ToJST(trades[i].ExecutedAt)
		y, m, d := t.Date()
		if y == todayY && m == todayM && d == todayD {
			strat.RecordTrade()
		}
	}
}

// strategyGetter wraps a Strategy to satisfy adapterworker.StrategyGetter.
type strategyGetter struct{ strat strategy.Strategy }

func (sg *strategyGetter) GetCurrentStrategy() strategy.Strategy { return sg.strat }
