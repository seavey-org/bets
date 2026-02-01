package models

import "time"

type MarketStatus string

const (
	MarketStatusOpen     MarketStatus = "open"
	MarketStatusClosed   MarketStatus = "closed"
	MarketStatusResolved MarketStatus = "resolved"
	MarketStatusCancelled MarketStatus = "cancelled"
)

const (
	PointsLogMarketBuy    PointsLogType = "market_buy"
	PointsLogMarketSell   PointsLogType = "market_sell"
	PointsLogMarketWin    PointsLogType = "market_win"
	PointsLogMarketRefund PointsLogType = "market_refund"
)

// Market represents a prediction market with continuous trading via CPMM.
type Market struct {
	ID               string          `json:"id" gorm:"primaryKey;type:text"`
	GroupID          string          `json:"group_id" gorm:"index;type:text;not null"`
	Title            string          `json:"title" gorm:"type:text;not null"`
	Description      string          `json:"description" gorm:"type:text"`
	Status           MarketStatus    `json:"status" gorm:"type:text;not null;default:open"`
	CreatedBy        string          `json:"created_by" gorm:"type:text;not null"`
	WinningOutcomeID string          `json:"winning_outcome_id,omitempty" gorm:"type:text"`
	ResolvedAt       *time.Time      `json:"resolved_at"`
	ClosesAt         *time.Time      `json:"closes_at"` // Optional auto-close time
	CreatedAt        time.Time       `json:"created_at"`
	Creator          User            `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Outcomes         []MarketOutcome `json:"outcomes,omitempty" gorm:"foreignKey:MarketID"`
	Group            Group           `json:"-" gorm:"foreignKey:GroupID"`

	// Virtual fields
	TotalVolume int `json:"total_volume" gorm:"-"`
	TradeCount  int `json:"trade_count" gorm:"-"`
}

// MarketOutcome represents one possible outcome in a market.
// Shares is the number of virtual shares in the CPMM liquidity pool.
// The product of all outcome shares is kept constant (invariant).
type MarketOutcome struct {
	ID       string  `json:"id" gorm:"primaryKey;type:text"`
	MarketID string  `json:"market_id" gorm:"index;type:text;not null"`
	Label    string  `json:"label" gorm:"type:text;not null"`
	Shares   float64 `json:"shares" gorm:"not null;default:100"` // CPMM pool shares

	// Virtual fields
	Price float64 `json:"price" gorm:"-"`
}

// SharePosition tracks how many shares a user holds for a specific outcome.
type SharePosition struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	MarketID  string    `json:"market_id" gorm:"uniqueIndex:idx_market_user_outcome;type:text;not null"`
	UserID    string    `json:"user_id" gorm:"uniqueIndex:idx_market_user_outcome;type:text;not null"`
	OutcomeID string    `json:"outcome_id" gorm:"uniqueIndex:idx_market_user_outcome;type:text;not null"`
	Shares    float64   `json:"shares" gorm:"not null;default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      User      `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Outcome   MarketOutcome `json:"outcome,omitempty" gorm:"foreignKey:OutcomeID"`
}

// Trade records a buy or sell transaction.
type Trade struct {
	ID         string    `json:"id" gorm:"primaryKey;type:text"`
	MarketID   string    `json:"market_id" gorm:"index;type:text;not null"`
	UserID     string    `json:"user_id" gorm:"index;type:text;not null"`
	OutcomeID  string    `json:"outcome_id" gorm:"type:text;not null"`
	Side       string    `json:"side" gorm:"type:text;not null"` // "buy" or "sell"
	Shares     float64   `json:"shares" gorm:"not null"`
	PointsCost int       `json:"points_cost" gorm:"not null"` // Positive for buys, negative for sells
	Price      float64   `json:"price" gorm:"not null"`       // Price per share at time of trade
	CreatedAt  time.Time `json:"created_at"`
	User       User      `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Outcome    MarketOutcome `json:"outcome,omitempty" gorm:"foreignKey:OutcomeID"`
}
