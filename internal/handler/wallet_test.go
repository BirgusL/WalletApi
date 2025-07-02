package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"WalletApi/internal/handler"
	"WalletApi/internal/model"
)

type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) CreateWallet(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockWalletService) ProcessTransaction(ctx context.Context, t model.Transaction) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockWalletService) GetBalance(ctx context.Context, walletID string) (int64, error) {
	args := m.Called(ctx, walletID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockWalletService) Shutdown() {
	m.Called()
}

func TestWalletHandler_CreateWallet_Success(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	mockService.On("CreateWallet", mock.Anything).Return(testUUID, nil)

	handler := handler.NewWalletHandler(mockService)

	req := httptest.NewRequest("POST", "/api/v1/wallets", nil)
	w := httptest.NewRecorder()

	handler.CreateWallet(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	data := responseBody["data"].(map[string]interface{})
	assert.Equal(t, testUUID, data["walletId"])
}

func TestWalletHandler_CreateWallet_ServiceError(t *testing.T) {
	mockService := new(MockWalletService)
	mockService.On("CreateWallet", mock.Anything).Return("", errors.New("db error"))

	handler := handler.NewWalletHandler(mockService)

	req := httptest.NewRequest("POST", "/api/v1/wallets", nil)
	w := httptest.NewRecorder()

	handler.CreateWallet(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	errorData := responseBody["error"].(map[string]interface{})
	assert.Equal(t, "Failed to create wallet", errorData["message"])
}

func TestWalletHandler_HandleTransaction_Success(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	mockService.On("ProcessTransaction", mock.Anything, mock.Anything).Return(nil)

	handler := handler.NewWalletHandler(mockService)

	transaction := model.Transaction{
		OperationType: model.Deposit,
		Amount:        100,
	}
	body, _ := json.Marshal(transaction)

	url := "/api/v1/wallets/" + testUUID + "/transactions"
	req := httptest.NewRequest("POST", url, bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleTransaction(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	data := responseBody["data"].(map[string]interface{})
	assert.Equal(t, "completed", data["status"])
	assert.Equal(t, testUUID, data["walletId"])
	assert.Equal(t, "DEPOSIT", data["operation"])
	assert.Equal(t, "100", data["amount"])
}

func TestWalletHandler_HandleTransaction_InvalidUUID(t *testing.T) {
	mockService := new(MockWalletService)
	handler := handler.NewWalletHandler(mockService)

	transaction := model.Transaction{
		OperationType: model.Deposit,
		Amount:        100,
	}
	body, _ := json.Marshal(transaction)

	url := "/api/v1/wallets/invalid-uuid/transactions"
	req := httptest.NewRequest("POST", url, bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleTransaction(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	errorData := responseBody["error"].(map[string]interface{})
	assert.Equal(t, "Invalid wallet ID format", errorData["message"])
}

func TestWalletHandler_HandleTransaction_InvalidJSON(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	handler := handler.NewWalletHandler(mockService)

	body := []byte(`{"operationType": "DEPOSIT", "amount": "should_be_number"}`)
	url := "/api/v1/wallets/" + testUUID + "/transactions"
	req := httptest.NewRequest("POST", url, bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleTransaction(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	errorData := responseBody["error"].(map[string]interface{})
	assert.Equal(t, "Invalid JSON format", errorData["message"])
}

func TestWalletHandler_HandleTransaction_ValidationErrors(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	handler := handler.NewWalletHandler(mockService)

	testCases := []struct {
		name        string
		body        string
		expectedMsg string
	}{
		{
			name:        "Invalid operation type",
			body:        `{"operationType": "INVALID", "amount": 100}`,
			expectedMsg: "Invalid operation type",
		},
		{
			name:        "Negative amount",
			body:        `{"operationType": "DEPOSIT", "amount": -100}`,
			expectedMsg: "Amount must be positive",
		},
		{
			name:        "Zero amount",
			body:        `{"operationType": "DEPOSIT", "amount": 0}`,
			expectedMsg: "Amount must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/wallets/" + testUUID + "/transactions"
			req := httptest.NewRequest("POST", url, strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			handler.HandleTransaction(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var responseBody map[string]interface{}
			err := json.NewDecoder(resp.Body).Decode(&responseBody)
			assert.NoError(t, err)

			errorData := responseBody["error"].(map[string]interface{})
			assert.Equal(t, tc.expectedMsg, errorData["message"])
		})
	}
}

func TestWalletHandler_HandleTransaction_ServiceErrors(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	handler := handler.NewWalletHandler(mockService)

	testCases := []struct {
		name         string
		serviceError error
		expectedCode int
		expectedMsg  string
	}{
		{
			name:         "Wallet not found",
			serviceError: model.ErrWalletNotFound,
			expectedCode: http.StatusNotFound,
			expectedMsg:  "Wallet not found",
		},
		{
			name:         "Insufficient funds",
			serviceError: model.ErrInsufficientFunds,
			expectedCode: http.StatusConflict,
			expectedMsg:  "Insufficient funds",
		},
		{
			name:         "Invalid amount",
			serviceError: model.ErrInvalidAmount,
			expectedCode: http.StatusBadRequest,
			expectedMsg:  "Invalid amount",
		},
		{
			name:         "Other error",
			serviceError: errors.New("database error"),
			expectedCode: http.StatusInternalServerError,
			expectedMsg:  "Transaction failed: database error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			mockService.On("ProcessTransaction", mock.Anything, mock.Anything).Return(tc.serviceError)

			transaction := model.Transaction{
				OperationType: model.Deposit,
				Amount:        100,
			}
			body, _ := json.Marshal(transaction)

			url := "/api/v1/wallets/" + testUUID + "/transactions"
			req := httptest.NewRequest("POST", url, bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.HandleTransaction(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedCode, resp.StatusCode)

			var responseBody map[string]interface{}
			err := json.NewDecoder(resp.Body).Decode(&responseBody)
			assert.NoError(t, err)

			errorData := responseBody["error"].(map[string]interface{})
			assert.Equal(t, tc.expectedMsg, errorData["message"])
		})
	}
}

func TestWalletHandler_HandleGetBalance_Success(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	mockService.On("GetBalance", mock.Anything, testUUID).Return(int64(150), nil)

	handler := handler.NewWalletHandler(mockService)

	url := "/api/v1/wallets/" + testUUID
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	handler.HandleGetBalance(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	data := responseBody["data"].(map[string]interface{})
	assert.Equal(t, float64(150), data["balance"])
}

func TestWalletHandler_HandleGetBalance_WalletNotFound(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	mockService.On("GetBalance", mock.Anything, testUUID).Return(int64(0), model.ErrWalletNotFound)

	handler := handler.NewWalletHandler(mockService)

	url := "/api/v1/wallets/" + testUUID
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	handler.HandleGetBalance(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	errorData := responseBody["error"].(map[string]interface{})
	assert.Equal(t, "Wallet not found", errorData["message"])
}

func TestWalletHandler_HandleGetBalance_ServiceError(t *testing.T) {
	testUUID := uuid.NewString()
	mockService := new(MockWalletService)
	mockService.On("GetBalance", mock.Anything, testUUID).Return(int64(0), errors.New("db error"))

	handler := handler.NewWalletHandler(mockService)

	url := "/api/v1/wallets/" + testUUID
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	handler.HandleGetBalance(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var responseBody map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)

	errorData := responseBody["error"].(map[string]interface{})
	assert.Equal(t, "Failed to get balance", errorData["message"])
}
