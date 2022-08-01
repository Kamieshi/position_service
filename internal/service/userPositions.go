// Package service
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

const (
	timeIntervalFixedClosePosition      = 10 * time.Millisecond
	timeIntervalCheckCommonProfit       = 10 * time.Millisecond
	sizeQueueClosedPositionOnWriteToRep = 100
)

type UserPositions struct {
	Positions                     *list.List
	PositionsMap                  map[uuid.UUID]*ActivePosition
	userStorage                   *userStorage.UserService
	rwm                           sync.RWMutex
	BufferWriteClosePositionsInDB chan *model.Position
}

// NewUserPositions Constructor
func NewUserPositions(userSt *userStorage.UserService) *UserPositions {
	logrus.Debug("NewUserPositions")
	return &UserPositions{
		Positions:                     list.New(),
		PositionsMap:                  make(map[uuid.UUID]*ActivePosition),
		BufferWriteClosePositionsInDB: make(chan *model.Position, sizeQueueClosedPositionOnWriteToRep), // Add size buffer in config
		userStorage:                   userSt,
	}
}

// Add active position into PositionsMap and Positions List
func (p *UserPositions) Add(activePosition *ActivePosition) error {
	logrus.Debug("Add")
	p.rwm.RLock()
	if _, exist := p.PositionsMap[activePosition.position.ID]; exist {
		p.rwm.RUnlock()
		return fmt.Errorf("user positions / Add / Current position is exist : %v ", activePosition.position.ID)
	}
	p.rwm.RUnlock()
	p.rwm.Lock()
	p.PositionsMap[activePosition.position.ID] = activePosition
	p.Positions.PushBack(activePosition.position)
	p.rwm.Unlock()
	return nil
}

// FixedClosedPosition active position if this position was opened, send message into close chanel and TakeActualState goroutine will be stopped
func (p *UserPositions) FixedClosedPosition(position *model.Position) error {
	logrus.Debug("FixedClosedPosition")
	err := p.userStorage.AddProfit(position.Profit, position.User.ID)
	if err != nil {
		return fmt.Errorf("user position / FixedClosedPosition / add profit: %v", err)
	}
	p.closeAndDelete(position)
	p.rwm.RLock()
	if p.PositionsMap[position.ID].closedTriggeredSync {
		p.rwm.RUnlock()
		return nil
	}
	p.rwm.RUnlock()

	p.writeInBufferClosedPositions(position)
	return nil
}

func (p *UserPositions) closeAndDelete(position *model.Position) {
	close(p.PositionsMap[position.ID].chFromClose)
	delete(p.PositionsMap, position.ID)
}

func (p *UserPositions) writeInBufferClosedPositions(position *model.Position) {
	p.BufferWriteClosePositionsInDB <- position
}

// CloseByID close position. This method required for close position from handler
func (p *UserPositions) CloseByID(positionID uuid.UUID) (*model.Position, error) {
	logrus.Debug("CloseByID")
	if _, e := p.PositionsMap[positionID]; !e {
		return nil, fmt.Errorf("user positions/ CloseByID / ActivePosition with ID %s not exist ", positionID)
	}
	p.rwm.RLock()
	position := p.PositionsMap[positionID]
	p.rwm.RUnlock()
	err := position.StopTakeActualState()
	if err != nil {
		return nil, err
	}
	return position.position, nil
}

// StartTakeActualState run goroutine from take active position in actual state
func (p *UserPositions) StartTakeActualState(ctx context.Context, positionID uuid.UUID, chPrice chan *model.Price) {
	p.rwm.RLock()
	go p.PositionsMap[positionID].StartTakeActualStateAndAutoClose(ctx, chPrice)
	p.rwm.RUnlock()
}

// FixedClosedActivePositions G. In some period check all Active positions and when find position with field IsActive == false
// fixed this active position ( clean map, list, update user balance and write to db)
func (p *UserPositions) FixedClosedActivePositions() {
	logrus.Debug("Start FixedClosedActivePositions for user")
	for {
		p.rwm.Lock()
		for positionElement := p.Positions.Front(); positionElement != nil; positionElement = positionElement.Next() {
			p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RLock()
			if !positionElement.Value.(*model.Position).IsOpened {
				p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RUnlock()
				err := p.FixedClosedPosition(positionElement.Value.(*model.Position))
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
		time.Sleep(timeIntervalFixedClosePosition)
	}
}

// CheckSummaryProfitAndAutoCloseMostNegativePositionWhenCommonProfitBecameNegative  TODO Rename and destroy on more smale func
func (p *UserPositions) CheckSummaryProfitAndAutoCloseMostNegativePositionWhenCommonProfitBecameNegative() {
	var minProfitPosition *model.Position
	commonProfit := int64(0)
	for {
		p.rwm.Lock()

		for positionElement := p.Positions.Front(); positionElement != nil; positionElement = positionElement.Next() {
			if !positionElement.Value.(*model.Position).IsOpened {
				continue
			}
			if minProfitPosition == nil {
				minProfitPosition = positionElement.Value.(*model.Position)
			} else if minProfitPosition.Profit > positionElement.Value.(*model.Position).Profit {
				minProfitPosition = positionElement.Value.(*model.Position)
			}
			p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RLock()
			commonProfit += positionElement.Value.(*model.Position).Profit
			p.PositionsMap[positionElement.Value.(*model.Position).ID].rwm.RUnlock()
		}
		p.rwm.Unlock()
		if commonProfit < 0 {
			user, err := p.userStorage.Get(context.Background(), p.Positions.Front().Value.(*model.Position).User.ID)
			if err != nil {
				logrus.WithError(err).Error("user position / FixedClosedPosition / get user")
			}
			p.userStorage.RLock()
			if user.Balance+commonProfit <= 0 {
				pos, err := p.CloseByID(minProfitPosition.ID)
				if err != nil {
					logrus.Warn("user position / CheckSummaryProfit / Error close position : ", err)
				}
				logrus.Debug(pos, " was auto close because common profit is less than client balance")
			}
			p.userStorage.RUnlock()
		}
		minProfitPosition = nil
		commonProfit = 0
		time.Sleep(timeIntervalCheckCommonProfit)
	}
}

// CloseTriggeredSync Close position triggerred chanel sync from postgres
func (p *UserPositions) CloseTriggeredSync(position *model.Position) error {
	p.rwm.RLock()
	if _, e := p.PositionsMap[position.ID]; !e {
		return fmt.Errorf("user positions/ CloseTriggeredSync / ActivePosition with ID %s not exist ", position.ID)
	}
	activePosition := p.PositionsMap[position.ID]
	p.rwm.RUnlock()
	activePosition.closedTriggeredSync = true
	err := p.PositionsMap[position.ID].StopTakeActualState()
	if err != nil {
		return fmt.Errorf("user positions/ CloseTriggeredSync / StopTakeActualState() with ID %s ", position.ID)
	}
	return nil
}
