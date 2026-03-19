package api

import (
	"context"
	"time"
)

func svcUnavailableErr(msg string) ServiceUnavailableJSONResponse {
	return ServiceUnavailableJSONResponse{Message: &msg}
}

func internalErr(msg string) InternalServerErrorJSONResponse {
	return InternalServerErrorJSONResponse{Message: &msg}
}

// PostApiTradingStart implements StrictServerInterface - start trading
func (s *Server) PostApiTradingStart(ctx context.Context, request PostApiTradingStartRequestObject) (PostApiTradingStartResponseObject, error) {
	s.logger.UI().Info("Trading start requested via API")

	if s.app == nil {
		return PostApiTradingStart503JSONResponse{svcUnavailableErr("Application not initialized")}, nil
	}

	if err := s.app.SetTradingEnabled(true); err != nil {
		s.logger.Error("Failed to start trading: " + err.Error())
		errStatus := Error
		errMsg := "Failed to start trading: " + err.Error()
		enabled := false
		now := time.Now()
		return PostApiTradingStart500JSONResponse{
			Enabled:   &enabled,
			Status:    &errStatus,
			Message:   &errMsg,
			Timestamp: &now,
		}, nil
	}

	s.logger.UI().Info("Trading started successfully via API")
	successStatus := Success
	msg := "Trading started successfully"
	enabled := true
	now := time.Now()
	return PostApiTradingStart200JSONResponse{
		Enabled:   &enabled,
		Status:    &successStatus,
		Message:   &msg,
		Timestamp: &now,
	}, nil
}

// PostApiTradingStop implements StrictServerInterface - stop trading
func (s *Server) PostApiTradingStop(ctx context.Context, request PostApiTradingStopRequestObject) (PostApiTradingStopResponseObject, error) {
	s.logger.UI().Info("Trading stop requested via API")

	if s.app == nil {
		return PostApiTradingStop503JSONResponse{svcUnavailableErr("Application not initialized")}, nil
	}

	if err := s.app.SetTradingEnabled(false); err != nil {
		s.logger.Error("Failed to stop trading: " + err.Error())
		errStatus := Error
		errMsg := "Failed to stop trading: " + err.Error()
		enabled := true
		now := time.Now()
		return PostApiTradingStop500JSONResponse{
			Enabled:   &enabled,
			Status:    &errStatus,
			Message:   &errMsg,
			Timestamp: &now,
		}, nil
	}

	s.logger.UI().Info("Trading stopped successfully via API")
	successStatus := Success
	msg := "Trading stopped successfully"
	enabled := false
	now := time.Now()
	return PostApiTradingStop200JSONResponse{
		Enabled:   &enabled,
		Status:    &successStatus,
		Message:   &msg,
		Timestamp: &now,
	}, nil
}

// GetApiTradingStatus implements StrictServerInterface - get trading status
func (s *Server) GetApiTradingStatus(ctx context.Context, request GetApiTradingStatusRequestObject) (GetApiTradingStatusResponseObject, error) {
	if s.app == nil {
		return GetApiTradingStatus503JSONResponse{svcUnavailableErr("Application not initialized")}, nil
	}

	tradingEnabled := s.app.IsTradingEnabled()
	now := time.Now()

	var statusVal TradingStatusResponseStatus
	var msg string
	if tradingEnabled {
		statusVal = Running
		msg = "Trading is currently active"
	} else {
		statusVal = Stopped
		msg = "Trading is currently stopped"
	}

	return GetApiTradingStatus200JSONResponse{
		Enabled:   &tradingEnabled,
		Status:    &statusVal,
		Message:   &msg,
		Timestamp: &now,
	}, nil
}

// PostApiStrategyReset implements StrictServerInterface - reset strategy state
func (s *Server) PostApiStrategyReset(ctx context.Context, request PostApiStrategyResetRequestObject) (PostApiStrategyResetResponseObject, error) {
	s.logger.UI().Info("Strategy reset requested via API")

	if s.app == nil || s.app.GetCurrentStrategy() == nil {
		return PostApiStrategyReset503JSONResponse{svcUnavailableErr("No strategy available to reset")}, nil
	}

	strat := s.app.GetCurrentStrategy()
	if err := strat.Reset(); err != nil {
		s.logger.Error("Failed to reset strategy: " + err.Error())
		return PostApiStrategyReset500JSONResponse{internalErr("Failed to reset strategy: " + err.Error())}, nil
	}

	s.logger.UI().Info("Strategy reset successfully via API")
	status := "success"
	msg := "Strategy reset successfully"
	return PostApiStrategyReset200JSONResponse{
		Status:  &status,
		Message: &msg,
	}, nil
}
