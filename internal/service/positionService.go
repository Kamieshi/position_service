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

const actualState = true
const enoughForOpenPosition = true

// OpenPosition open position
func (p *PositionsService) OpenPosition(ctx context.Context, position *model.Position) error {
	logrus.Debug("Position service / OpenPosition ")
	if state, err := p.checkActualOpenedPriceState(position); err != nil {
		return fmt.Errorf("service position / OpenPosition / Error get actualState price : %v ", err)
	} else {
		if state != actualState {
			return fmt.Errorf("service position / OpenPosition / Opened price isn't actual ")
		}
	}

	if enough, err := p.checkEnoughBalanceUser(ctx, position); err != nil {
		return fmt.Errorf("service position / OpenPosition / try check user balance : %v ", err)
	} else {
		if enough != enoughForOpenPosition {
			return fmt.Errorf("service position / OpenPosition / Isn't enough monay for open position")
		}
	}

	if !p.userPositionsIsExist(position.User.ID) {
		p.addNewUserPositions(position.User.ID)
	}

	activePosition := NewActiveOpenedPosition(position)

	if err := p.addToUserPositions(activePosition); err != nil {
		return fmt.Errorf("service position / OpenPosition / add active position to user activePositions : %v ", err)
	}
	if err := p.writeToDB(activePosition); err != nil {
		return fmt.Errorf("service position / OpenPosition / write position to DB: %v ", err)
	}
	p.startTakeActualStateAndAddSubscriber(activePosition)
	return nil
}

// ClosePosition close position
func (p *PositionsService) ClosePosition(ctx context.Context, userID, positionID uuid.UUID) (*model.Position, error) {
	logrus.Debug("Position service / ClosePosition ")
	if !p.userPositionsIsExist(userID) {
		return nil, fmt.Errorf("ActivePosition service / CloasePosition / user %s not exist ")
	}
	position, err := p.closePositionForUser(userID, positionID)
	if err != nil {
		return nil, fmt.Errorf("position service / FixedClosedPosition / close position : %v", err)
	}
	return position, nil
}

// WriteClosedPositions G for write closed position in db from buffer closed position
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

func (p *PositionsService) userPositionsIsExist(userID uuid.UUID) bool {
	if _, exist := p.UsersPositions[userID]; !exist {
		return false
	}
	return true
}

func (p *PositionsService) closePositionForUser(userID uuid.UUID, positionID uuid.UUID) (*model.Position, error) {
	position, err := p.UsersPositions[userID].CloseByID(positionID)
	if err != nil {
		return nil, fmt.Errorf("position service / closePositionForUser / close position : %v", err)
	}
	return position, nil
}

func (p *PositionsService) checkActualOpenedPriceState(position *model.Position) (bool, error) {
	currentPrice, err := p.PriceStorage.GetPrice(position.CompanyID)
	if err != nil {
		return false, fmt.Errorf("service position / Add / Try get current price from PriceStorage : %v ", err)
	}
	if currentPrice.Bid != position.OpenPrice.Bid {
		return false, nil
	}
	return actualState, nil
}

func (p *PositionsService) checkEnoughBalanceUser(ctx context.Context, position *model.Position) (bool, error) {
	user, err := p.UserStorage.Get(ctx, position.User.ID)
	if err != nil {
		return false, fmt.Errorf("service position / checkEnoughBalanceUser / Try get user : %v ", err)
	}
	if int64(position.OpenPrice.Ask*position.CountBuyPosition) > user.Balance {
		return false, nil
	}
	return true, nil

}

func (p *PositionsService) addNewUserPositions(userID uuid.UUID) {
	p.rwm.Lock()
	p.UsersPositions[userID] = NewUserPositions(p.UserStorage)
	p.rwm.Unlock()
	go p.UsersPositions[userID].FixedClosedActivePositions()
	go p.UsersPositions[userID].CheckSummaryProfitAndAutoCloseMostNegativePositionWhenCommonProfitBecameNegative()
	go p.WriteClosedPositions(p.CtxApp, userID)
}

func (p *PositionsService) addToUserPositions(activePosition *ActivePosition) error {
	if err := p.UsersPositions[activePosition.position.User.ID].Add(activePosition); err != nil {
		return fmt.Errorf("position service / addToUserPositions / Add Active position to userActivePositions : %v", err)
	}
	return nil
}

func (p *PositionsService) startTakeActualStateAndAddSubscriber(activePosition *ActivePosition) {
	chPrice := make(chan *model.Price)
	p.PriceStorage.AddSubscriber(chPrice, activePosition.position.CompanyID)
	p.rwm.RLock()
	p.UsersPositions[activePosition.position.User.ID].StartTakeActualState(p.CtxApp, activePosition.position.ID, chPrice)
	p.rwm.RUnlock()
}

func (p *PositionsService) writeToDB(activePosition *ActivePosition) error {
	tx, err := p.PositionRepository.Pool.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("PositionManager service_old/ writeToDB / get tx from pool : %v", err)
	}

	err = p.PositionRepository.InsertTx(context.Background(), tx, activePosition.position)
	if err != nil {
		return fmt.Errorf("service position / writeToDB / Insert to position into DB : %v ", err)
	}
	err = tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("service position / writeToDB / Commit transaction : %v ", err)
	}
	return nil
}
