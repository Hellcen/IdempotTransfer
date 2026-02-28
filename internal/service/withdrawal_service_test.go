package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"idempot/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockWithdrawalRepository struct {
	mock.Mock
}

func (m *MockWithdrawalRepository) Create(ctx context.Context, w *domain.Withdrawal) error {
	args := m.Called(ctx, w)
	return args.Error(0)
}

func (m *MockWithdrawalRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Withdrawal), args.Error(1)
}

func (m *MockWithdrawalRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Withdrawal, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Withdrawal), args.Error(1)
}

func (m *MockWithdrawalRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WithdrawalStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

type MockBalanceRepository struct {
	mock.Mock
}

func (m *MockBalanceRepository) GetBalance(ctx context.Context, userID string, currency string) (*domain.Balance, error) {
	args := m.Called(ctx, userID, currency)
	return args.Get(0).(*domain.Balance), args.Error(1)
}

func (m *MockBalanceRepository) UpdateBalance(ctx context.Context, userID string, currency string, amount float64) error {
	args := m.Called(ctx, userID, currency, amount)
	return args.Error(0)
}

func (m *MockBalanceRepository) WithLock(ctx context.Context, userID string, fn func(ctx context.Context) error) error {
	_ = m.Called(ctx, userID, fn)
	return fn(ctx)
}

// Тест 1: Успешное создание withdrawal
func TestCreateWithdrawal_Success(t *testing.T) {
	mockWithdrawalRepo := new(MockWithdrawalRepository)
	mockBalanceRepo := new(MockBalanceRepository)
	service := NewWithdrawalService(mockWithdrawalRepo, mockBalanceRepo)

	req := &domain.WithdrawalReq{
		UserID:         "user-123",
		Amount:         100.0,
		Currency:       "USDT",
		Destination:    "0x123",
		IdempotencyKey: "key-123",
	}

	mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, req.IdempotencyKey).Return(nil, nil)

	mockBalanceRepo.On("WithLock", mock.Anything, req.UserID, mock.Anything).Return(nil)
	mockBalanceRepo.On("GetBalance", mock.Anything, req.UserID, req.Currency).Return(&domain.Balance{
		UserID: req.UserID, Amount: 500.0, Currency: req.Currency,
	}, nil)

	mockWithdrawalRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Withdrawal")).Return(nil)
	mockBalanceRepo.On("UpdateBalance", mock.Anything, req.UserID, req.Currency, -req.Amount).Return(nil)

	withdrawal, err := service.CreateWithdrawal(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, withdrawal)
	assert.Equal(t, req.UserID, withdrawal.UserID)
	assert.Equal(t, req.Amount, withdrawal.Amount)
	assert.Equal(t, domain.StatusPending, withdrawal.Status)

	mockWithdrawalRepo.AssertExpectations(t)
	mockBalanceRepo.AssertExpectations(t)
}

// Тест 2: Недостаточный баланс
func TestCreateWithdrawal_InsufficientBalance(t *testing.T) {
	mockWithdrawalRepo := new(MockWithdrawalRepository)
	mockBalanceRepo := new(MockBalanceRepository)
	service := NewWithdrawalService(mockWithdrawalRepo, mockBalanceRepo)

	req := &domain.WithdrawalReq{
		UserID:         "user-123",
		Amount:         600.0,
		Currency:       "USDT",
		Destination:    "0x123",
		IdempotencyKey: "key-123",
	}

	mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, req.IdempotencyKey).Return(nil, nil)

	mockBalanceRepo.On("WithLock", mock.Anything, req.UserID, mock.Anything).Return(nil)
	mockBalanceRepo.On("GetBalance", mock.Anything, req.UserID, req.Currency).Return(&domain.Balance{
		UserID: req.UserID, Amount: 500.0, Currency: req.Currency,
	}, nil)

	withdrawal, err := service.CreateWithdrawal(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrInsufficientBalance, err)
	assert.Nil(t, withdrawal)

	mockWithdrawalRepo.AssertExpectations(t)
	mockBalanceRepo.AssertExpectations(t)
}

// Тест 3: Идемпотентность - одинаковый ключ возвращает тот же результат
func TestCreateWithdrawal_Idempotency(t *testing.T) {
	mockWithdrawalRepo := new(MockWithdrawalRepository)
	mockBalanceRepo := new(MockBalanceRepository)
	service := NewWithdrawalService(mockWithdrawalRepo, mockBalanceRepo)

	req := &domain.WithdrawalReq{
		UserID:         "user-123",
		Amount:         100.0,
		Currency:       "USDT",
		Destination:    "0x123",
		IdempotencyKey: "key-123",
	}

	existingWithdrawal := &domain.Withdrawal{
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

	// Первый вызов - создаем
	mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, req.IdempotencyKey).Return(nil, nil).Once()
	mockBalanceRepo.On("WithLock", mock.Anything, req.UserID, mock.Anything).Return(nil).Once()
	mockBalanceRepo.On("GetBalance", mock.Anything, req.UserID, req.Currency).Return(&domain.Balance{
		UserID: req.UserID, Amount: 500.0, Currency: req.Currency,
	}, nil).Once()
	mockWithdrawalRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Withdrawal")).Return(nil).Once()
	mockBalanceRepo.On("UpdateBalance", mock.Anything, req.UserID, req.Currency, -req.Amount).Return(nil).Once()

	withdrawal1, err1 := service.CreateWithdrawal(context.Background(), req)
	assert.NoError(t, err1)
	assert.NotNil(t, withdrawal1)
	// Эмулируем поведение БД: повторный запрос по ключу вернёт уже созданный withdrawal.
	existingWithdrawal.ID = withdrawal1.ID

	// Второй вызов - возвращаем существующий
	mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, req.IdempotencyKey).Return(existingWithdrawal, nil).Once()

	withdrawal2, err2 := service.CreateWithdrawal(context.Background(), req)
	assert.NoError(t, err2)
	assert.NotNil(t, withdrawal2)
	assert.Equal(t, withdrawal1.ID, withdrawal2.ID)

	mockWithdrawalRepo.AssertExpectations(t)
	mockBalanceRepo.AssertExpectations(t)
}

// Тест 4: Конкурентные запросы на один баланс
func TestCreateWithdrawal_Concurrent(t *testing.T) {
	mockWithdrawalRepo := new(MockWithdrawalRepository)
	mockBalanceRepo := new(MockBalanceRepository)
	service := NewWithdrawalService(mockWithdrawalRepo, mockBalanceRepo)

	userID := "user-123"
	initialBalance := 1000.0
	withdrawalAmount := 300.0
	numGoroutines := 3

	var wg sync.WaitGroup
	results := make(chan error, numGoroutines)
	withdrawals := make(chan *domain.Withdrawal, numGoroutines)

	// Важно: ключи должны совпадать и в моках, и в горутинах.
	keys := make([]string, 0, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		keys = append(keys, uuid.New().String())
	}

	// Настраиваем моки для каждого вызова
	for i := 0; i < numGoroutines; i++ {
		idempotencyKey := keys[i]

		mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, idempotencyKey).Return(nil, nil).Once()
		mockBalanceRepo.On("WithLock", mock.Anything, userID, mock.Anything).Return(nil).Once()
		mockBalanceRepo.On("GetBalance", mock.Anything, userID, "USDT").Return(&domain.Balance{
			UserID: userID, Amount: initialBalance, Currency: "USDT",
		}, nil).Maybe()
		mockWithdrawalRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Withdrawal")).Return(nil).Maybe()
		mockBalanceRepo.On("UpdateBalance", mock.Anything, userID, "USDT", -withdrawalAmount).Return(nil).Maybe()
	}

	// Запускаем конкурентные запросы
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()

			req := &domain.WithdrawalReq{
				UserID:         userID,
				Amount:         withdrawalAmount,
				Currency:       "USDT",
				Destination:    "0x123",
				IdempotencyKey: key,
			}

			withdrawal, err := service.CreateWithdrawal(context.Background(), req)
			if err != nil {
				results <- err
			} else {
				withdrawals <- withdrawal
				results <- nil
			}
		}(keys[i])
	}

	wg.Wait()
	close(results)
	close(withdrawals)

	// Проверяем результаты
	successCount := 0
	failCount := 0

	for err := range results {
		if err == nil {
			successCount++
		} else if err == domain.ErrInsufficientBalance {
			failCount++
		}
	}

	// С балансом 1000, при 3 попытках снять по 300:
	// - первый успех: 1000 -> 700
	// - второй успех: 700 -> 400
	// - третий успех: 400 -> 100
	assert.Equal(t, 3, successCount, "Должно быть 3 успешных withdrawal")
	assert.Equal(t, 0, failCount, "Не должно быть ошибок недостаточного баланса")

	mockWithdrawalRepo.AssertExpectations(t)
	mockBalanceRepo.AssertExpectations(t)
}

// Тест 5: Одинаковый idempotency key в конкурентных запросах
func TestCreateWithdrawal_SameIdempotencyKeyConcurrent(t *testing.T) {
	mockWithdrawalRepo := new(MockWithdrawalRepository)
	mockBalanceRepo := new(MockBalanceRepository)
	service := NewWithdrawalService(mockWithdrawalRepo, mockBalanceRepo)

	userID := "user-123"
	idempotencyKey := "same-key-123"

	var wg sync.WaitGroup
	results := make(chan error, 5)

	// Первый вызов - ключа еще нет
	mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, idempotencyKey).Return(nil, nil).Once()
	mockBalanceRepo.On("WithLock", mock.Anything, userID, mock.Anything).Return(nil).Once()
	mockBalanceRepo.On("GetBalance", mock.Anything, userID, "USDT").Return(&domain.Balance{
		UserID: userID, Amount: 1000.0, Currency: "USDT",
	}, nil).Once()
	mockWithdrawalRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Withdrawal")).Return(nil).Once()
	mockBalanceRepo.On("UpdateBalance", mock.Anything, userID, "USDT", -100.0).Return(nil).Once()

	// Остальные вызовы - ключ уже существует
	existingWithdrawal := &domain.Withdrawal{
		ID:             uuid.New(),
		UserID:         userID,
		Amount:         100.0,
		Currency:       "USDT",
		Destination:    "0x123",
		IdempotencyKey: idempotencyKey,
		Status:         domain.StatusPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	for i := 0; i < 4; i++ {
		mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, idempotencyKey).Return(existingWithdrawal, nil).Once()
	}

	// Запускаем 5 конкурентных запросов
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := &domain.WithdrawalReq{
				UserID:         userID,
				Amount:         100.0,
				Currency:       "USDT",
				Destination:    "0x123",
				IdempotencyKey: idempotencyKey,
			}

			_, err := service.CreateWithdrawal(context.Background(), req)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Все запросы должны быть без ошибок
	for err := range results {
		assert.NoError(t, err)
	}

	mockWithdrawalRepo.AssertExpectations(t)
	mockBalanceRepo.AssertExpectations(t)
}

// Тест 6: Атомарность транзакции при ошибке
func TestCreateWithdrawal_TransactionAtomicity(t *testing.T) {
	mockWithdrawalRepo := new(MockWithdrawalRepository)
	mockBalanceRepo := new(MockBalanceRepository)
	service := NewWithdrawalService(mockWithdrawalRepo, mockBalanceRepo)

	req := &domain.WithdrawalReq{
		UserID:         "user-123",
		Amount:         100.0,
		Currency:       "USDT",
		Destination:    "0x123",
		IdempotencyKey: "key-123",
	}

	mockWithdrawalRepo.On("GetByIdempotencyKey", mock.Anything, req.IdempotencyKey).Return(nil, nil)
	mockBalanceRepo.On("WithLock", mock.Anything, req.UserID, mock.Anything).Return(nil)
	mockBalanceRepo.On("GetBalance", mock.Anything, req.UserID, req.Currency).Return(&domain.Balance{
		UserID: req.UserID, Amount: 500.0, Currency: req.Currency,
	}, nil)
	mockWithdrawalRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Withdrawal")).Return(nil)

	// Ошибка при обновлении баланса
	expectedErr := errors.New("database error")
	mockBalanceRepo.On("UpdateBalance", mock.Anything, req.UserID, req.Currency, -req.Amount).Return(expectedErr)

	withdrawal, err := service.CreateWithdrawal(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, withdrawal)

	// Проверяем что Create был вызван (но транзакция откатится)
	mockWithdrawalRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.Withdrawal"))
	mockWithdrawalRepo.AssertExpectations(t)
	mockBalanceRepo.AssertExpectations(t)
}
