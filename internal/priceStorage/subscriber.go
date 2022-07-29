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

// NewSubscriber Constructor
func NewSubscriber() *StreamPriceCompany {
	log.Debug("func NewSubscriber() *StreamPriceCompany ")
	return &StreamPriceCompany{
		DataChan:    make(chan *model.Price),
		Subscribers: list.New(),
	}
}

// AddSubscriber add new subscriber to subscribers
func (s *StreamPriceCompany) AddSubscriber(chTo chan *model.Price) {
	s.rwm.Lock()
	s.Subscribers.PushBack(chTo)
	s.rwm.Lock()
}

// StartStreaming goroutine from listen chanel update end stream to other channels subscribers
func (s *StreamPriceCompany) StartStreaming(ctx context.Context) {
	log.Debug("Start stream StartStreaming")
	for {
		select {
		case <-ctx.Done():
			return
		case pr := <-s.DataChan:
			log.Debug("Get Data from Chanal")
			if s.Subscribers.Len() > 0 {
				log.Debug("Get Data from Chanal LEN > 0")
				tt := time.Now()
				for chElem := s.Subscribers.Front(); chElem != nil; chElem = chElem.Next() {
					log.Debug("Send to chanel ", chElem.Value)
					select {
					case _, op := <-chElem.Value.(chan *model.Price):
						log.Debug("Closed Chanel")
						if chElem.Next() != nil {
							chElem = chElem.Next()
							s.Subscribers.Remove(chElem.Prev())
							continue
						}
						s.Subscribers.Remove(chElem)
						_ = op
					default:
						log.Debug("Ch S")
						chElem.Value.(chan *model.Price) <- pr
						log.Debug("Ch D")
					}
				}
				log.Info(time.Since(tt), " Count Position : ", s.Subscribers.Len())
			}
		}
	}
}
