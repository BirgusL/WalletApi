package model

import (
	"errors"
)

var (
	ErrWalletNotFound    = errors.New("wallet not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInvalidOperation  = errors.New("invalid operation type")
)

type OperationType string

const (
	Deposit  OperationType = "DEPOSIT"
	Withdraw OperationType = "WITHDRAW"
)

type Transaction struct {
	WalletID      string        `json:"walletId"`
	OperationType OperationType `json:"operationType"`
	Amount        int64         `json:"amount"`
}
