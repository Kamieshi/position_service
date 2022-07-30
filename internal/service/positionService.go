package service

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/priceStorage"
	"github.com/Kamieshi/position_service/internal/repository"
	"github.com/Kamieshi/position_service/internal/userStorage"
)

type PositionsService struct {
	UsersPositions     map[uuid.UUID]*UserPositions
	PriceStorage       *priceStorage.PriceStore
	UserStorage        *userStorage.UserService
	PositionRepository *repository.PositionRepository
	CtxApp             context.Context
	rwm                sync.RWMutex
}

func (p *PositionsService) OpenPosition(ctx context.Context, position *model.Position) error {
	currentPrice, err := p.PriceStorage.GetPrice(position.CompanyID)
	if err != nil {
		return fmt.Errorf("service position / Add / Try get current price from PriceStorage : %v ", err)
	}

	if currentPrice.Bid != position.OpenPrice.Bid {
		return fmt.Errorf("service position / Add / Not actual BID : %v ", err)
	}

	user, err := p.UserStorage.Get(ctx, position.User.ID)
	if err != nil {
		return fmt.Errorf("service position / Add / Try get user : %v ", err)
	}

	if int64(position.OpenPrice.Ask*position.CountBuyPosition) > user.Balance {
		return fmt.Errorf("service position / Add / Don't have money : %d  >  %d",
			int64(position.OpenPrice.Ask*position.CountBuyPosition),
			user.Balance,
		)
	}

	_, exist := p.UsersPositions[user.ID]
	if !exist {
		p.rwm.Lock()
		p.UsersPositions[user.ID] = NewUserPositions(p.UserStorage)
		p.rwm.Unlock()
		go p.UsersPositions[user.ID].FixedClosed()
		go p.UsersPositions[user.ID].CheckSummaryProfit()
		go p.WriteClosedPositions(p.CtxApp, user.ID)
	}

	position.ID = uuid.New()
	tx, err := p.PositionRepository.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("PositionManager service_old/ Add / get tx from pool : %v", err)
	}

	position.IsOpened = true
	err = p.PositionRepository.InsertTx(ctx, tx, position)
	if err != nil {
		return fmt.Errorf("service position / Add / Insert to position into DB : %v ", err)
	}
	chData := make(chan *model.Price)
	err = p.UsersPositions[user.ID].Add(p.PriceStorage.CtxApp, position, chData)
	if err != nil {
		return fmt.Errorf("service position / Add / Try open position : %v ", err)
	}
	p.PriceStorage.AddSubscriber(chData, position.CompanyID)
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("PositionManager service_old / Add / commit transaction : %v", err)
	}
	return nil
}

func (p *PositionsService) ClosePosition(ctx context.Context, userID, positionID uuid.UUID) (*model.Position, error) {
	if _, exist := p.UsersPositions[userID]; !exist {
		return nil, fmt.Errorf("Position service / CloasePosition / user %s not exist ")
	}
	position, err := p.UsersPositions[userID].CloseByID(positionID)
	if err != nil {
		return nil, fmt.Errorf("position service / Close / close position : %v", err)
	}
	return position, nil
}

func (p *PositionsService) WriteClosedPositions(ctx context.Context, userID uuid.UUID) {
	logrus.Debugf("Start WriteClosedPositions for user %v", userID)
	p.rwm.RLock()
	bufChan := p.UsersPositions[userID].BufferWriteClosePositionsInDB
	p.rwm.RUnlock()
	for {
		select {
		case <-ctx.Done():
			return
		case position := <-bufChan:
			tx, err := p.PositionRepository.Pool.Begin(ctx)
			if err != nil {
				logrus.WithError(err).Error("position service / WriteClosedPositions / open transaction")
				continue
			}
			err = p.UserStorage.AddProfitInRepositoryTX(ctx, tx, position.User.ID, position.Profit)
			if err != nil {
				logrus.WithError(err).Error("position service / WriteClosedPositions / Add profit to balance")
				continue
			}

			err = p.PositionRepository.ClosePositionTx(ctx, tx, position)
			if err != nil {
				logrus.WithError(err).Error("position service / WriteClosedPositions / UpdatePosition")
				continue
			}
			err = tx.Commit(ctx)
			if err != nil {
				logrus.WithError(err).Error("position service / WriteClosedPositions / Commit transaction")
			}
		}
	}
}
