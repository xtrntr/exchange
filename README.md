# Trading Exchange Learning Project

A simple trading exchange built with Go and PostgreSQL for learning purposes. This project implements a basic exchange with user authentication, order placement, order book management, and a simple matching engine.

## Features

- **User Management**: Register and login with JWT authentication
- **Order Placement**: Place buy/sell limit orders for a single trading pair (BTC/USD)
- **Order Book**: In-memory order book sorted by price-time priority
- **Matching Engine**: Match orders based on price-time priority
- **Trade History**: Record and query executed trades

## Prerequisites

- Go 1.21+ (https://go.dev/dl/)
- Docker and Docker Compose (https://docs.docker.com/get-docker/)
- Git (https://git-scm.com/downloads)

## Project Structure

```
exchange/
├── cmd/server/               # Application entry point
├── internal/                 # Internal packages
│   ├── api/                  # HTTP handlers
│   ├── auth/                 # Authentication logic
│   ├── db/                   # Database connection and queries
│   ├── models/               # Data structures
│   └── exchange/             # Order book and matching engine
├── migrations/               # SQL migrations
├── docker-compose.yml        # Docker configuration
└── README.md                 # This file
```

## Setup Instructions

1. **Clone the repository**:
   ```bash
   git clone https://github.com/yourusername/exchange.git
   cd exchange
   ```

2. **Start PostgreSQL via Docker**:
   ```bash
   docker-compose up -d
   ```

3. **Apply database migrations**:
   ```bash
   # Find container ID
   docker ps
   
   # Apply migration (replace CONTAINER_ID with your actual container ID)
   docker exec -i CONTAINER_ID psql -U exchange_user -d exchange_db < migrations/001_init.sql
   ```

4. **Build and run the server**:
   ```bash
   go mod tidy  # Download dependencies
   go run cmd/server/main.go
   ```

The server will start at http://localhost:8080.

## API Usage

Test the API with curl:

### 1. Register a user

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}'
```

### 2. Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}'
```

Save the token from the response for subsequent requests.

### 3. Place a sell order

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -d '{"type":"sell","price":50000.00,"quantity":0.1}'
```

### 4. Place a buy order

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -d '{"type":"buy","price":51000.00,"quantity":0.05}'
```

### 5. View order book

```bash
curl -X GET http://localhost:8080/orderbook \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

### 6. View your orders

```bash
curl -X GET http://localhost:8080/orders \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

### 7. View your trades

```bash
curl -X GET http://localhost:8080/trades \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

## Next Steps for Learning

After completing this project, consider extending it with:

1. **Concurrency**: Make the matching engine concurrent with goroutines
2. **WebSocket**: Add real-time order book updates
3. **Multiple Trading Pairs**: Support multiple asset pairs
4. **Market Orders**: Implement market orders (execute at best available price)
5. **Unit Tests**: Add comprehensive test coverage
6. **Frontend**: Create a simple web interface
7. **Persistence**: Store the order book in the database for durability
8. **Authentication**: Add more advanced security features (rate limiting, refresh tokens)

## Notes

- JWT secret is hardcoded for simplicity. In production, use environment variables.
- The matching engine is synchronous. For a production system, consider a concurrent approach.
- Floating-point arithmetic is used for price/quantity. In production, use a decimal library.

## License

MIT 
