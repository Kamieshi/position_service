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

type ClientPositions struct {
	Positions                     *list.List
	channelsFromClose             map[uuid.UUID]chan bool
	userStorage                   *userStorage.UserService
	rwm                           sync.RWMutex
	BufferWriteClosePositionsInDB chan *model.Position
}

func NewClientPositions(userSt *userStorage.UserService) *ClientPositions {
	logrus.Debug("NewClientPositions")
	return &ClientPositions{
		Positions:                     list.New(),
		channelsFromClose:             make(map[uuid.UUID]chan bool),
		BufferWriteClosePositionsInDB: make(chan *model.Position, 100), // Add size buffer in config
		userStorage:                   userSt,
	}
}

func (p *ClientPositions) OpenPosition(ctx context.Context, position *model.Position, chPrice chan *model.Price) error {
	logrus.Debug("OpenPosition")
	p.rwm.RLock()
	if _, exist := p.channelsFromClose[position.ID]; exist {
		p.rwm.RUnlock()
		return fmt.Errorf("user positions / OpenPostition / Current position is exist : %v ", position.ID)
	}
	p.rwm.RUnlock()
	p.rwm.Lock()
	chCLose := make(chan bool)
	p.channelsFromClose[position.ID] = chCLose
	p.Positions.PushBack(position)
	p.rwm.Unlock()
	go p.TakeActualState(ctx, position, chPrice)
	return nil
}

func (p *ClientPositions) ClosePosition(position *model.Position) error {
	logrus.Debug("ClosePosition")
	delete(p.channelsFromClose, position.ID)
	err := p.userStorage.AddProfit(position.Profit, position.Client.ID)
	if err != nil {
		return fmt.Errorf("user position / ClosePosition / add profit: %v", err)
	}
	close(p.channelsFromClose[position.ID])
	p.BufferWriteClosePositionsInDB <- position
	return nil
}

func (p *ClientPositions) CloseAfterHandler(positionID uuid.UUID) error {
	logrus.Debug("CloseAfterHandler")
	p.channelsFromClose[positionID] <- true
	select {
	case _, op := <-p.channelsFromClose[positionID]:
		_ = op
		return nil
	case <-time.After(time.Second * 1):
		return fmt.Errorf("user positions/ CloseAfterHandler / TimeOutError")
	}
}

func (p *ClientPositions) TakeActualState(ctx context.Context, position *model.Position, chPrice chan *model.Price) {
	logrus.Debug("TakeActualState")
	for {
		logrus.Debug("TakeActualState in FOR")
		select {
		case <-ctx.Done():
			close(chPrice)
			return
		case price := <-chPrice:
			if !position.IsOpened {
				close(chPrice)
				break
			}
			dopProfit := int64(0)
			if position.IsSales {
				dopProfit += int64(position.CountBuyPosition) * int64(position.OpenPrice.Bid)
			}
			position.Profit = int64(position.CountBuyPosition)*(int64(price.Bid)-int64(position.OpenPrice.Ask)) + dopProfit
			if position.IsFixes {
				if position.Profit+dopProfit >= position.MaxCurrentCost || position.Profit+dopProfit <= position.MinCurrentCost {
					p.rwm.Lock()
					position.IsOpened = false
					p.rwm.Unlock()
					close(chPrice)
					break
				}
			}
		case <-p.channelsFromClose[position.ID]:
			position.IsOpened = false
			close(chPrice)
			return
		}
	}
}

func (p *ClientPositions) MonitorPositions() {
	logrus.Debug("MonitorPositions")
	for {
		p.rwm.Lock()
		commonProfit := int64(0)
		for positionElement := p.Positions.Front(); positionElement != nil; positionElement = positionElement.Next() {
			if !positionElement.Value.(*model.Position).IsOpened {
				err := p.ClosePosition(positionElement.Value.(*model.Position))
				if err != nil {
					logrus.WithError(err).Error("user positions / MonitorCommonProfit / try close position")
				}
				if positionElement.Next() != nil {
					positionElement = positionElement.Next()
					p.Positions.Remove(positionElement.Prev())
					continue
				}
				p.Positions.Remove(positionElement)
			}
			commonProfit += positionElement.Value.(*model.Position).Profit
		}
		if p.Positions.Len() > 0 {
			user, err := p.userStorage.Get(context.Background(), p.Positions.Front().Value.(*model.Position).Client.ID)
			if err != nil {
				logrus.WithError(err).Error("user position / ClosePosition / get user")
			}
			if user.Balance-commonProfit <= 0 {
				logrus.Warn(user, "TODO ", commonProfit)
			}
		}
		time.Sleep(10 * time.Millisecond)
		p.rwm.Unlock()
	}
}
