// Package handlers gRPC Handlers
package handlers

import (
	"context"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/Kamieshi/position_service/internal/model"
	"github.com/Kamieshi/position_service/internal/userStorage"
	"github.com/Kamieshi/position_service/protoc"
)

// UsersManagerServerImplement work with Users
type UsersManagerServerImplement struct {
	UserService *userStorage.UserService
	protoc.UsersManagerServer
}

// GetUser Get User
func (c *UsersManagerServerImplement) GetUser(ctx context.Context, req *protoc.GetUserRequest) (*protoc.GetUserResponse, error) {
	log.Debug("Handler Get User ", req)
	User, err := c.UserService.GetByName(ctx, req.Name)
	if err != nil {
		log.WithError(err).Error("GetUser handler UserManagerServer")
		return &protoc.GetUserResponse{Error: err.Error()}, nil
	}
	return &protoc.GetUserResponse{
		User: &protoc.User{
			ID:      User.ID.String(),
			Name:    User.Name,
			Balance: User.Balance,
		},
	}, nil
}

// CreateUser Create newUser
func (c *UsersManagerServerImplement) CreateUser(ctx context.Context, req *protoc.CreateUserRequest) (*protoc.CreateUserResponse, error) {
	log.Debug("Handler Create User", req)
	User := &model.User{Name: req.User.Name, Balance: req.User.Balance}
	err := c.UserService.CreateUser(ctx, User)
	if err != nil {
		log.WithError(err).Warningln("CreateUser handler UserManagerServer")
		return &protoc.CreateUserResponse{
			User:  req.User,
			Error: err.Error(),
		}, nil
	}
	return &protoc.CreateUserResponse{
		User: &protoc.User{
			ID:      User.ID.String(),
			Name:    User.Name,
			Balance: User.Balance,
		},
	}, nil
}

// AddBalance Add some different to balance User (positive/negative ~ add/take off)
func (c *UsersManagerServerImplement) AddBalance(ctx context.Context, req *protoc.AddBalanceRequest) (*protoc.AddBalanceResponse, error) {
	log.Debug("Handler Add Balance", req)
	id, err := uuid.Parse(req.UserID)
	if err != nil {
		log.WithError(err).Error("ErrorParse,AddBalance handler UserManagerServer")
		return &protoc.AddBalanceResponse{Error: err.Error()}, nil
	}
	User, err := c.UserService.Get(ctx, id)
	if err != nil {
		log.WithError(err).Error("ErrorGetUser,AddBalance handler UserManagerServer")
		return &protoc.AddBalanceResponse{Error: err.Error()}, nil
	}

	if User.Balance+req.DifferentBalance < 0 {
		log.WithError(err).Error("user handler / AddBalance / low balance ")
		return &protoc.AddBalanceResponse{Error: "Don't have enough money"}, nil
	}

	User.Balance += req.DifferentBalance
	err = c.UserService.Update(ctx, User)
	if err != nil {
		log.WithError(err).Error("ErrorUpdate,AddBalance handler UserManagerServer")
		return &protoc.AddBalanceResponse{Error: err.Error()}, nil
	}
	return &protoc.AddBalanceResponse{}, nil
}

// GetAllUsers Get all users
func (c *UsersManagerServerImplement) GetAllUsers(ctx context.Context, req *protoc.GetAllUserRequest) (*protoc.GetAllUsersResponse, error) {
	log.Debug("Handler GetAllUser", req)
	Users, err := c.UserService.GetAll(ctx)
	if err != nil {
		log.WithError(err).Error("GetAllUser handler UserManagerServer")
		return &protoc.GetAllUsersResponse{Error: err.Error()}, nil
	}
	protoUsers := make([]*protoc.User, 0, len(Users))
	for _, cl := range Users {
		protoUsers = append(protoUsers, &protoc.User{
			ID:      cl.ID.String(),
			Name:    cl.Name,
			Balance: cl.Balance,
		})
	}
	return &protoc.GetAllUsersResponse{Users: protoUsers}, nil
}
