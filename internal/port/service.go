package port

import (
    "context"
    "github.com/google/uuid"
    "idempot/internal/domain"
)

type WithdrawalService interface {
    CreateWithdrawal(ctx context.Context, req *domain.WithdrawalReq) (*domain.Withdrawal, error)
    GetWithdrawal(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error)
    ConfirmWithdrawal(ctx context.Context, id uuid.UUID) error
}