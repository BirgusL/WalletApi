package service

import (
	"context"
	"hash/fnv"
	"sync"

	"WalletApi/internal/model"
	"WalletApi/internal/repository"
)

// WalletService interface for working with wallets
type WalletService interface {
	CreateWallet(ctx context.Context) (string, error)
	ProcessTransaction(ctx context.Context, t model.Transaction) error
	GetBalance(ctx context.Context, walletID string) (int64, error)
	Shutdown()
}

type walletService struct {
	repo    repository.WalletRepository
	queues  []chan transactionRequest
	workers int
	wg      sync.WaitGroup
}

type transactionRequest struct {
	ctx    context.Context
	t      model.Transaction
	result chan error
}

// New WalletService creates a new implementation of WalletService
func NewWalletService(repo repository.WalletRepository, workers int) WalletService {
	queues := make([]chan transactionRequest, workers)
	for i := range queues {
		queues[i] = make(chan transactionRequest, 10000)
	}

	s := &walletService{
		repo:    repo,
		queues:  queues,
		workers: workers,
	}

	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.processTransactions(i)
	}

	return s
}

func (s *walletService) getShard(walletID string) int {
	h := fnv.New32a()
	h.Write([]byte(walletID))
	return int(h.Sum32()) % s.workers
}

func (s *walletService) ProcessTransaction(ctx context.Context, t model.Transaction) error {
	if t.Amount <= 0 {
		return model.ErrInvalidAmount
	}

	shard := s.getShard(t.WalletID)
	resultChan := make(chan error, 1)

	s.queues[shard] <- transactionRequest{
		ctx:    ctx,
		t:      t,
		result: resultChan,
	}

	return <-resultChan
}

func (s *walletService) GetBalance(ctx context.Context, walletID string) (int64, error) {
	return s.repo.GetBalance(ctx, walletID)
}

func (s *walletService) processTransactions(shardIndex int) {
	defer s.wg.Done()
	for req := range s.queues[shardIndex] {
		var err error
		switch req.t.OperationType {
		case model.Deposit:
			err = s.repo.ProcessTransaction(req.ctx, req.t.WalletID, req.t.Amount, true)
		case model.Withdraw:
			err = s.repo.ProcessTransaction(req.ctx, req.t.WalletID, req.t.Amount, false)
		default:
			err = model.ErrInvalidOperation
		}
		req.result <- err
	}
}

func (s *walletService) CreateWallet(ctx context.Context) (string, error) {
	return s.repo.CreateWallet(ctx)
}

func (s *walletService) Shutdown() {
	for i := range s.queues {
		close(s.queues[i])
	}
	s.wg.Wait()
}
