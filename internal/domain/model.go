package domain

import (
	"time"

	"github.com/google/uuid"
)

type WithdrawalStatus string

const (
	StatusPending   WithdrawalStatus = "pending"
	StatusConfirmed WithdrawalStatus = "confirmed"
	StatusFailed    WithdrawalStatus = "failed"
)

type WithdrawalReq struct {
	UserID         string  `json:"user_id" validate:"required"`
	Amount         float64 `json:"amount" validate:"gt=0"`
	Currency       string  `json:"currency" validate:"required"`
	Destination    string  `json:"destination" validate:"required"`
	IdempotencyKey string  `json:"idempotency_key" validate:"required"`
}

type Withdrawal struct {
	ID             uuid.UUID
	UserID         string
	Amount         float64
	Currency       string
	Destination    string
	IdempotencyKey string
	Status         WithdrawalStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Balance struct {
	UserID   string
	Amount   float64 //maybe string
	Currency string
}
