package model

import "github.com/google/uuid"

// Position model position
type Position struct {
	ID               uuid.UUID `db:"id,omitempty"`
	Client           *User     `db:"client,omitempty"`
	OpenPrice        Price     `db:"open_price,omitempty"`
	CompanyID        string    `db:"company"`
	IsOpened         bool      `db:"is_opened,omitempty"`
	PriceClose       uint32    `db:"price_close,omitempty"`
	Profit           int64     `db:"profit,omitempty"`
	MaxCurrentCost   int64     `db:"max_position_cost,omitempty"`
	MinCurrentCost   int64     `db:"min_position_cost,omitempty"`
	CountBuyPosition uint32    `db:"count_buy_position"`
	IsSales          bool      `db:"is_sales"` // true/false : sale/buy
	IsFixes          bool      `db:"is_fixes"` // user limit or not
}
