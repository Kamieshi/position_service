package service

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/userStorage"
)

type UserPositions struct {
	Positions                     *list.List
	PositionsMap                  map[uuid.UUID]*Position
	userStorage                   *userStorage.UserService
	rwm                           sync.RWMutex
	BufferWriteClosePositionsInDB chan *model.Position
}

func NewUserPositions(userSt *userStorage.UserService) *UserPositions {
	logrus.Debug("NewUserPositions")
	return &UserPositions{
		Positions:                     list.New(),
		PositionsMap:                  make(map[uuid.UUID]*Position),
		BufferWriteClosePositionsInDB: make(chan *model.Position, 100), // Add size buffer in config
		userStorage:                   userSt,
	}
}

func (p *UserPositions) Add(ctx context.Context, position *model.Position, chPrice chan *model.Price) error {
	logrus.Debug("Add")
	p.rwm.RLock()
	if _, exist := p.PositionsMap[position.ID]; exist {
		p.rwm.RUnlock()
		return fmt.Errorf("user positions / OpenPostition / Current position is exist : %v ", position.ID)
	}
	p.rwm.RUnlock()
	p.rwm.Lock()
	chCLose := make(chan bool)
	positionSync := &Position{
		position:    position,
		chFromClose: chCLose,
	}
	p.PositionsMap[position.ID] = positionSync
	p.Positions.PushBack(position)
	go p.PositionsMap[position.ID].StartTakeActualState(ctx, chPrice)
	p.rwm.Unlock()
	return nil
}

func (p *UserPositions) Close(position *model.Position) error {
	logrus.Debug("Close")
	err := p.userStorage.AddProfit(position.Profit, position.User.ID)
	if err != nil {
		return fmt.Errorf("user position / Close / add profit: %v", err)
	}
	close(p.PositionsMap[position.ID].chFromClose)
	p.delete(position)
	p.writeInRepository(position)
	logrus.Debugf("Position %s was closed, profit %d")
	return nil
}

func (p *UserPositions) delete(position *model.Position) {
	delete(p.PositionsMap, position.ID)
}

func (p *UserPositions) writeInRepository(position *model.Position) {
	p.BufferWriteClosePositionsInDB <- position
}

func (p *UserPositions) CloseByID(positionID uuid.UUID) (*model.Position, error) {
	logrus.Debug("CloseByID")
	if _, e := p.PositionsMap[positionID]; !e {
		return nil, fmt.Errorf("user positions/ CloseByID / Position with ID %s not exist ", positionID)
	}
	position := p.PositionsMap[positionID]
	err := position.StopTakeActualState()
	if err != nil {
		return nil, err
	}
	return position.position, nil

}

func (p *UserPositions) FixedClosed() {
	logrus.Debug("Start FixedClosed for user")
	for {
		p.rwm.Lock()
		for positionElement := p.Positions.Front(); positionElement != nil; positionElement = positionElement.Next() {
			p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RLock()
			if !positionElement.Value.(*model.Position).IsOpened {
				p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RUnlock()
				err := p.Close(positionElement.Value.(*model.Position))
				if err != nil {
					logrus.WithError(err).Error("user positions / MonitorCommonProfit / try close position")
				}
				if positionElement.Next() != nil {
					positionElement = positionElement.Next()
					p.Positions.Remove(positionElement.Prev())
					continue
				}
				p.Positions.Remove(positionElement)
			} else {
				p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RUnlock()
			}

		}
		p.rwm.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
}

func (p *UserPositions) CheckSummaryProfit() {
	commonProfit := int64(0)
	for {
		p.rwm.Lock()
		for positionElement := p.Positions.Front(); positionElement != nil; positionElement = positionElement.Next() {
			if !positionElement.Value.(*model.Position).IsOpened {
				continue
			}
			p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RLock()
			commonProfit += positionElement.Value.(*model.Position).Profit
			p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RUnlock()
		}
		p.rwm.Unlock()
		if commonProfit < 0 {
			user, err := p.userStorage.Get(context.Background(), p.Positions.Front().Value.(*model.Position).User.ID)
			if err != nil {
				logrus.WithError(err).Error("user position / Close / get user")
			}
			p.userStorage.RLock()
			if user.Balance+commonProfit <= 0 {
				logrus.Warn(user, "TODO ", commonProfit)
			}
			p.userStorage.RUnlock()
		}
		commonProfit = 0
		time.Sleep(10 * time.Millisecond)
	}
}
