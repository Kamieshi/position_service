// package repository
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"

	"github.com/Kamieshi/position_service/internal/model"
)

// UserRepository rep for work with client struct
type UserRepository struct {
	Pool *pgxpool.Pool
}

// Insert insert new client
func (c *UserRepository) Insert(ctx context.Context, client *model.User) error {
	client.ID = uuid.New()
	querySQL := "INSERT INTO clients(id,name,balance) VALUES($1,$2,$3)"
	cm, err := c.Pool.Exec(ctx, querySQL, client.ID, client.Name, client.Balance)
	if err != nil {
		return fmt.Errorf("repository client/Insert : %v ", err)
	}
	if !cm.Insert() {
		return fmt.Errorf("repository client/Insert, incorrect data for INISERT : %v ", cm.String())
	}
	return nil
}

// Update update clients if this client exist
func (c *UserRepository) Update(ctx context.Context, client *model.User) error {
	querySQL := "UPDATE clients SET name=$1,balance=$2 WHERE id=$3"
	cm, err := c.Pool.Exec(ctx, querySQL, client.Name, client.Balance, client.ID)
	if err != nil {
		return fmt.Errorf("repository client/Update : %v ", err)
	}
	if !cm.Update() {
		return fmt.Errorf("repository client/Update, incorrect data for Update : %v ", cm.String())
	}
	return nil
}

// Delete delete client
func (c *UserRepository) Delete(ctx context.Context, clientID uuid.UUID) error {
	querySQL := "DELETE FROM clients WHERE id=$1"
	cm, err := c.Pool.Exec(ctx, querySQL, clientID)
	if err != nil {
		return fmt.Errorf("repository client/Delete : %v ", err)
	}
	if !cm.Delete() {
		log.Errorf("User %s was delete", clientID.String())
	}
	return nil
}

// GetByID client by ID
func (c *UserRepository) GetByID(ctx context.Context, clientID uuid.UUID) (*model.User, error) {
	querySQL := "SELECT id,name,balance FROM clients WHERE id=$1"
	client := model.User{}
	if err := c.Pool.QueryRow(ctx, querySQL, clientID).Scan(&client.ID, &client.Name, &client.Balance); err != nil {
		return nil, fmt.Errorf("repository client/GetByID : %v .Input value : %v", err, clientID)
	}
	return &client, nil
}

// GetByName client by name
func (c *UserRepository) GetByName(ctx context.Context, clientName string) (*model.User, error) {
	querySQL := "SELECT id,name,balance FROM clients WHERE name=$1"
	client := model.User{}
	if err := c.Pool.QueryRow(ctx, querySQL, clientName).Scan(&client.ID, &client.Name, &client.Balance); err != nil {
		return nil, fmt.Errorf("repository client/GetByName : %v ", err)
	}
	return &client, nil
}

// UpdateTx update clients if this client exist
func (c *UserRepository) UpdateTx(ctx context.Context, tx pgx.Tx, client *model.User) error {
	querySQL := "UPDATE clients SET name=$1,balance=$2 WHERE id=$3"
	cm, err := tx.Exec(ctx, querySQL, client.Name, client.Balance, client.ID)
	if err != nil {
		return fmt.Errorf("repository client/Update : %v ", err)
	}
	if !cm.Update() {
		return fmt.Errorf("repository client/Update, incorrect data for Update : %v ", cm.String())
	}
	return nil
}

func (c *UserRepository) GetAll(ctx context.Context) ([]*model.User, error) {
	rows, err := c.Pool.Query(ctx, "SELECT id, name,balance FROM clients")
	if err != nil {
		return nil, fmt.Errorf("repository client / GetAll / Error response from BD : %v", err)
	}
	clients := make([]*model.User, 0, len(rows.RawValues()))
	for rows.Next() {
		client := model.User{}
		err = rows.Scan(
			&client.ID,
			&client.Name,
			&client.Balance,
		)
		if err != nil {
			return nil, fmt.Errorf("repository client / GetAll / Error scan : %v", err)
		}
		clients = append(clients, &client)
	}
	return clients, nil
}
