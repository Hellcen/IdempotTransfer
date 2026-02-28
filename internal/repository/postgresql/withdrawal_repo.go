package postgresql

import (
	"context"
	"database/sql"
	"idempot/internal/domain"
	"idempot/internal/port"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type ctxtype string

const (
	trKey ctxtype = "tx"
)

var (
	uniqueConstraint   pq.ErrorCode = "23505"
	lockNotAvailable   pq.ErrorCode = "55P03"
)

type withdrawalRepository struct {
	db *sql.DB
}

type balanceRepository struct {
	db *sql.DB
}

func NewWithdrawalRepository(db *sql.DB) port.WithdrawalRepository {
	return &withdrawalRepository{db: db}
}

func NewBalanceRepository(db *sql.DB) port.BalanceRepository {
	return &balanceRepository{db: db}
}

func getTr(ctx context.Context) (*sql.Tx, bool) {
	tr, ok := ctx.Value(trKey).(*sql.Tx)
	return tr, ok
}

func (wr *withdrawalRepository) Create(ctx context.Context, w *domain.Withdrawal) error {
	const query = `INSERT INTO withdrawals (id, user_id, amount, currency, destination, idempotency_key, status, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	tr, ok := getTr(ctx)

	var err error
	if ok {
		_, err = tr.ExecContext(ctx, query, w.ID, w.UserID, w.Amount, w.Currency, w.Destination, w.IdempotencyKey, w.Status, w.CreatedAt, w.UpdatedAt)
	} else {
		_, err = wr.db.ExecContext(ctx, query, w.ID, w.UserID, w.Amount, w.Currency, w.Destination, w.IdempotencyKey, w.Status, w.CreatedAt, w.UpdatedAt)
	}

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == uniqueConstraint {
			if pqErr.Constraint == "withdrawals_idempotency_key_key" {
				return domain.ErrDuplicateRequest
			}
		}
		return err
	}

	return nil
}

func (r *withdrawalRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error) {
	var w domain.Withdrawal
	const query = `SELECT id, user_id, amount, currency, destination, idempotency_key, status, created_at, updated_at 
              FROM withdrawals WHERE id = $1`

	tx, ok := getTr(ctx)
	var err error
	if ok {
		err = tx.QueryRowContext(ctx, query, id).Scan(
			&w.ID, &w.UserID, &w.Amount, &w.Currency, &w.Destination, &w.IdempotencyKey, &w.Status, &w.CreatedAt, &w.UpdatedAt,
		)
	} else {
		err = r.db.QueryRowContext(ctx, query, id).Scan(
			&w.ID, &w.UserID, &w.Amount, &w.Currency, &w.Destination, &w.IdempotencyKey, &w.Status, &w.CreatedAt, &w.UpdatedAt,
		)
	}

	if err == sql.ErrNoRows {
		return nil, domain.ErrWithdrawalNotFound
	}
	return &w, err
}

func (r *withdrawalRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Withdrawal, error) {
	var w domain.Withdrawal
	const query = `SELECT id, user_id, amount, currency, destination, idempotency_key, status, created_at, updated_at 
              FROM withdrawals WHERE idempotency_key = $1`

	tx, ok := getTr(ctx)
	var err error
	if ok {
		err = tx.QueryRowContext(ctx, query, key).Scan(
			&w.ID, &w.UserID, &w.Amount, &w.Currency, &w.Destination, &w.IdempotencyKey, &w.Status, &w.CreatedAt, &w.UpdatedAt,
		)
	} else {
		err = r.db.QueryRowContext(ctx, query, key).Scan(
			&w.ID, &w.UserID, &w.Amount, &w.Currency, &w.Destination, &w.IdempotencyKey, &w.Status, &w.CreatedAt, &w.UpdatedAt,
		)
	}

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &w, err
}

func (r *withdrawalRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WithdrawalStatus) error {
	const query = `UPDATE withdrawals SET status = $1, updated_at = $2 WHERE id = $3`

	tx, ok := getTr(ctx)
	var result sql.Result
	var err error
	if ok {
		result, err = tx.ExecContext(ctx, query, status, time.Now(), id)
	} else {
		result, err = r.db.ExecContext(ctx, query, status, time.Now(), id)
	}

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrWithdrawalNotFound
	}
	return nil
}

//--------------------Balance

func (r *balanceRepository) GetBalance(ctx context.Context, userID string, currency string) (*domain.Balance, error) {
	var balance domain.Balance
	const query = `SELECT user_id, amount, currency FROM balances WHERE user_id = $1 AND currency = $2`

	tr, ok := getTr(ctx)
	var err error
	if ok {
		err = tr.QueryRowContext(ctx, query, userID, currency).Scan(&balance.UserID, &balance.Amount, &balance.Currency)
	} else {
		err = r.db.QueryRowContext(ctx, query, userID, currency).Scan(&balance.UserID, &balance.Amount, &balance.Currency)
	}

	if err == sql.ErrNoRows {
		return &domain.Balance{UserID: userID, Amount: 0, Currency: currency}, nil
	}
	return &balance, err
}

func (r *balanceRepository) WithLock(ctx context.Context, userID string, fn func(ctx context.Context) error) error {
	// Serializable
	tr, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
		ReadOnly:  false,
	})
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tr.Rollback()
			panic(p)
		}
	}()

	// блокировка строки баланса пользователя
	_, err = tr.ExecContext(ctx, "SELECT id FROM balances WHERE user_id = $1 FOR UPDATE NOWAIT", userID)
	if err != nil {
		tr.Rollback()
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == lockNotAvailable {
			return domain.ErrLockTimeout
		}
		if err == sql.ErrNoRows {
			//Если нет записи, создаем
			_, err = tr.ExecContext(ctx,
				"INSERT INTO balances (user_id, currency, amount) VALUES ($1, 'USDT', 0) ON CONFLICT DO NOTHING",
				userID)
			if err != nil {
				return err
			}

			//Защита от идемпотентности(двойное списание)
			_, err = tr.ExecContext(ctx, "SELECT id FROM balances WHERE user_id = $1 FOR UPDATE NOWAIT", userID)
			if err != nil {
				tr.Rollback()
				if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == lockNotAvailable {
					return domain.ErrLockTimeout
				}
				return err
			}
		} else {
			return err
		}
	}

	txCtx := context.WithValue(ctx, trKey, tr)

	if err := fn(txCtx); err != nil {
		tr.Rollback()
		return err
	}

	if err := tr.Commit(); err != nil {
		return err
	}

	return nil
}

func (r *balanceRepository) UpdateBalance(ctx context.Context, userID string, currency string, amount float64) error {
    query := `
        INSERT INTO balances (user_id, currency, amount, updated_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (user_id, currency) DO UPDATE 
        SET amount = balances.amount + $3, updated_at = $4
    `
    
    tr, ok := getTr(ctx)
    var err error
    if ok {
        _, err = tr.ExecContext(ctx, query, userID, currency, amount, time.Now())
    } else {
        _, err = r.db.ExecContext(ctx, query, userID, currency, amount, time.Now())
    }
    return err
}
