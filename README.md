# Trading Exchange Learning Project

A simple trading exchange built with Go and PostgreSQL for learning purposes. This project implements a basic exchange with user authentication, order placement, order book management, and a simple matching engine.

## Features

- **User Management**: Register and login with JWT authentication
- **Order Placement**: Place buy/sell limit orders for a single trading pair (BTC/USD)
- **Order Book**: In-memory order book sorted by price-time priority
- **Matching Engine**: Match orders based on price-time priority
- **Trade History**: Record and query executed trades
- **Real-time Updates**: WebSocket-based order book updates
- **Interactive UI**: TradingView Lightweight Charts integration

## Prerequisites

- Go 1.21+ (https://go.dev/dl/)
- Docker and Docker Compose (https://docs.docker.com/get-docker/)
- Git (https://git-scm.com/downloads)
- Modern web browser with WebSocket support

## Project Structure

```
exchange/
├── cmd/server/               # Application entry point
├── frontend/                 # Web interface
│   ├── index.html           # Main HTML with TradingView integration
│   ├── styles.css           # Dark theme styling
│   └── script.js            # Chart and WebSocket handling
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
   git clone https://github.com/xtrntr/exchange.git
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

5. **Access the Web Interface**:
   - Open `http://localhost:8080` in your browser
   - Login with the test credentials:
     ```
     Username: testuser
     Password: testpass
     ```
   - The interface includes:
     - Real-time order book chart
     - Order placement form
     - Your orders list with cancel functionality

The server will start at http://localhost:8080.

## WebSocket API

The application provides real-time order book updates via WebSocket:

### Connection
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
```

### Message Format
The server broadcasts order book updates every 5 seconds:
```json
{
  "buy_orders": [
    {"id": 1, "price": 50000.00, "quantity": 0.1, "type": "buy"}
  ],
  "sell_orders": [
    {"id": 2, "price": 51000.00, "quantity": 0.05, "type": "sell"}
  ]
}
```

### Chart Integration
The frontend uses TradingView Lightweight Charts to visualize the order book:
- Candlestick chart showing current price action
- Real-time updates from WebSocket data
- Dark theme matching the UI
- Responsive design

## Running Tests

The project includes comprehensive integration tests that verify functionality using a real PostgreSQL database.

### Prerequisites

1. **PostgreSQL Running**: Ensure the PostgreSQL container is running:
   ```bash
   docker-compose up -d
   ```

2. **Install gotestsum** (optional, for enhanced test output):
   ```bash
   go install gotest.tools/gotestsum@latest
   ```

### Running Tests

1. **Run all tests with gotestsum**:
   ```bash
   gotestsum --format testname
   ```

2. **Run all tests with standard Go test**:
   ```bash
   go test ./...
   ```

3. **Run tests for specific packages**:
   ```bash
   go test ./internal/auth
   go test ./internal/db
   go test ./internal/exchange
   go test ./internal/api
   ```

4. **Run tests with coverage**:
   ```bash
   go test -coverprofile=coverage.out ./...
   go tool cover -html=coverage.out  # View coverage in browser
   ```

### Test Cases

The test suite covers:

1. **Authentication (`auth_test.go`)**:
   - User registration with password hashing
   - Login with JWT generation
   - Token validation and expiration
   - Edge cases (duplicate users, invalid credentials)

2. **Database Operations (`db_test.go`)**:
   - Order creation and validation
   - Order cancellation (including concurrent cancellations)
   - User order retrieval
   - Trade recording

3. **Exchange Logic (`exchange_test.go`)**:
   - Order book management
   - Price-time priority ordering
   - Order matching and execution
   - Partial fills and order removal

4. **HTTP Handlers (`handlers_test.go`)**:
   - Order placement and validation
   - Order cancellation
   - Order book viewing
   - Authentication middleware
   - Error handling

### Notes

- Tests require a running PostgreSQL instance (via Docker)
- Database is reset between test runs for isolation
- JWT secret is hardcoded for testing
- Some tests verify concurrent operations
- Each test file includes its own TestMain for setup

## API Usage

Test the API with curl:

### Test Credentials
For testing purposes, you can use these credentials:
```
Username: testuser
Password: testpass
```

### 1. Register a user

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}'
```

### 2. Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}'
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
