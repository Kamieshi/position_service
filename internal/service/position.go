package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/sirupsen/logrus"
)

type Position struct {
	position    *model.Position
	chFromClose chan bool
	rwm         sync.RWMutex
}

func (p *Position) StartTakeActualState(ctx context.Context, chPrice chan *model.Price) {
	logrus.Debug("TakeActualState for position : ", p.position.ID)
	p.rwm.Lock()
	p.position.IsOpened = true
	p.rwm.Unlock()
	for {
		select {
		case <-ctx.Done():
			close(chPrice)
			return
		case <-p.chFromClose:
			close(chPrice)
			return
		case price := <-chPrice:
			if !p.position.IsOpened {
				close(chPrice)
				return
			}
			dopProfit := int64(0)
			if p.position.IsSales {
				dopProfit += int64(p.position.CountBuyPosition) * int64(p.position.OpenPrice.Bid)
			}
			p.rwm.Lock()
			p.position.Profit = int64(p.position.CountBuyPosition)*(int64(price.Bid)-int64(p.position.OpenPrice.Ask)) + dopProfit
			p.rwm.Unlock()
			if p.position.IsFixes {
				if p.position.Profit+dopProfit >= p.position.MaxCurrentCost || p.position.Profit+dopProfit <= p.position.MinCurrentCost {
					p.rwm.Lock()
					p.position.WasAutoCLose = true
					p.position.IsOpened = false
					p.rwm.Unlock()
					close(chPrice)
					return
				}
			}

		}
	}
}

func (p *Position) StopTakeActualState() error {
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
