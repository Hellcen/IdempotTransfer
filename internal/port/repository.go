package port

import (
	"context"
	"idempot/internal/domain"

	"github.com/google/uuid"
)

type WithdrawalRepository interface {
	Create(ctx context.Context, w *domain.Withdrawal) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Withdrawal, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WithdrawalStatus) error
}

type BalanceRepository interface {
	GetBalance(ctx context.Context, userID string, currency string) (*domain.Balance, error)
	WithLock(ctx context.Context, userID string, fn func(ctx context.Context) error) error
	UpdateBalance(ctx context.Context, userID string, currency string, amount float64) error
}