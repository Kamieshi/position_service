package model

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// User model user
type User struct {
	ID      uuid.UUID `db:"id" pg:"type:uuid" bun:"type:uuid,default:uuid_generate_v4()"`
	Name    string    `db:"name"`
	Balance int64     `db:"balance"`
	Rwm     sync.RWMutex
}

// ToString output format "c.ID|c.Name|c.Balance"
func (c *User) ToString() string {
	return fmt.Sprintf("%s|%s|%d", c.ID, c.Name, c.Balance)
}

// UserFromString input format "c.ID|c.Name|c.Balance"
func UserFromString(strUser string) (*User, error) {
	data := strings.Split(strUser, "|")
	id, err := uuid.Parse(data[0])
	if err != nil {
		return nil, fmt.Errorf("mdoels user / FromString error parse : %v", err)
	}
	balance, err := strconv.Atoi(data[2])
	if err != nil {
		return nil, fmt.Errorf("mdoels user / FromString error parse : %v", err)
	}
	return &User{
		ID:      id,
		Name:    data[1],
		Balance: int64(balance),
	}, nil
}
