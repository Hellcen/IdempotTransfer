package service

import (
	"context"
	"idempot/internal/domain"
	"idempot/internal/port"
	"time"

	"github.com/google/uuid"
)

type withdrawalService struct {
	withdrawalRepo port.WithdrawalRepository
	balanceRepo    port.BalanceRepository
}

func NewWithdrawalService(
	withdrawalRepo port.WithdrawalRepository,
	balanceRepo port.BalanceRepository,
) port.WithdrawalService {
	return &withdrawalService{
		withdrawalRepo: withdrawalRepo,
		balanceRepo:    balanceRepo,
	}
}

func (s *withdrawalService) CreateWithdrawal(ctx context.Context, req *domain.WithdrawalReq) (*domain.Withdrawal, error) {
    // Сначала проверяем idempotency key без транзакции для производительности
    existing, err := s.withdrawalRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
    if err != nil {
        return nil, err
    }
    
    if existing != nil {
        // Verify payload matches
        if existing.UserID != req.UserID || existing.Amount != req.Amount || 
           existing.Currency != req.Currency || existing.Destination != req.Destination {
            return nil, domain.ErrIdempotencyKeyMismatch
        }
        return existing, nil
    }

    var withdrawal *domain.Withdrawal
    
    
    err = s.balanceRepo.WithLock(ctx, req.UserID, func(txCtx context.Context) error {
        // Проверяем баланс внутри транзакции
        balance, err := s.balanceRepo.GetBalance(txCtx, req.UserID, req.Currency)
        if err != nil {
            return err
        }

        if balance.Amount < req.Amount {
            return domain.ErrInsufficientBalance
        }

        withdrawal = &domain.Withdrawal{
            ID:             uuid.New(),
            UserID:         req.UserID,
            Amount:         req.Amount,
            Currency:       req.Currency,
            Destination:    req.Destination,
            IdempotencyKey: req.IdempotencyKey,
            Status:         domain.StatusPending,
            CreatedAt:      time.Now(),
            UpdatedAt:      time.Now(),
        }

        if err := s.withdrawalRepo.Create(txCtx, withdrawal); err != nil {
            return err
        }

        if err := s.balanceRepo.UpdateBalance(txCtx, req.UserID, req.Currency, -req.Amount); err != nil {
            return err
        }

        return nil
    })

    if err != nil {
        return nil, err
    }

    return withdrawal, nil
}

func (s *withdrawalService) GetWithdrawal(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error) {
    return s.withdrawalRepo.GetByID(ctx, id)
}

func (s *withdrawalService) ConfirmWithdrawal(ctx context.Context, id uuid.UUID) error {
    withdrawal, err := s.withdrawalRepo.GetByID(ctx, id)
    if err != nil {
        return err
    }

    if withdrawal.Status != domain.StatusPending {
        return nil //Already processed
    }

    return s.withdrawalRepo.UpdateStatus(ctx, id, domain.StatusConfirmed)
}

