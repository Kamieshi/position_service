package service

import (
	"fmt"
	"sync"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/priceStorage"
	"github.com/Kamieshi/position_service/internal/repository"
	"github.com/Kamieshi/position_service/internal/userStorage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// PositionsService  Main service for work with positions
type PositionsService struct {
	UsersPositions     map[uuid.UUID]*UserPositions
	PriceStorage       *priceStorage.PriceStore
	UserStorage        *userStorage.UserService
	PositionRepository *repository.PositionRepository
	CtxApp             context.Context
	rwm                sync.RWMutex
}

const (
	chanelOpenPosition  = "open_position"
	chanelClosePosition = "close_position"
)

// OpenPosition open position
func (p *PositionsService) OpenPosition(ctx context.Context, position *model.Position) error {
	logrus.Debug("Position service / OpenPosition ")
	if state, err := p.checkActualOpenedPriceState(position); err != nil {
		return fmt.Errorf("service position / OpenPosition / Error get actualState price : %v ", err)
	} else if !state {
		return fmt.Errorf("service position / OpenPosition / Opened price isn't actual ")
	}

	if enough, err := p.checkEnoughBalanceUser(ctx, position); err != nil {
		return fmt.Errorf("service position / OpenPosition / try check user balance : %v ", err)
	} else if !enough {
		return fmt.Errorf("service position / OpenPosition / Isn't enough monay for open position")
	}

	if position.IsFixes {
		if !p.checkStartStateProfit(position) {
			return fmt.Errorf("invalid fixed param ")
		}
	}

	if !p.userPositionsIsExist(position.User.ID) {
		p.addNewUserPositions(position.User.ID)
	}

	activePosition := NewActiveOpenedPosition(position)

	if err := p.addToUserPositions(activePosition); err != nil {
		return fmt.Errorf("service position / OpenPosition / add active position to user activePositions : %v ", err)
	}
	if err := p.writeNewToDB(activePosition); err != nil {
		return fmt.Errorf("service position / OpenPosition / write position to DB: %v ", err)
	}
	p.startTakeActualStateAndAddSubscriber(activePosition)
	return nil
}

// ClosePosition close position
func (p *PositionsService) ClosePosition(ctx context.Context, userID, positionID uuid.UUID) (*model.Position, error) {
	logrus.Debug("Position service / ClosePosition ")
	if !p.userPositionsIsExist(userID) {
		return nil, fmt.Errorf("ActivePosition service / CloasePosition / user %s not exist", userID)
	}
	position, err := p.closePositionForUser(userID, positionID)
	if err != nil {
		return nil, fmt.Errorf("position service / FixedClosedPosition / close position : %v", err)
	}
	return position, nil
}

// WriteClosedPositions G for write closed position in db from buffer closed position
func (p *PositionsService) WriteClosedPositions(ctx context.Context, userID uuid.UUID) {
	logrus.Debug("start WriteClosedPositions")
	logrus.Debugf("Start WriteClosedPositions for user %v", userID)
	p.rwm.RLock()
	bufChan := p.UsersPositions[userID].BufferWriteClosePositionsInDB
	p.rwm.RUnlock()
	for {
		select {
		case <-ctx.Done():
			return
		case position := <-bufChan:
			logrus.Debug("Was send message in buffer : ", position)
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

			cm, err := tx.Exec(ctx, fmt.Sprintf("NOTIFY %s, '%s'", chanelClosePosition, position.ID.String()))
			if err != nil {
				logrus.WithError(err).Error("position service / WriteClosedPositions / Send notify : ", cm.String())
			}
			err = tx.Commit(ctx)
			if err != nil {
				logrus.WithError(err).Error("position service / WriteClosedPositions / Commit transaction")
			}
		}
	}
}

// SyncPositionService G listen open and close position channels from DB and sync current instance
func (p *PositionsService) SyncPositionService(ctx context.Context) {
	logrus.Debug("start SyncPositionService")
	conn, err := p.getInitListenConnection(ctx)
	if err != nil {
		logrus.WithError(err).Error("position service / SyncPositionService / Get connection ")
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, err := conn.WaitForNotification(ctx)
			logrus.Debug("Was send sync message : ", message)
			if err != nil {
				logrus.WithError(err).Error("position service / SyncPostgresChanel / send listen position_close")
				continue
			}
			positionID, err := uuid.Parse(message.Payload)
			if err != nil {
				logrus.WithError(err).Error("position service / SyncPostgresChanel / parse id from message : ", message.Payload)
				continue
			}
			position, err := p.getPositionFromRepository(positionID)
			if err != nil {
				logrus.WithError(err).Error("position service / SyncPostgresChanel / get position from db ")
				continue
			}

			switch message.Channel {
			case chanelOpenPosition:
				if err := p.openPositionTriggeredSync(position); err != nil {
					logrus.WithError(err).Warn("position service / SyncPostgresChanel / open position")
					continue
				}
			case chanelClosePosition:
				if err := p.closePositionTriggeredSync(position); err != nil {
					logrus.WithError(err).Warn("position service / SyncPostgresChanel / open position")
					continue
				}
			default:
				logrus.Errorf("Incorrect chanel name %s", message.Channel)
			}
		}
	}
}

func (p *PositionsService) GetByID(ctx context.Context, positionID uuid.UUID) (*model.Position, error) {
	pos, err := p.PositionRepository.Get(ctx, positionID)
	if err != nil {
		return nil, fmt.Errorf("Position service / GetById / Get position from rep : %v", err)
	}
	return pos, nil
}

func (p *PositionsService) GetAllUserPositions(ctx context.Context, userID uuid.UUID) ([]*model.Position, error) {
	positions, err := p.PositionRepository.GetAllUserPositions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service position / GetAllUserPositions / get positions form repository : %v ", err)
	}
	return positions, err
}

func (p *PositionsService) userPositionsIsExist(userID uuid.UUID) bool {
	if _, exist := p.UsersPositions[userID]; !exist {
		return false
	}
	return true
}

func (p *PositionsService) closePositionForUser(userID, positionID uuid.UUID) (*model.Position, error) {
	p.rwm.RLock()
	position, err := p.UsersPositions[userID].CloseByID(positionID)
	p.rwm.RUnlock()
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
	return true, nil
}

func (p *PositionsService) checkEnoughBalanceUser(ctx context.Context, position *model.Position) (bool, error) {
	user, err := p.UserStorage.Get(ctx, position.User.ID)
	if err != nil {
		return false, fmt.Errorf("service position / checkEnoughBalanceUser / Try get user : %v ", err)
	}
	if position.IsSales {
		return true, nil
	}
	if int64(position.OpenPrice.Ask*position.CountBuyPosition) > user.Balance {
		return false, nil
	}
	return true, nil
}

func (p *PositionsService) checkStartStateProfit(position *model.Position) bool {
	profit := int64(position.CountBuyPosition*position.OpenPrice.Bid) - int64(position.CountBuyPosition*position.OpenPrice.Ask)
	return profit >= position.MinCurrentCost && profit <= position.MaxCurrentCost
}

func (p *PositionsService) addNewUserPositions(userID uuid.UUID) {
	p.rwm.Lock()
	p.UsersPositions[userID] = NewUserPositions(p.UserStorage)
	p.rwm.Unlock()
	go p.UsersPositions[userID].FixedClosedActivePositions()
	go p.UsersPositions[userID].CheckCommonProfit()
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

func (p *PositionsService) writeNewToDB(activePosition *ActivePosition) error {
	logrus.Debug("writeNewToDB")
	tx, err := p.PositionRepository.Pool.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("PositionManager service_old/ writeNewToDB / get tx from pool : %v", err)
	}

	err = p.PositionRepository.InsertTx(context.Background(), tx, activePosition.position)
	if err != nil {
		return fmt.Errorf("service position / writeNewToDB / Insert to position into DB : %v ", err)
	}

	cm, err := tx.Exec(context.Background(), fmt.Sprintf("NOTIFY %s, '%s'", chanelOpenPosition, activePosition.position.ID.String()))
	if err != nil {
		logrus.WithError(err).Error("position service / WriteClosedPositions / Send notify : ", cm.String())
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("service position / writeNewToDB / Commit transaction : %v ", err)
	}
	return nil
}

func (p *PositionsService) getInitListenConnection(ctx context.Context) (*pgx.Conn, error) {
	chListenerConnection, err := p.PositionRepository.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("position service / getInitListenConnection / cannot get connection from pgxPool : %v", err)
	}
	cm, err := chListenerConnection.Exec(
		context.Background(),
		fmt.Sprintf("LISTEN %s;", chanelOpenPosition),
	)
	if err != nil {
		return nil, fmt.Errorf("position service / getInitListenConnection / send listen position_open : %v .%s", err, cm.String())
	}
	cm, err = chListenerConnection.Exec(
		context.Background(),
		fmt.Sprintf("LISTEN %s;", chanelClosePosition),
	)
	if err != nil {
		logrus.WithError(err).Error("position service / getInitListenConnection / send listen position_close : ", cm.String())
		return nil, fmt.Errorf("position service / getInitListenConnection /  send listen position_close : %v .%s", err, cm.String())
	}
	return chListenerConnection.Conn(), nil
}

func (p *PositionsService) openPositionTriggeredSync(position *model.Position) error {
	if !p.userPositionsIsExist(position.User.ID) {
		p.addNewUserPositions(position.User.ID)
	}
	activePosition := NewActiveOpenedPosition(position)
	if err := p.addToUserPositions(activePosition); err != nil {
		return fmt.Errorf("service position / openPositionTriggeredSync / add active position to user activePositions : %v ", err)
	}
	p.startTakeActualStateAndAddSubscriber(activePosition)
	return nil
}

func (p *PositionsService) closePositionTriggeredSync(position *model.Position) error {
	if p.userPositionsIsExist(position.User.ID) {
		p.rwm.RLock()
		err := p.UsersPositions[position.User.ID].CloseTriggeredSync(position)
		p.rwm.RUnlock()
		if err != nil {
			return fmt.Errorf("position service / closePositionTriggeredSync / Close position : %v", err)
		}
	}
	return nil
}

func (p *PositionsService) getPositionFromRepository(positionID uuid.UUID) (*model.Position, error) {
	position, err := p.PositionRepository.Get(context.Background(), positionID)
	if err != nil {
		return nil, fmt.Errorf("position service / getPositionFromRepository / get position %s from Rep : %v", positionID, err)
	}
	return position, nil
}
