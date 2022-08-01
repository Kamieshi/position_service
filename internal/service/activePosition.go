package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/Kamieshi/position_service/internal/model"
)

// ActivePosition layer for work with position in context user Positions
type ActivePosition struct {
	position            *model.Position
	chFromClose         chan bool
	closedTriggeredSync bool
	rwm                 sync.RWMutex
}

// NewActiveOpenedPosition Constructor
func NewActiveOpenedPosition(position *model.Position) *ActivePosition {
	if position.ID == uuid.Nil {
		position.ID = uuid.New()
	}
	chCLose := make(chan bool)
	position.IsOpened = true
	return &ActivePosition{
		position:    position,
		chFromClose: chCLose,
	}
}

// StartTakeActualStateAndAutoClose G for take model.Position in actual state
func (p *ActivePosition) StartTakeActualStateAndAutoClose(ctx context.Context, chPrice chan *model.Price) {
	logrus.Debug("TakeActualState for position : ", p.position.ID)
	for {
		select {
		case <-ctx.Done():
			close(chPrice)
			return
		case <-p.chFromClose:
			p.rwm.Lock()
			p.position.IsOpened = false
			p.rwm.Unlock()
			close(chPrice)
			return
		case price := <-chPrice:
			if !p.position.IsOpened {
				continue
			}
			profit := p.CalculateProfit(price)
			if p.CheckCloseConditions(profit) {
				p.rwm.Lock()
				p.position.WasAutoCLose = true
				p.position.IsOpened = false
				p.rwm.Unlock()
				close(chPrice)
				return
			}

			p.rwm.Lock()
			p.position.Profit = profit
			p.rwm.Unlock()
		}
	}
}

// StopTakeActualState stop G StartTakeActualStateAndAutoClose
func (p *ActivePosition) StopTakeActualState() error {
	p.chFromClose <- true
	select {
	case _, op := <-p.chFromClose:
		if !op {
			p.rwm.Lock()
			p.position.IsOpened = false
			p.rwm.Unlock()
			return nil
		}
	}
	return fmt.Errorf("position / StopTakeActualState / touble with close chanen from error")
}

// CheckCloseConditions check conditions max and min profit
func (p *ActivePosition) CheckCloseConditions(profit int64) bool {
	closeCondition := false
	p.rwm.RLock()
	if p.position.IsFixes {
		if profit >= p.position.MaxCurrentCost || p.position.Profit <= p.position.MinCurrentCost {
			closeCondition = true
		}
	}
	p.rwm.RUnlock()
	return closeCondition
}

// CalculateProfit Calculate profit
func (p *ActivePosition) CalculateProfit(price *model.Price) int64 {
	profit := int64(0)
	p.rwm.RLock()
	if p.position.IsSales {
		profit = int64(p.position.CountBuyPosition) * int64(price.Bid)
	} else {
		profit = int64(p.position.CountBuyPosition) * (int64(price.Bid) - int64(p.position.OpenPrice.Ask))
	}
	p.rwm.RUnlock()
	return profit
}
