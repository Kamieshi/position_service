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
	sync.RWMutex
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
	p.RLock()
	if data, exist := p.Companies[companyID]; exist {
		p.RUnlock()
		return data, nil
	}
	p.RUnlock()
	return nil, fmt.Errorf("service_old priceStorage store /GetPrice :%v", fmt.Errorf("company {%v} not found", companyID))
}

// SetPrice Set New or Update company last priceStorage
func (p *PriceStore) SetPrice(companyID string, pr *model.Price) {
	logrus.Debug("SetPrice")
	p.Lock()
	if _, exist := p.Companies[companyID]; !exist {
		logrus.WithField("Company", fmt.Sprintf("%v", companyID)).Info()
	}
	p.Companies[companyID] = pr
	p.Unlock()
	p.RLock()
	_, ex := p.StreamSubscribers[companyID]
	p.RUnlock()
	if !ex {
		p.Lock()
		p.StreamSubscribers[companyID] = NewSubscriber()
		p.Unlock()
		go p.StreamSubscribers[companyID].StartStreaming(p.CtxApp)
	}
	logrus.Debug("SetPrice SS")
	p.StreamSubscribers[companyID].DataChan <- pr
	logrus.Debug("SetPrice SSS")
}

// ListenStream Goroutine from save Price store in consistent state with Redis Stream
func (p *PriceStore) ListenStream(ctx context.Context) {
	logrus.Debug("Start listening from GRPC")
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
			logrus.Debug("Get data from GRPC riceStorage")
			if err != nil {
				logrus.WithError(err).Error("service_old priceStorage store/ListenStream stream.Recv()")
			}
			tt, err := time.Parse("2006-01-02T15:04:05.000TZ-07:00", data.Time)

			if err != nil {
				logrus.WithError(err).Error("service_old priceStorage store/ListenStream Parse time")
			}
			p.SetPrice(data.Company.ID, &model.Price{
				Ask:  data.Ask,
				Bid:  data.Bid,
				Time: tt,
			})
		}
	}
}

// AddSubscriber Add new subscriber to definite Company
func (p *PriceStore) AddSubscriber(ch chan *model.Price, companyID string) {
	logrus.Debug("AddSubscriber")
	p.Lock()
	_, exist := p.StreamSubscribers[companyID]
	if !exist {
		logrus.Debug("New Subscriber")
		p.StreamSubscribers[companyID] = NewSubscriber()
	}

	p.StreamSubscribers[companyID].AddSubscriber(ch)
	logrus.Debug("Was Added")
	p.Unlock()
}
