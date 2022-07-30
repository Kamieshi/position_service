package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/service"
	"github.com/Kamieshi/position_service/protoc"
)

// PositionManagerServerImplement Implement PositionManagerServer
type PositionManagerServerImplement struct {
	PositionsManager *service.PositionsService
	protoc.PositionsManagerServer
}

// OpenPosition Open new position
func (p *PositionManagerServerImplement) OpenPosition(ctx context.Context, req *protoc.OpenPositionRequest) (*protoc.OpenPositionResponse, error) {
	log.Debug("Handler Open Position", req)
	clientID, err := uuid.Parse(req.UserID)
	if err != nil {
		log.WithError(err).Error("Add handler PositionManagerServer Parse ID")
		return &protoc.OpenPositionResponse{}, err
	}
	timeIn, err := time.Parse("2006-01-02T15:04:05.000TZ-07:00", req.Price.Time)
	if err != nil {
		log.WithError(err).Error("Add handler PositionManagerServer Parse Time")
		return &protoc.OpenPositionResponse{}, err
	}
	price := model.Price{
		Ask:  req.Price.Ask,
		Bid:  req.Price.Bid,
		Time: timeIn,
	}

	position := &model.Position{
		User: &model.User{
			ID: clientID,
		},
		CompanyID:        req.Price.Company.ID,
		OpenPrice:        price,
		CountBuyPosition: req.CountBuyPosition,
		MaxCurrentCost:   req.MaxProfit,
		MinCurrentCost:   req.MinProfit,
		IsSales:          req.IsSales,
		IsFixes:          req.IsFixed,
	}

	err = p.PositionsManager.OpenPosition(ctx, position)
	if err != nil {
		log.WithError(err).Error("Add handler PositionManagerServer Add")
		return &protoc.OpenPositionResponse{
			ID: "",
		}, err
	}
	return &protoc.OpenPositionResponse{
		ID: position.ID.String(),
	}, err
}

// ClosePosition Close position
func (p *PositionManagerServerImplement) ClosePosition(ctx context.Context, req *protoc.ClosePositionRequest) (*protoc.ClosePositionResponse, error) {
	log.Debug("Handler Close Position", req)
	positionID, err := uuid.Parse(req.PositionID)
	if err != nil {
		log.WithError(err).Error("_closePosition handler PositionManagerServer Parse ID")
		return &protoc.ClosePositionResponse{}, err
	}
	clientID, err := uuid.Parse(req.UserID)
	if err != nil {
		log.WithError(err).Error("_closePosition handler PositionManagerServer Parse Time")
		return &protoc.ClosePositionResponse{}, err
	}

	position, err := p.PositionsManager.ClosePosition(ctx, clientID, positionID)
	if err != nil {
		log.WithError(err).Error("_closePosition handler PositionManagerServer Close Position")
		return &protoc.ClosePositionResponse{}, err
	}

	return &protoc.ClosePositionResponse{
		Profit: position.Profit,
	}, nil
}
