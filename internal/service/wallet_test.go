package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"WalletApi/internal/model"
	"WalletApi/internal/service"
)

// MockWalletRepository implements repository.WalletRepository
type MockWalletRepository struct {
	mock.Mock
}

func (m *MockWalletRepository) CreateWallet(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockWalletRepository) ProcessTransaction(ctx context.Context, walletID string, amount int64, isDeposit bool) error {
	args := m.Called(ctx, walletID, amount, isDeposit)
	return args.Error(0)
}

func (m *MockWalletRepository) GetBalance(ctx context.Context, walletID string) (int64, error) {
	args := m.Called(ctx, walletID)
	return args.Get(0).(int64), args.Error(1)
}

func TestWalletService_CreateWallet(t *testing.T) {
	testUUID := uuid.NewString()
	mockRepo := new(MockWalletRepository)
	mockRepo.On("CreateWallet", mock.Anything).Return(testUUID, nil)

	walletService := service.NewWalletService(mockRepo, 1)
	defer walletService.Shutdown()

	walletID, err := walletService.CreateWallet(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, testUUID, walletID)
	_, err = uuid.Parse(walletID)
	assert.NoError(t, err, "Returned ID is not a valid UUID")
}

func TestWalletService_GetBalance(t *testing.T) {
	testUUID := uuid.NewString()
	mockRepo := new(MockWalletRepository)
	mockRepo.On("GetBalance", mock.Anything, testUUID).Return(int64(100), nil)

	walletService := service.NewWalletService(mockRepo, 1)
	defer walletService.Shutdown()

	balance, err := walletService.GetBalance(context.Background(), testUUID)

	assert.NoError(t, err)
	assert.Equal(t, int64(100), balance)
}

func TestWalletService_ProcessTransaction_Success(t *testing.T) {
	testUUID := uuid.NewString()
	mockRepo := new(MockWalletRepository)
	mockRepo.On("ProcessTransaction", mock.Anything, testUUID, int64(100), true).Return(nil)

	walletService := service.NewWalletService(mockRepo, 1)
	defer walletService.Shutdown()

	transaction := model.Transaction{
		WalletID:      testUUID,
		OperationType: model.Deposit,
		Amount:        100,
	}

	err := walletService.ProcessTransaction(context.Background(), transaction)
	assert.NoError(t, err)
}

func TestWalletService_ProcessTransaction_ValidationError(t *testing.T) {
	walletService := service.NewWalletService(nil, 1)
	defer walletService.Shutdown()

	testCases := []struct {
		name        string
		transaction model.Transaction
		expectedErr error
	}{
		{
			name: "Invalid amount",
			transaction: model.Transaction{
				WalletID:      "wallet-1",
				OperationType: model.Deposit,
				Amount:        -50,
			},
			expectedErr: model.ErrInvalidAmount,
		},
		{
			name: "Invalid operation",
			transaction: model.Transaction{
				WalletID:      "wallet-1",
				OperationType: "INVALID_OPERATION",
				Amount:        100,
			},
			expectedErr: model.ErrInvalidOperation,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := walletService.ProcessTransaction(context.Background(), tc.transaction)
			assert.ErrorIs(t, err, tc.expectedErr)
		})
	}
}

func TestWalletService_Sharding(t *testing.T) {
	uuid1 := uuid.NewString()
	uuid2 := uuid.NewString()

	mockRepo := new(MockWalletRepository)
	mockRepo.On("ProcessTransaction", mock.Anything, uuid1, mock.Anything, true).Return(nil).Times(2)
	mockRepo.On("ProcessTransaction", mock.Anything, uuid2, mock.Anything, true).Return(nil).Once()

	walletService := service.NewWalletService(mockRepo, 2)
	defer walletService.Shutdown()

	transactions := []model.Transaction{
		{WalletID: uuid1, OperationType: model.Deposit, Amount: 100},
		{WalletID: uuid2, OperationType: model.Deposit, Amount: 200},
		{WalletID: uuid1, OperationType: model.Deposit, Amount: 50},
	}

	// Collecting errors through the channel
	errChan := make(chan error, len(transactions))
	var wg sync.WaitGroup

	for _, tx := range transactions {
		wg.Add(1)
		go func(t model.Transaction) {
			defer wg.Done()
			err := walletService.ProcessTransaction(context.Background(), t)
			errChan <- err
		}(tx)
	}

	wg.Wait()
	close(errChan)

	// Checking for errors
	for err := range errChan {
		assert.NoError(t, err)
	}

	mockRepo.AssertExpectations(t)
}

func TestWalletService_Shutdown(t *testing.T) {
	mockRepo := new(MockWalletRepository)
	walletService := service.NewWalletService(mockRepo, 2)

	done := make(chan struct{})
	go func() {
		walletService.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Shutdown timed out")
	}
}

func TestWalletService_WorkerProcessing(t *testing.T) {
	testUUID := uuid.NewString()
	processed := make(chan struct{})

	mockRepo := new(MockWalletRepository)
	mockRepo.On("ProcessTransaction", mock.Anything, testUUID, int64(100), true).
		Run(func(args mock.Arguments) {
			close(processed)
		}).
		Return(nil)

	walletService := service.NewWalletService(mockRepo, 1)
	defer walletService.Shutdown()

	err := walletService.ProcessTransaction(context.Background(), model.Transaction{
		WalletID:      testUUID,
		OperationType: model.Deposit,
		Amount:        100,
	})
	assert.NoError(t, err)

	select {
	case <-processed:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Transaction was not processed")
	}
}

func TestWalletService_RepositoryError(t *testing.T) {
	testUUID := uuid.NewString()
	expectedErr := errors.New("database error")

	mockRepo := new(MockWalletRepository)
	mockRepo.On("ProcessTransaction", mock.Anything, testUUID, int64(100), true).Return(expectedErr)

	walletService := service.NewWalletService(mockRepo, 1)
	defer walletService.Shutdown()

	err := walletService.ProcessTransaction(context.Background(), model.Transaction{
		WalletID:      testUUID,
		OperationType: model.Deposit,
		Amount:        100,
	})
	assert.ErrorIs(t, err, expectedErr)
}
