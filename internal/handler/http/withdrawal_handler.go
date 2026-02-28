package http

import (
	"encoding/json"
	"idempot/internal/domain"
	"idempot/internal/port"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
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

func (h *WithdrawalHandler) WithLogger(logger *log.Logger) *WithdrawalHandler {
    h.logger = logger
    return h
}

func (h *WithdrawalHandler) AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") {
            h.logger.Printf("Unauthorized access attempt from %s", r.RemoteAddr)
            h.respondError(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
            return
        }

        token := strings.TrimPrefix(auth, "Bearer ")
        if token != h.authToken {
            h.logger.Printf("Invalid token attempt from %s", r.RemoteAddr)
            h.respondError(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func (h *WithdrawalHandler) CreateWithdrawal(w http.ResponseWriter, r *http.Request) {
    var req domain.WithdrawalReq
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.logger.Printf("Invalid request body: %v", err)
        h.respondError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    if err := h.validate.Struct(req); err != nil {
        h.logger.Printf("Validation failed: %v", err)
        h.respondError(w, err.Error(), http.StatusBadRequest)
        return
    }

    h.logger.Printf("Creating withdrawal for user %s, amount %f %s", 
        req.UserID, req.Amount, req.Currency)

    withdrawal, err := h.service.CreateWithdrawal(r.Context(), &req)
    if err != nil {
        switch err {
        case domain.ErrInsufficientBalance:
            h.logger.Printf("Insufficient balance for user %s", req.UserID)
            h.respondError(w, err.Error(), http.StatusConflict)
        case domain.ErrIdempotencyKeyMismatch:
            h.logger.Printf("Idempotency key mismatch for key %s", req.IdempotencyKey)
            h.respondError(w, err.Error(), http.StatusUnprocessableEntity)
        case domain.ErrDuplicateRequest:
            h.logger.Printf("Duplicate request with key %s", req.IdempotencyKey)
            h.respondError(w, err.Error(), http.StatusConflict)
        case domain.ErrLockTimeout:
            h.logger.Printf("Lock timeout for user %s", req.UserID)
            h.respondError(w, "too many concurrent requests", http.StatusTooManyRequests)
        default:
            h.logger.Printf("Internal error creating withdrawal: %v", err)
            h.respondError(w, "internal server error", http.StatusInternalServerError)
        }
        return
    }

    h.logger.Printf("Withdrawal created successfully: %s", withdrawal.ID)
    h.respondJSON(w, withdrawal, http.StatusCreated)
}

func (h *WithdrawalHandler) GetWithdrawal(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "id")
    id, err := uuid.Parse(idStr)
    if err != nil {
        h.logger.Printf("Invalid withdrawal ID: %s", idStr)
        h.respondError(w, "invalid withdrawal id", http.StatusBadRequest)
        return
    }

    withdrawal, err := h.service.GetWithdrawal(r.Context(), id)
    if err != nil {
        if err == domain.ErrWithdrawalNotFound {
            h.logger.Printf("Withdrawal not found: %s", id)
            h.respondError(w, err.Error(), http.StatusNotFound)
        } else {
            h.logger.Printf("Error getting withdrawal %s: %v", id, err)
            h.respondError(w, "internal server error", http.StatusInternalServerError)
        }
        return
    }

    h.respondJSON(w, withdrawal, http.StatusOK)
}

func (h *WithdrawalHandler) ConfirmWithdrawal(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "id")
    id, err := uuid.Parse(idStr)
    if err != nil {
        h.logger.Printf("Invalid withdrawal ID for confirmation: %s", idStr)
        h.respondError(w, "invalid withdrawal id", http.StatusBadRequest)
        return
    }

    if err := h.service.ConfirmWithdrawal(r.Context(), id); err != nil {
        h.logger.Printf("Error confirming withdrawal %s: %v", id, err)
        h.respondError(w, "internal server error", http.StatusInternalServerError)
        return
    }

    h.logger.Printf("Withdrawal confirmed: %s", id)
    w.WriteHeader(http.StatusOK)
}

func (h *WithdrawalHandler) respondJSON(w http.ResponseWriter, data interface{}, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(data); err != nil {
        h.logger.Printf("Error encoding response: %v", err)
    }
}

func (h *WithdrawalHandler) respondError(w http.ResponseWriter, message string, status int) {
    h.respondJSON(w, map[string]string{"error": message}, status)
}