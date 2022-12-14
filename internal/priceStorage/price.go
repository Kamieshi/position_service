// Package priceStorage
package priceStorage

import (
	"fmt"
	"sync"
	"time"

	"github.com/Kamieshi/price_service/protoc"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/Kamieshi/position_service/internal/model"
)

// PriceStore Common Price store from all Position
type PriceStore struct {
	rwm               sync.RWMutex
	PricesStream      protoc.OwnPriceStreamClient
	Companies         map[string]*model.Price
	StreamSubscribers map[string]*StreamPriceCompany
	CtxApp            context.Context
}

// NewPriceStore Constructor
func NewPriceStore(ctx context.Context, grpcStreamResponse protoc.OwnPriceStreamClient) *PriceStore {
	prStorage := &PriceStore{
		PricesStream:      grpcStreamResponse,
		Companies:         make(map[string]*model.Price),
		StreamSubscribers: make(map[string]*StreamPriceCompany),
		CtxApp:            ctx,
	}
	return prStorage
}

// GetPrice Company priceStorage
func (p *PriceStore) GetPrice(companyID string) (*model.Price, error) {
	logrus.Debug("GetPrice priceStorage")
	p.rwm.RLock()
	if data, exist := p.Companies[companyID]; exist {
		p.rwm.RUnlock()
		return data, nil
	}
	p.rwm.RUnlock()
	return nil, fmt.Errorf("service_old priceStorage store /GetPrice :%v", fmt.Errorf("company {%v} not found", companyID))
}

// SetPrice Set New or Update company last priceStorage
func (p *PriceStore) SetPrice(companyID string, pr *model.Price) {
	p.rwm.Lock()
	if _, exist := p.Companies[companyID]; !exist {
		logrus.WithField("Company", fmt.Sprintf("%s", companyID)).Info()
	}
	p.Companies[companyID] = pr
	p.rwm.Unlock()
	logrus.Debug("Price For company ", companyID, " was updated. Current Delay ", time.Since(pr.Time))
}

// ShareToSubscribers Share new price into concrete StreamPriceCompany
func (p *PriceStore) ShareToSubscribers(companyID string, price *model.Price) {
	if !p.checkExistSubscriberStream(companyID) {
		p.rwm.Lock()
		p.StreamSubscribers[companyID] = NewStreamPriceCompany()
		go p.StreamSubscribers[companyID].StartStreaming(p.CtxApp)
		p.rwm.Unlock()
	}
	p.StreamSubscribers[companyID].DataChan <- price
}

func (p *PriceStore) checkExistSubscriberStream(companyID string) bool {
	p.rwm.RLock()
	_, ex := p.StreamSubscribers[companyID]
	p.rwm.RUnlock()
	return ex
}

// ListenStream Goroutine from save Price store in consistent state with Redis Stream
func (p *PriceStore) ListenStream(ctx context.Context) {
	logrus.Debug("Start to listen stream from GRPC")
	stream, err := p.PricesStream.GetPriceStream(ctx, &protoc.GetPriceStreamRequest{})
	if err != nil {
		logrus.WithError(err).Error("Start listen priceStorage stream")
		panic(err)
	}
	for {
		select {
		case <-ctx.Done():
			logrus.Info("Goroutine listener priceStorage stream was Done")
			return
		default:
			data, err := stream.Recv()
			if err != nil {
				logrus.WithError(err).Error("service_old priceStorage store/ListenStream stream.Recv()")
			}
			tt, err := time.Parse("2006-01-02T15:04:05.000TZ-07:00", data.Time)

			if err != nil {
				logrus.WithError(err).Error("service_old priceStorage store/ListenStream Parse time")
			}
			price := &model.Price{
				Ask:  data.Ask,
				Bid:  data.Bid,
				Time: tt,
			}
			p.SetPrice(data.Company.ID, price)
			p.ShareToSubscribers(data.Company.ID, price)
		}
	}
}

// AddSubscriber Add new subscriber to definite Company
func (p *PriceStore) AddSubscriber(ch chan *model.Price, companyID string) {
	logrus.Debug("AddSubscriber fot company ", companyID)
	p.rwm.RLock()
	_, exist := p.StreamSubscribers[companyID]
	p.rwm.RUnlock()
	if !exist {
		logrus.Debug("New Subscriber for company ", companyID)
		p.rwm.Lock()
		p.StreamSubscribers[companyID] = NewStreamPriceCompany()
		p.rwm.Unlock()
	}
	p.StreamSubscribers[companyID].AddSubscriber(ch)
}
