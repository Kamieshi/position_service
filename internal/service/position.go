package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/priceStorage"
	"github.com/Kamieshi/position_service/internal/repository"
	"github.com/Kamieshi/position_service/internal/userStorage"
)

type PositionService struct {
	ClientsPositions   map[uuid.UUID]*ClientPositions
	PriceStorage       *priceStorage.PriceStore
	UserStorage        *userStorage.UserService
	PositionRepository *repository.PositionRepository
}

func (p *PositionService) OpenPosition(ctx context.Context, position *model.Position) error {
	currentPrice, err := p.PriceStorage.GetPrice(position.CompanyID)
	if err != nil {
		return fmt.Errorf("service position / OpenPosition / Try get current price from PriceStorage : %v ", err)
	}

	if currentPrice.Bid != position.OpenPrice.Bid {
		return fmt.Errorf("service position / OpenPosition / Not actual BID : %v ", err)
	}

	user, err := p.UserStorage.Get(ctx, position.Client.ID)
	if err != nil {
		return fmt.Errorf("service position / OpenPosition / Try get user : %v ", err)
	}

	if int64(position.OpenPrice.Ask*position.CountBuyPosition) > user.Balance {
		return fmt.Errorf("service position / OpenPosition / Don't have money : %d  >  %d",
			int64(position.OpenPrice.Ask*position.CountBuyPosition),
			user.Balance,
		)
	}

	_, exist := p.ClientsPositions[user.ID]
	if !exist {
		p.ClientsPositions[user.ID] = NewClientPositions(p.UserStorage)
		go p.ClientsPositions[user.ID].MonitorPositions()
	}

	position.ID = uuid.New()
	tx, err := p.PositionRepository.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("PositionManager service_old/ OpenPosition / get tx from pool : %v", err)
	}

	err = p.PositionRepository.InsertTx(ctx, tx, position)
	if err != nil {
		return fmt.Errorf("service position / OpenPosition / Insert to position into DB : %v ", err)
	}

	chData := make(chan *model.Price)
	position.IsOpened = true
	err = p.ClientsPositions[user.ID].OpenPosition(p.PriceStorage.CtxApp, position, chData)
	if err != nil {
		return fmt.Errorf("service position / OpenPosition / Try open position : %v ", err)
	}
	p.PriceStorage.AddSubscriber(chData, position.CompanyID)
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("PositionManager service_old / OpenPosition / commit transaction : %v", err)
	}
	return nil
}

func (p *PositionService) ClosePosition(ctx context.Context, userID, positionID uuid.UUID) error {
	err := p.ClientsPositions[userID].CloseAfterHandler(positionID)
	if err != nil {
		return fmt.Errorf("position service / ClosePosition / close position : %v", err)
	}
	return err
}

func (p *PositionService) WriterIntoDB(ctx context.Context, clientID uuid.UUID) {
	for {
		select {
		case <-ctx.Done():
			return
		case position := <-p.ClientsPositions[clientID].BufferWriteClosePositionsInDB:
			tx, err := p.PositionRepository.Pool.Begin(ctx)
			if err != nil {
				logrus.WithError(err).Error("position service / WriterIntoDB / open transaction")
				continue
			}
			err = p.UserStorage.AddProfitInRepositoryTX(ctx, tx, position.Client.ID, position.Profit)
			if err != nil {
				logrus.WithError(err).Error("position service / WriterIntoDB / Add profit to balance")
				continue
			}

			err = p.PositionRepository.ClosePositionTx(ctx, tx, position)
			if err != nil {
				logrus.WithError(err).Error("position service / WriterIntoDB / UpdatePosition")
				continue
			}
			err = tx.Commit(ctx)
			if err != nil {
				logrus.WithError(err).Error("position service / WriterIntoDB / Commit transaction")
			}
		}
	}
}
