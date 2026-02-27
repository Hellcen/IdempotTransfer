package postgresql

import (
	"database/sql"
	"idempot/internal/port"
)

type ctxtype string

const ()

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