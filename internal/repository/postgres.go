package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"WalletApi/internal/model"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateWallet(ctx context.Context) (string, error) {
	var walletID string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO wallets (balance) VALUES (0) RETURNING id::text`).Scan(&walletID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", model.ErrWalletNotFound
		}
		return "", fmt.Errorf("failed to create wallet: %v", err)
	}

	return walletID, nil
}

func (r *PostgresRepository) ProcessTransaction(ctx context.Context, walletID string, amount int64, isDeposit bool) error {
	// Validation of the amount
	if amount <= 0 {
		return model.ErrInvalidAmount
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Checking the wallet's existence
	var exists bool
	err = tx.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM wallets WHERE id = $1)",
		walletID,
	).Scan(&exists)

	if err != nil {
		return fmt.Errorf("wallet existence check failed: %w", err)
	}
	if !exists {
		return model.ErrWalletNotFound
	}

	// 2. Getting the current balance with the lock
	var balance int64
	err = tx.QueryRowContext(ctx,
		"SELECT balance FROM wallets WHERE id = $1 FOR UPDATE",
		walletID,
	).Scan(&balance)

	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// 3. We check whether there are enough funds to debit
	if !isDeposit && balance < amount {
		return model.ErrInsufficientFunds
	}

	// 4. Calculating the new balance
	var newBalance int64
	if isDeposit {
		newBalance = balance + amount
	} else {
		newBalance = balance - amount
	}

	// 5. Updating the balance
	_, err = tx.ExecContext(ctx,
		"UPDATE wallets SET balance = $1 WHERE id = $2",
		newBalance,
		walletID,
	)
	if err != nil {
		return fmt.Errorf("balance update failed: %w", err)
	}

	// 6. Fixing the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaction commit failed: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetBalance(ctx context.Context, walletID string) (int64, error) {
	var balance int64
	err := r.db.QueryRowContext(ctx,
		"SELECT balance FROM wallets WHERE id = $1",
		walletID,
	).Scan(&balance)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, model.ErrWalletNotFound
		}
		return 0, err
	}
	return balance, nil
}

func (r *PostgresRepository) RunMigrations(ctx context.Context) error {
	// Getting the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Creating the absolute path to the migration file
	migrationPath := filepath.Join(wd, "migrations", "001_init.sql")

	// Reading the migration file
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file at %s: %w", migrationPath, err)
	}

	// Migrating
	if _, err := r.db.ExecContext(ctx, string(migration)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	return nil
}
