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
	UserID         string
	Amount         float64
	Currency       string
	Destination    string
	IdempotencyKey string
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
