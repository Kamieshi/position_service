// Package repository
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/Kamieshi/position_service/internal/model"
)

// PositionRepository repo for work with positions
type PositionRepository struct {
	Pool *pgxpool.Pool
}

// InsertTx Create new position
func (p *PositionRepository) InsertTx(ctx context.Context, tx pgx.Tx, position *model.Position) error {
	querySQL := `INSERT INTO positions(
		id, "user", company, ask_open, bid_open, is_opened,close_profit, time_price_open,count_buy_position,
		max_position_cost, min_position_cost, is_sales, is_fixed)
		VALUES
		($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	cm, err := tx.Exec(ctx, querySQL,
		position.ID,
		position.User.ID,
		position.CompanyID,
		position.OpenPrice.Ask,
		position.OpenPrice.Bid,
		position.IsOpened,
		0,
		position.OpenPrice.Time,
		position.CountBuyPosition,
		position.MaxCurrentCost,
		position.MinCurrentCost,
		position.IsSales,
		position.IsFixes,
	)
	if err != nil {
		return fmt.Errorf("repository Position/InsertTx : %v ", err)
	}
	if !cm.Insert() {
		return fmt.Errorf("repository Position/InsertTx, incorrect data for INISERT : %v ", cm.String())
	}
	return nil
}

// ClosePositionTx Close current position. Add exec into transaction
func (p *PositionRepository) ClosePositionTx(ctx context.Context, tx pgx.Tx, position *model.Position) error {
	querySQL := "UPDATE positions SET is_opened=$1 , close_profit=$2 WHERE id =$3"
	cm, err := tx.Exec(ctx, querySQL, position.IsOpened, position.Profit, position.ID)
	if err != nil {
		return fmt.Errorf("repository Position/ClosePositionTx : %v ", err)
	}
	if !cm.Update() {
		return fmt.Errorf("repository Position/ClosePositionTx, incorrect data for UPDATE : %v ", cm.String())
	}
	return nil
}

// Get position from db
func (p *PositionRepository) Get(ctx context.Context, positionID uuid.UUID) (*model.Position, error) {
	position := model.Position{}
	position.User = &model.User{}
	position.OpenPrice = model.Price{}
	querySQL := "SELECT " +
		"id, user, company, ask_open, bid_open, is_opened,close_profit, time_price_open,count_buy_position, max_position_cost, min_position_cost," +
		" is_sales, is_fixed FROM positions WHERE id=$1;"
	err := p.Pool.QueryRow(ctx, querySQL, positionID).Scan(
		&position.ID,
		&position.User.ID,
		&position.CompanyID,
		&position.OpenPrice.Ask,
		&position.OpenPrice.Bid,
		&position.IsOpened,
		&position.Profit,
		&position.OpenPrice.Time,
		&position.CountBuyPosition,
		&position.MaxCurrentCost,
		&position.MinCurrentCost,
		&position.IsSales,
		&position.IsFixes,
	)
	if err != nil {
		return nil, fmt.Errorf("repository Position/Get : %v ", err)
	}
	return &position, err
}
