package domain

import "errors"

var (
	ErrDuplicateRequest       = errors.New("duplicate request")
	ErrWithdrawalNotFound     = errors.New("withdrawal not found")
	ErrInsufficientBalance    = errors.New("insufficient balance")
	ErrIdempotencyKeyMismatch = errors.New("idempotency key mismatch")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrLockTimeout            = errors.New("lock timeout")
)
