// Package priceStorage
package priceStorage

import (
	"container/list"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/Kamieshi/position_service/internal/model"
)

// StreamPriceCompany stream update price to subscribers
type StreamPriceCompany struct {
	DataChan    chan *model.Price
	Subscribers *list.List
	rwm         sync.RWMutex
}

// NewStreamPriceCompany Constructor
func NewStreamPriceCompany() *StreamPriceCompany {
	log.Debug("func NewStreamPriceCompany() *StreamPriceCompany ")
	return &StreamPriceCompany{
		DataChan:    make(chan *model.Price),
		Subscribers: list.New(),
	}
}

// AddSubscriber add new subscriber to subscribers
func (s *StreamPriceCompany) AddSubscriber(chTo chan *model.Price) {
	s.rwm.Lock()
	s.Subscribers.PushBack(chTo)
	s.rwm.Unlock()
}

// StartStreaming goroutine from listen chanel update end stream to other channels subscribers
func (s *StreamPriceCompany) StartStreaming(ctx context.Context) {
	log.Debug("Start stream StartStreaming")
	for {
		select {
		case <-ctx.Done():
			return
		case pr := <-s.DataChan:
			s.rwm.RLock()
			if s.Subscribers.Len() > 0 {
				s.rwm.RUnlock()
				tt := time.Now()
				s.rwm.Lock()
				for chElem := s.Subscribers.Front(); chElem != nil; {
					select {
					case _, op := <-chElem.Value.(chan *model.Price):
						log.Debug("Chanel was deleted")
						if chElem.Next() == nil {
							s.Subscribers.Remove(chElem)
							chElem = chElem.Next()
							continue
						}
						chElem = chElem.Next()
						s.Subscribers.Remove(chElem.Prev())
						_ = op
					default:
						chElem.Value.(chan *model.Price) <- pr
						chElem = chElem.Next()
					}
				}
				s.rwm.Unlock()
				log.Info(time.Since(tt), " Count Position : ", s.Subscribers.Len())
				continue
			}
			s.rwm.RUnlock()
		}
	}
}
