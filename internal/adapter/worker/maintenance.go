package worker

import (
	"context"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	"github.com/bmf-san/gogocoin/internal/utils"
)

const maintenanceLastCleanupKey = "maintenance_last_cleanup_date"

// MaintenanceWorker runs daily cleanup tasks
type MaintenanceWorker struct {
	logger        logger.LoggerInterface
	db            domain.MaintenanceRepository
	appState      domain.AppStateRepository
	retentionDays int
	checkInterval time.Duration
}

// NewMaintenanceWorker creates a new maintenance worker
func NewMaintenanceWorker(logger logger.LoggerInterface, db domain.MaintenanceRepository, retentionDays int, appState domain.AppStateRepository) *MaintenanceWorker {
	return &MaintenanceWorker{
		logger:        logger,
		db:            db,
		appState:      appState,
		retentionDays: retentionDays,
		checkInterval: 10 * time.Minute,
	}
}

// Name returns the worker name.
func (w *MaintenanceWorker) Name() string { return "maintenance-worker" }

// Run starts the maintenance worker (runs daily cleanup at midnight 00:00)
func (w *MaintenanceWorker) Run(ctx context.Context) error {
	w.logger.System().Info("Maintenance worker started (daily cleanup at 00:00)")

	// Track last execution date to prevent duplicates; restore from persistent
	// storage so a process restart doesn't trigger a duplicate cleanup.
	lastCleanupDate := ""
	if w.appState != nil {
		if val, err := w.appState.GetAppState(maintenanceLastCleanupKey); err == nil && val != "" {
			lastCleanupDate = val
		}
	}
	// (i.e., started after 00:00 but cleanup hasn't run yet today)
	startupDelay := time.NewTimer(1 * time.Minute)
	// Always drain/stop the timer to prevent leaks.
	defer func() {
		if !startupDelay.Stop() {
			select {
			case <-startupDelay.C:
			default:
			}
		}
	}()

	select {
	case <-startupDelay.C:
		nowAfterDelay := utils.NowInJST()
		// If current time is after 00:10 (10 minute window), run missed cleanup
		if nowAfterDelay.Hour() > 0 || (nowAfterDelay.Hour() == 0 && nowAfterDelay.Minute() >= 10) {
			w.logger.System().Info("Running missed cleanup on startup")
			w.runDailyCleanup()
			today := utils.NowInJST().Format("20060102")
			lastCleanupDate = today
			if w.appState != nil {
				if err := w.appState.SaveAppState(maintenanceLastCleanupKey, today); err != nil {
					w.logger.System().WithError(err).Warn("Failed to persist cleanup date after startup cleanup")
				}
			}
		} else {
			w.logger.System().Info("Skipping startup cleanup - will run at midnight")
		}
	case <-ctx.Done():
		return nil
	}

	// Check every 10 minutes for midnight cleanup
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.System().Info("Maintenance worker stopped")
			return nil
		case <-ticker.C:
			now := utils.NowInJST()
			currentDate := now.Format("20060102") // YYYYMMDD in JST

			// Daily cleanup at JST midnight (00:00, within 10-minute window)
			if now.Hour() == 0 && now.Minute() < 10 && currentDate != lastCleanupDate {
				lastCleanupDate = currentDate
				if w.appState != nil {
					if err := w.appState.SaveAppState(maintenanceLastCleanupKey, currentDate); err != nil {
						w.logger.System().WithError(err).Warn("Failed to persist cleanup date")
					}
				}
				w.runDailyCleanup()
			}
		}
	}
}

// runDailyCleanup executes daily database cleanup (removes old data based on retention policy)
func (w *MaintenanceWorker) runDailyCleanup() {
	w.logger.System().WithField("retention_days", w.retentionDays).Info("Starting daily cleanup")

	// Get database size before cleanup
	sizeBefore, err := w.db.GetDatabaseSize()
	if err != nil {
		w.logger.System().WithError(err).Warn("Failed to get database size before cleanup, continuing anyway")
	}

	// Run cleanup - removes all data older than retention period
	if err := w.db.CleanupOldData(w.retentionDays); err != nil {
		w.logger.System().WithError(err).Error("Daily cleanup failed")
		return
	}

	// Get database size after cleanup
	sizeAfter, err := w.db.GetDatabaseSize()
	if err != nil {
		w.logger.System().WithError(err).Warn("Failed to get database size after cleanup, continuing anyway")
	}
	savedBytes := sizeBefore - sizeAfter
	savedMB := float64(savedBytes) / 1024 / 1024

	w.logger.System().WithField("size_before_mb", float64(sizeBefore)/1024/1024).
		WithField("size_after_mb", float64(sizeAfter)/1024/1024).
		WithField("saved_mb", savedMB).
		Info("Daily cleanup completed successfully")

	// Log table statistics
	stats, err := w.db.GetTableStats()
	if err != nil {
		w.logger.System().WithError(err).Warn("Failed to get table statistics, skipping stats logging")
	} else {
		w.logger.System().WithField("table_stats", stats).Info("Database table statistics after cleanup")
	}
}
