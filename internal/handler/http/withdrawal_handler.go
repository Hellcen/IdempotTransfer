package http

import (
	"idempot/internal/port"
	"log"

	"github.com/go-playground/validator/v10"
)


type WithdrawalHandler struct {
    service   port.WithdrawalService
    validate  *validator.Validate
    authToken string
    logger    *log.Logger
}

func NewWithdrawalHandler(service port.WithdrawalService, authToken string) *WithdrawalHandler {
    return &WithdrawalHandler{
        service:   service,
        validate:  validator.New(),
        authToken: authToken,
        logger:    log.Default(),
    }
}