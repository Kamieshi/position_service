// Package userStorage
package userStorage

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/repository"
)

// UserService struct for work with Users
type UserService struct {
	UserRep *repository.UserRepository
	Users   map[uuid.UUID]*model.User
	sync.RWMutex
}

// NewUserService Constructor
func NewUserService(ctx context.Context, UserRep *repository.UserRepository) *UserService {
	clServise := &UserService{
		UserRep: UserRep,
		Users:   make(map[uuid.UUID]*model.User),
	}
	var err error
	con, err := UserRep.Pool.Acquire(ctx)
	if err != nil {
		log.WithError(err).Fatal("service_old User / Get connection from pool ")
	}
	_, err = con.Exec(ctx, "LISTEN update_user")
	if err != nil {
		log.WithError(err).Fatal()
	}
	// Listening end sync users
	go func() {

		for {
			select {
			case <-ctx.Done():
				return
			default:
				notification, err := con.Conn().WaitForNotification(ctx)
				log.Debug("Notification was send ", notification.Payload)
				if err != nil {
					log.WithError(err).Error("Listener chanel sync users")
					continue
				}
				UserFromPool, err := model.UserFromString(notification.Payload)
				if err != nil {
					log.WithError(err).Error("service_old User / parse from String ", notification.Payload)
					continue
				}
				clServise.RLock()
				User, exist := clServise.Users[UserFromPool.ID]
				clServise.RUnlock()
				if exist {
					if User.ToString() == notification.Payload {
						continue
					}
					err = clServise.Sync(ctx, User)
					if err != nil {
						log.WithError(err).Error("service_old User / Error sync ", notification.Payload)
					}
				}
			}
		}
	}()
	return clServise
}

// Get User from cache or DB
func (c *UserService) Get(ctx context.Context, id uuid.UUID) (*model.User, error) {
	c.RLock()
	User, exist := c.Users[id]
	c.RUnlock()
	if exist {
		return User, nil
	}
	User, err := c.UserRep.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("serviece User/ Get : %v", err)
	}
	c.Lock()
	c.Users[User.ID] = User
	c.Unlock()
	if err != nil {
		return nil, errors.New("UserService / GetUser : User isn't exist")
	}
	return User, nil
}

func (c *UserService) AddProfit(profit int64, userID uuid.UUID) error {
	user, err := c.Get(context.Background(), userID)
	if err != nil {
		return fmt.Errorf("serviece User/ AddProfit : %v", err)
	}
	c.Lock()
	user.Balance += profit
	c.Unlock()
	return nil
}

// Sync User with DB User
func (c *UserService) Sync(ctx context.Context, User *model.User) error {
	c.RLock()
	cl, exist := c.Users[User.ID]
	if !exist {
		c.RUnlock()
		log.Error(fmt.Errorf("User service_old / sync : User isn't in UserService cache %v", User))
		return nil
	}
	clFromDB, err := c.UserRep.GetByID(ctx, User.ID)
	if err != nil {
		return fmt.Errorf("User service_old / sync : %v", err)
	}
	cl.Lock()
	cl.Balance = clFromDB.Balance
	cl.Name = clFromDB.Name
	cl.Unlock()
	return nil
}

// GetByName get by name
func (c *UserService) GetByName(ctx context.Context, name string) (*model.User, error) {
	User, err := c.UserRep.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return User, nil
}

// CreateUser User from cache and DB
func (c *UserService) CreateUser(ctx context.Context, User *model.User) error {
	err := c.UserRep.Insert(ctx, User)
	if err != nil {
		return fmt.Errorf("service_old User / SetcUser Error from Repository : %v", err)
	}
	return nil
}

// Update User
func (c *UserService) Update(ctx context.Context, User *model.User) error {
	tx, err := c.UserRep.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("User service_old/ Update /Open Begin: %v ", err)
	}
	err = c.UserRep.UpdateTx(ctx, tx, User)
	if err != nil {
		return fmt.Errorf("User service_old / Update / Update User TX: %v ", err)
	}
	c.RLock()
	cl, exist := c.Users[User.ID]
	c.RUnlock()
	if exist {
		c.Lock()
		cl.Balance = User.Balance
		cl.Name = User.Name
		c.Unlock()
	}
	cm, err := tx.Exec(ctx, fmt.Sprintf("NOTIFY update_user, '%s'", cl.ToString()))
	if err != nil {
		return fmt.Errorf("User service_old / Update / Send notify err: %v, message %s ", err, cm.String())
	}
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("User service_old / Update : %v ", err)
	}
	return nil
}

// GetAll users
func (c *UserService) GetAll(ctx context.Context) ([]*model.User, error) {
	Users, err := c.UserRep.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("Service User / GetAll / get all Users from repository User : %v", err)
	}
	return Users, nil
}

func (c *UserService) AddProfitInRepositoryTX(ctx context.Context, tx pgx.Tx, userID uuid.UUID, profit int64) error {
	err := c.UserRep.AddProfitTX(ctx, tx, userID, profit)
	if err != nil {
		return fmt.Errorf("User storage / AddProfitInRepositoryTX / Try add profit to user : %v", err)
	}
	return nil
}
