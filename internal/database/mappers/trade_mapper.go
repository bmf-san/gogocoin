package mappers

import (
	"encoding/json"

	"github.com/bmf-san/gogocoin/v1/internal/database/models"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// TradeMapper handles conversion between domain.Trade and models.TradeModel
type TradeMapper struct{}

// NewTradeMapper creates a new TradeMapper
func NewTradeMapper() *TradeMapper {
	return &TradeMapper{}
}

// ToDomain converts a TradeModel to domain.Trade with metadata deserialization
func (m *TradeMapper) ToDomain(model *models.TradeModel) (domain.Trade, error) {
	trade := model.ToDomain()

	// Deserialize metadata if present
	if model.MetadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(model.MetadataJSON), &metadata); err != nil {
			return trade, err
		}
		trade.Metadata = metadata
	}

	return trade, nil
}

// ToModel converts a domain.Trade to TradeModel with metadata serialization
func (m *TradeMapper) ToModel(trade domain.Trade) (*models.TradeModel, error) {
	model := models.FromDomainTrade(trade)

	// Serialize metadata if present
	if trade.Metadata != nil {
		metadataJSON, err := json.Marshal(trade.Metadata)
		if err != nil {
			return nil, err
		}
		model.MetadataJSON = string(metadataJSON)
	}

	return model, nil
}

// ToDomainList converts a slice of TradeModels to domain.Trades
func (m *TradeMapper) ToDomainList(models []*models.TradeModel) ([]domain.Trade, error) {
	trades := make([]domain.Trade, 0, len(models))
	for _, model := range models {
		trade, err := m.ToDomain(model)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, nil
}
