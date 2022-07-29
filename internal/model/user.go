package model

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// User model client
type User struct {
	ID      uuid.UUID `db:"id"`
	Name    string    `db:"name"`
	Balance int64     `db:"balance"`
	sync.RWMutex
}

// ToString output format "c.ID|c.Name|c.Balance"
func (c *User) ToString() string {
	return fmt.Sprintf("%s|%s|%d", c.ID, c.Name, c.Balance)
}

// ClientFromString input format "c.ID|c.Name|c.Balance"
func UserFromString(strClient string) (*User, error) {
	data := strings.Split(strClient, "|")
	id, err := uuid.Parse(data[0])
	if err != nil {
		return nil, fmt.Errorf("mdoels client / FromString error parse : %v", err)
	}
	balance, err := strconv.Atoi(data[2])
	if err != nil {
		return nil, fmt.Errorf("mdoels client / FromString error parse : %v", err)
	}
	return &User{
		ID:      id,
		Name:    data[1],
		Balance: int64(balance),
	}, nil
}
