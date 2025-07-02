# Wallet API Service

A high-performance REST API for managing digital wallets and processing transactions, designed to handle high concurrency scenarios.

## Features
- Create new wallets
- Deposit/withdraw funds with transaction processing
- Retrieve wallet balances
- Concurrent-safe operations (handles 1000+ RPS per wallet)
- Dockerized deployment with PostgreSQL
- Comprehensive test coverage

## Technologies
- **Backend**: Golang 1.23
- **Database**: PostgreSQL 15
- **Containerization**: Docker + Docker Compose
- **Concurrency Model**: Sharded worker pools with serializable transactions

## Getting Started

### Prerequisites
- Docker 20.10+
- Docker Compose 2.20+

### Installation
1. Clone the repository:
```bash
git clone https://github.com/BirgusL/WalletApi.git
cd WalletApi
```

2. Build and launch containers:

```bash
docker-compose up --build
```
### The system will:

- Initialize PostgreSQL database

- Apply database migrations

- Start API server on port 8080

### Configuration
Edit config.env for environment variables:

```ini
DB_URL=postgres://walletuser:walletpass@db:5432/walletdb?sslmode=disable
DB_NAME=walletdb
DB_USER=walletuser
DB_PASSWORD=walletpass
```
## API Documentation
- Create Wallet
```http
POST /api/v1/wallets
```
power shell
```power shell
$wallet = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/wallets" -Method Post
$walletId = $wallet.data.walletId
Write-Host "Wallet created: $walletId"
```

Response:

```json
{
  "data": {
    "walletId": "c6e5b8d0-7e9a-4a1b-9c3d-2f0b1e4d5c7a"
  }
}
```
- Process Transaction
```http
POST /api/v1/wallets/{WALLET_UUID}/transactions
```
Request Body:
```json
{
  "operationType": "DEPOSIT",
  "amount": 1500
}
```
power shell
```power shell
$body = @{
    operationType = "DEPOSIT"
    amount = 1000
} | ConvertTo-Json -Compress

$response = Invoke-RestMethod `
    -Uri "http://localhost:8080/api/v1/wallets/$walletId/transactions" `
    -Method Post `
    -Body $body `
    -ContentType "application/json"

Write-Host "Result: $($response | ConvertTo-Json -Depth 5)"
```
Response:

```json
{
  "data": {
    "status": "completed",
    "walletId": "c6e5b8d0-7e9a-4a1b-9c3d-2f0b1e4d5c7a",
    "operation": "DEPOSIT",
    "amount": "1500"
  }
}
```
- Get Balance
```http
GET /api/v1/wallets/{WALLET_UUID}
```
power shell
```power shell
$balance = (Invoke-RestMethod -Uri "http://localhost:8080/api/v1/wallets/$walletId").data.balance
Write-Host "Current balance: $balance"
```
Response:

```json
{
  "data": {
    "balance": 2500
  }
}
```
## Testing
Run tests with:

```bash
docker-compose run app go test -v ./...
```
### Test coverage includes:

- Service layer unit tests

- Handler HTTP tests

- Concurrency scenarios

- Error handling cases

## Concurrency Handling
The system ensures data consistency under high load through:

- Sharded Processing: Transactions are routed to worker pools based on wallet ID hash

- Database Isolation: Serializable transaction level with FOR UPDATE row locking

- Queue Buffering: Channel-based queues absorb request spikes

- Connection Pooling: Optimized PostgreSQL connection reuse

## Technical Requirements Met
- REST API with wallet operations

- PostgreSQL data storage

- Docker containerization

- Concurrent request handling (1000+ RPS)

- Error handling for insufficient funds/validation

- Environment variable configuration

- Comprehensive test coverage
