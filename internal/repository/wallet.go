package repository

import (
	"context"
)

type WalletRepository interface {
	ProcessTransaction(ctx context.Context, walletID string, amount int64, isDeposit bool) error
	GetBalance(ctx context.Context, walletID string) (int64, error)
	CreateWallet(ctx context.Context) (string, error)
}
