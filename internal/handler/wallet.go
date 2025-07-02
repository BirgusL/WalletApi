package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"WalletApi/internal/model"
	"WalletApi/internal/service"

	"github.com/google/uuid"
)

type WalletHandler struct {
	service service.WalletService
}

func NewWalletHandler(service service.WalletService) *WalletHandler {
	return &WalletHandler{service: service}
}

func (h *WalletHandler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	walletID, err := h.service.CreateWallet(r.Context())
	if err != nil {
		sendErrorResponse(w, "Failed to create wallet", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, map[string]string{"walletId": walletID})
}

func (h *WalletHandler) HandleTransaction(w http.ResponseWriter, r *http.Request) {
	// Extracting the walletID from the URL
	walletID := strings.TrimPrefix(r.URL.Path, "/api/v1/wallets/")
	walletID = strings.TrimSuffix(walletID, "/transactions")

	if _, err := uuid.Parse(walletID); err != nil {
		sendErrorResponse(w, "Invalid wallet ID format", http.StatusBadRequest)
		return
	}

	// Parsing the request body
	var t model.Transaction
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		sendErrorResponse(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Validation of fields
	if t.OperationType != model.Deposit && t.OperationType != model.Withdraw {
		sendErrorResponse(w, "Invalid operation type", http.StatusBadRequest)
		return
	}

	if t.Amount <= 0 {
		sendErrorResponse(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	// Setting the walletID from the URL
	t.WalletID = walletID

	// Processing the transaction
	if err := h.service.ProcessTransaction(r.Context(), t); err != nil {
		switch {
		case errors.Is(err, model.ErrWalletNotFound):
			sendErrorResponse(w, "Wallet not found", http.StatusNotFound)
		case errors.Is(err, model.ErrInsufficientFunds):
			sendErrorResponse(w, "Insufficient funds", http.StatusConflict)
		case errors.Is(err, model.ErrInvalidAmount):
			sendErrorResponse(w, "Invalid amount", http.StatusBadRequest)
		default:
			sendErrorResponse(w, "Transaction failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	sendSuccessResponse(w, map[string]string{
		"status":    "completed",
		"walletId":  walletID,
		"operation": string(t.OperationType),
		"amount":    strconv.FormatInt(t.Amount, 10),
	})
}

func (h *WalletHandler) HandleGetBalance(w http.ResponseWriter, r *http.Request) {
	walletID := strings.TrimPrefix(r.URL.Path, "/api/v1/wallets/")
	if _, err := uuid.Parse(walletID); err != nil {
		sendErrorResponse(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	balance, err := h.service.GetBalance(r.Context(), walletID)
	if err != nil {
		if errors.Is(err, model.ErrWalletNotFound) {
			sendErrorResponse(w, "Wallet not found", http.StatusNotFound)
		} else {
			sendErrorResponse(w, "Failed to get balance", http.StatusInternalServerError)
		}
		return
	}

	sendSuccessResponse(w, map[string]int64{"balance": balance})
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
		},
	})
}

func sendSuccessResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": data,
	})
}
