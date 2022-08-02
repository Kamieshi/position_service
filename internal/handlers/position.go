package handlers

import (
	"context"
	"time"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/service"
	"github.com/Kamieshi/position_service/protoc"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// PositionManagerServerImplement Implement PositionManagerServer
type PositionManagerServerImplement struct {
	PositionsManager *service.PositionsService
	protoc.PositionsManagerServer
}

// OpenPosition Open new position
func (p *PositionManagerServerImplement) OpenPosition(ctx context.Context, req *protoc.OpenPositionRequest) (*protoc.OpenPositionResponse, error) {
	logrus.Debug("Handler Open ActivePosition", req)
	clientID, err := uuid.Parse(req.UserID)
	if err != nil {
		logrus.WithError(err).Error("Add handler PositionManagerServer Parse ID")
		return &protoc.OpenPositionResponse{Error: err.Error()}, nil
	}
	timeIn, err := time.Parse("2006-01-02T15:04:05.000TZ-07:00", req.Price.Time)
	if err != nil {
		logrus.WithError(err).Error("Add handler PositionManagerServer Parse Time")
		return &protoc.OpenPositionResponse{Error: err.Error()}, nil
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
		logrus.WithError(err).Error("Add handler PositionManagerServer Add")
		return &protoc.OpenPositionResponse{
			ID:    "",
			Error: err.Error(),
		}, nil
	}
	return &protoc.OpenPositionResponse{
		ID: position.ID.String(),
	}, nil
}

// ClosePosition FixedClosedPosition position
func (p *PositionManagerServerImplement) ClosePosition(ctx context.Context, req *protoc.ClosePositionRequest) (*protoc.ClosePositionResponse, error) {
	logrus.Debug("Handler FixedClosedPosition ActivePosition", req)
	positionID, err := uuid.Parse(req.PositionID)
	if err != nil {
		logrus.WithError(err).Error("position handler / ClosePosition /  PositionManagerServer Parse ID")
		return &protoc.ClosePositionResponse{Error: err.Error()}, nil
	}
	clientID, err := uuid.Parse(req.UserID)
	if err != nil {
		logrus.WithError(err).Error("position handler / ClosePosition /  PositionManagerServer Parse Time")
		return &protoc.ClosePositionResponse{Error: err.Error()}, nil
	}

	position, err := p.PositionsManager.ClosePosition(ctx, clientID, positionID)
	if err != nil {
		logrus.WithError(err).Error("position handler / ClosePosition / PositionManagerServer FixedClosedPosition ActivePosition")
		return &protoc.ClosePositionResponse{Error: err.Error()}, nil
	}

	return &protoc.ClosePositionResponse{
		Profit: position.Profit,
	}, nil
}

func (p *PositionManagerServerImplement) GetPositionByID(ctx context.Context, req *protoc.GetPositionByIDRequest) (*protoc.GetPositionByIDResponse, error) {
	idParse, err := uuid.Parse(req.PositionID)
	if err != nil {
		logrus.WithError(err).Error("handler position / GetPositionByID / Parse uuid ")
		return &protoc.GetPositionByIDResponse{
			Error: err.Error(),
		}, nil
	}
	position, err := p.PositionsManager.GetByID(ctx, idParse)
	if err != nil {
		logrus.WithError(err).Error("handler position / GetPositionByID / get position from service")
		return &protoc.GetPositionByIDResponse{
			Error: err.Error(),
		}, nil
	}
	return &protoc.GetPositionByIDResponse{
		Position: &protoc.Position{
			PositionID:      position.ID.String(),
			CompanyID:       position.CompanyID,
			OpenedBid:       position.OpenPrice.Bid,
			OpenedAsk:       position.OpenPrice.Ask,
			TimeOpenedPrice: position.OpenPrice.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
			IsOpened:        position.IsOpened,
			IsSale:          position.IsSales,
			IsFixed:         position.IsFixes,
			MaxProfit:       position.MaxCurrentCost,
			MinProfit:       position.MinCurrentCost,
			UserID:          position.User.ID.String(),
			CloseProfit:     position.Profit,
		},
	}, nil
}

func (p *PositionManagerServerImplement) GetAllUserPositions(ctx context.Context, req *protoc.GetAllUserPositionsRequest) (*protoc.GetAllUserPositionsResponse, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		logrus.WithError(err).Error("handler position / GetAllUserPositions /  Parse ID")
		return &protoc.GetAllUserPositionsResponse{Error: err.Error()}, nil
	}

	positions, err := p.PositionsManager.GetAllUserPositions(ctx, userID)
	if err != nil {
		logrus.WithError(err).Error("handler position / GetAllUserPositions / Get position form service")
		return &protoc.GetAllUserPositionsResponse{
			Error: err.Error(),
		}, nil
	}
	protoPositions := make([]*protoc.Position, 0, len(positions))
	for _, position := range positions {
		protoPositions = append(protoPositions, &protoc.Position{
			PositionID:      position.ID.String(),
			CompanyID:       position.CompanyID,
			OpenedBid:       position.OpenPrice.Bid,
			OpenedAsk:       position.OpenPrice.Ask,
			TimeOpenedPrice: position.OpenPrice.Time.Format("2006-01-02T15:04:05.000TZ-07:00"),
			IsOpened:        position.IsOpened,
			IsSale:          position.IsSales,
			IsFixed:         position.IsFixes,
			MaxProfit:       position.MaxCurrentCost,
			MinProfit:       position.MinCurrentCost,
			UserID:          position.User.ID.String(),
			CloseProfit:     position.Profit,
		})
	}
	return &protoc.GetAllUserPositionsResponse{
		Positions: protoPositions,
	}, nil
}
