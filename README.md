## Order Processing System

The application includes a complete order processing system for cryptocurrency transactions

Expected user flow:
- User creating order to exchange BEP-20 (BSC Network) USDT for anything, it can be fiat currency, digital product, etc...
- We're creating unique deposit digital wallet (Secure HD(Hierarchical Deterministic) wallet generation using BIP32/BIP39) for user, if it does not have yet
- All keys are located in one HD wallet. We can easily backup them using seed phrase
- We are monitoring blockchain, to see transactions to this wallet and order amount validation
- When we see transaction to our deposit addresses, we know exactly, who did the transfer

### Features

- **Wallet Generation**: Secure HD wallet generation using BIP32/BIP39, Metamask-like
- **Sequential Wallet Indices**: Deterministic wallet generation with sequential indices per user
- **User-Specific Wallet Management**: Support for multiple users with isolated wallet sequences
- **Transaction Monitoring**: Real-time monitoring of blockchain transactions
- **Order Management**: Create and track orders with status updates
- **USDT Support**: Process BEP-20 USDT transactions on Binance Smart Chain

### How It Works

1. **Order Creation**:
   - User creates an order specifying the amount
   - System generates a unique wallet address for the user with a sequential index
   - Order is stored with 'pending' status

2. **Wallet Management**:
   - Each user has their own sequence of wallet indices
   - Wallets are generated deterministically using BIP32/BIP39
   - Indices are stored in the database for persistence across server restarts
   - The system ensures uniqueness of indices per user

3. **Transaction Monitoring**:
   - System subscribes to new blocks on the blockchain
   - For each block, it analyzes transactions to detect USDT transfers
   - When a transfer to a system-generated wallet is detected, the corresponding order is updated

4. **Order Completion**:
   - When sufficient funds are received, the order status is updated to 'completed'
   - Multiple orders can be processed for the same wallet address

### Configuration

The blockchain integration can be configured using environment variables:

```bash
# Blockchain RPC URL
RPC_URL=https://bsc-dataseed.binance.org/

# Wallet seed phrase (KEEP THIS SECURE!)
WALLET_SEED=your secure seed phrase here

# Database connection string
DATABASE_URL=postgres://user:password@pgpool:5432/exchange
```

### Database Schema

The system uses a PostgreSQL database with the following schema:

```sql
-- Orders table
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    wallet VARCHAR(255) NOT NULL,
    amount VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending'
);

CREATE INDEX idx_orders_wallet ON orders(wallet);
CREATE INDEX idx_orders_user_id ON orders(user_id);

-- Wallets table
CREATE TABLE wallets (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL UNIQUE,
    derivation_path VARCHAR(255) NOT NULL,
    wallet_index INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_wallets_address ON wallets(address);
CREATE INDEX idx_wallets_user_id ON wallets(user_id);
CREATE INDEX idx_wallets_index ON wallets(wallet_index);
ALTER TABLE wallets ADD CONSTRAINT unique_user_wallet_index UNIQUE (user_id, wallet_index);
```

### API Endpoints

#### Create Order

```
POST /create_order?user_id=USER_ID&amount=AMOUNT
```

**Response**:

```json
{
  "status": "success",
  "wallet": "0x123abc..."
}
```

#### Generate Wallet

```
POST /generate_wallet?user_id=USER_ID
```

**Response**:

```json
{
  "status": "success",
  "wallet": "0x123abc..."
}
```

#### Get User Orders

```
GET /orders/user?user_id=USER_ID
```

**Response**:

```json
[
  {
    "id": 1,
    "user_id": "user123",
    "wallet": "0x123abc...",
    "amount": "100.0",
    "status": "pending"
  },
  {
    "id": 2,
    "user_id": "user123",
    "wallet": "0x123abc...",
    "amount": "50.0",
    "status": "completed"
  }
]
```

#### Get User Wallets

```
GET /wallets/user?user_id=USER_ID
```

**Response**:

```json
[
  {
    "address": "0x123abc..."
  },
  {
    "address": "0x456def..."
  }
]
```

#### Get Wallet Transactions

```
GET /transactions/wallet?wallet=WALLET_ADDRESS
```

**Response**:

```json
[
  {
    "id": 1,
    "tx_hash": "0xabc123...",
    "wallet": "0x123abc...",
    "amount": "100.0",
    "block_number": 12345678,
    "confirmed": true,
    "created_at": "2023-03-15T12:34:56Z"
  }
]
```

### Future API Enhancements

In future updates, the wallet management API will be enhanced to include:

1. Wallet indices in responses
2. Creation timestamps for wallets
3. Additional wallet metadata
4. Pagination for large wallet collections

### Planned API Endpoints

The following API endpoints are planned for future implementation to support the enhanced wallet management system:

#### Get User Wallets (Planned)

```

### Database Setup

The application uses PostgreSQL for storing order data. The database is automatically set up when running with Docker:

1. PostgreSQL database runs on port 5432
2. PgAdmin web interface is available at <http://localhost:5050> (login: <admin@admin.com> / password: admin)
3. Database migrations are automatically applied on application startup

#### Manual Database Setup

If you're not using Docker, you'll need to set up PostgreSQL manually:

1. Install PostgreSQL on your system
2. Create a database named `exchange`
3. Set the `DATABASE_URL` environment variable:

   ```bash
   export DATABASE_URL="postgres://username:password@localhost:5432/exchange?sslmode=disable"
   ```

#### Database Migrations

The application uses golang-migrate for database migrations. Migrations are stored in the `migrations` directory and are automatically applied when the application starts.

To manually run migrations:

```bash
# Install golang-migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path ./migrations -database "${DATABASE_URL}" up
```

To create a new migration:

```bash
migrate create -ext sql -dir ./migrations -seq migration_name
```

### Transaction Tracking

The application tracks blockchain transactions for generated wallets:

1. When a user creates an order, they receive a unique wallet address.
2. The system monitors the blockchain for incoming transactions to these wallets.
3. When a transaction is detected, it's recorded in the database and monitored for confirmations.
4. Once a transaction has enough confirmations, it's marked as confirmed and processed.
5. During processing, the system updates the status of any pending orders associated with the wallet.

You can view transactions for a specific wallet using the API:

```
GET /transactions/wallet?wallet=0x123...
```

### Blockchain Monitoring

The application monitors the blockchain for incoming transactions using a polling approach:

1. The system polls for new blocks every 5 seconds
2. When new blocks are detected, each block is processed to find relevant transactions
3. For USDT transfers to system-generated wallets, transactions are recorded in the database
4. After a transaction receives the required number of confirmations, it's marked as confirmed
5. Confirmed transactions are processed to update order statuses

This approach ensures reliable transaction monitoring even when using public RPC endpoints that don't support WebSocket subscriptions.

# Crypto P2P Trading

This application is designed for visualizing the processing of p2p orders, allowing users to sell crypto for fiat currency. It provides a seamless experience for users looking to engage in p2p trading, ensuring secure and efficient transactions.

## Features

- Real-time candlestick chart visualization of p2p order processing
- Multiple cryptocurrency pair support (BTC/USDT, ETH/USDT, SOL/USDT, BNB/USDT, XRP/USDT) for selling crypto for fiat
- Real-time order processing speed metrics (orders per second)
- Simulated trading data generation
- Responsive design with dark theme
- Consistent trading pair order
- Real-time trading data visualization with WebSockets
- Candlestick charts for multiple trading pairs
- Order processing with high throughput (up to 3000 orders per second)
- Secure HD wallet generation with sequential indices and user-specific management
- Blockchain transaction monitoring and processing
- Database integration with PostgreSQL for order and transaction storage
- Automatic database migrations

## Tech Stack

- **Backend**: Go with Gorilla WebSocket for real-time data streaming
- **Frontend**: React and TradingView Lightweight Charts
- **Communication**: WebSockets for real-time updates, REST API for historical data

## Project Structure

```
crypto-p2p-trading-app/
├── backend/           # Go backend
├── frontend/          # React frontend
│   └── src/
│       ├── components/ # React components
│       └── services/   # API services
├── Dockerfile         # Multi-stage Docker build
└── docker-compose.yml # Docker Compose configuration
```

## Getting Started

### Using Docker (Recommended)

The easiest way to run the application is using Docker:

```bash
# Build and run with Docker
make docker

# Stop the Docker container
make docker-stop

# View Docker logs
make docker-logs
```

### Manual Setup

If you prefer to run the application without Docker:

1. Install dependencies:

   ```bash
   make install
   ```

2. Run the backend and frontend in separate terminals:

   ```bash
   # Terminal 1: Run backend
   make run-backend

   # Terminal 2: Run frontend
   make run-frontend
   ```

3. Open your browser and navigate to:
   - Frontend: <http://localhost:3000>
   - Backend API: <http://localhost:8080>

## Development

### Backend

The backend is written in Go and provides:

- REST API for trading pairs and historical data
- WebSocket endpoint for real-time updates
- Simulated candlestick data generation
- Order processing speed calculation

#### Code Quality

The project uses golangci-lint for static code analysis. To run the linter:

```bash
# Install golangci-lint
make lint-install

# Run linter
make lint

# Fix issues automatically
make lint-fix
```

### Frontend

The frontend is built with React and includes:

- TradingView Lightweight Charts for candlestick visualization of p2p order processing
- WebSocket connection for real-time updates
- Trading pair selection
- Price display with change indicators and order processing speed

## API Documentation

### Overview

The Crypto P2P Trading App API provides access to cryptocurrency trading pair data, historical candle data, and real-time updates via WebSocket.

### Base URL

```
http://localhost:8080
```

### REST API

#### Get Trading Pairs List

Returns a list of all available trading pairs with current prices, changes, and order processing speeds.

**URL**: `/api/pairs`

**Method**: `GET`

**Request Example**:

```bash
curl -X GET http://localhost:8080/api/pairs
```

**Successful Response**:

```json
[
  {
    "symbol": "BTCRUB",
    "lastPrice": 65000.0,
    "priceChange": 2.5,
    "ordersPerSecond": 2.1
  },
  {
    "symbol": "ETHRUB",
    "lastPrice": 3500.0,
    "priceChange": 1.2,
    "ordersPerSecond": 1.9
  },
  {
    "symbol": "SOLRUB",
    "lastPrice": 180.0,
    "priceChange": 3.7,
    "ordersPerSecond": 2.0
  },
  {
    "symbol": "BNBRUB",
    "lastPrice": 600.0,
    "priceChange": -0.5,
    "ordersPerSecond": 1.8
  },
  {
    "symbol": "XRPRUB",
    "lastPrice": 0.55,
    "priceChange": 0.8,
    "ordersPerSecond": 2.2
  }
]
```

**Response Codes**:

- `200 OK`: Successful request
- `500 Internal Server Error`: Server error

#### Get Candle Data

Returns historical candle data for the specified trading pair.

**URL**: `/api/candles/{symbol}`

**Method**: `GET`

**URL Parameters**:

- `{symbol}`: Trading pair symbol (e.g., BTCRUB)

**Request Example**:

```bash
curl -X GET http://localhost:8080/api/candles/BTCRUB
```

**Successful Response**:

```json
[
  {
    "time": 1677676800000,
    "open": 64500.0,
    "high": 65100.0,
    "low": 64400.0,
    "close": 65000.0,
    "volume": 100.5
  },
  {
    "time": 1677677100000,
    "open": 65000.0,
    "high": 65200.0,
    "low": 64900.0,
    "close": 65100.0,
    "volume": 90.2
  }
]
```

**Response Codes**:

- `200 OK`: Successful request
- `404 Not Found`: Trading pair not found
- `500 Internal Server Error`: Server error

### WebSocket API

#### WebSocket Connection

To receive real-time updates, the client must establish a WebSocket connection.

**URL**: `ws://localhost:8080/ws/{symbol}`

**URL Parameters**:

- `{symbol}`: Trading pair symbol (e.g., BTCRUB)

**Connection Example**:

```javascript
const socket = new WebSocket('ws://localhost:8080/ws/BTCRUB');

socket.onopen = () => {
  console.log('WebSocket connection established');
};

socket.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Data received:', data);
};

socket.onclose = () => {
  console.log('WebSocket connection closed');
};

socket.onerror = (error) => {
  console.error('WebSocket error:', error);
};
```

#### Message Format

The server sends updates in JSON format:

```json
{
  "symbol": "BTCRUB",
  "lastPrice": 65200.0,
  "priceChange": 0.3,
  "ordersPerSecond": 2.1,
  "lastCandle": {
    "time": 1677677400000,
    "open": 65100.0,
    "high": 65300.0,
    "low": 65000.0,
    "close": 65200.0,
    "volume": 110.7
  }
}
```

#### Error Handling

If an error occurs, the server may close the connection. The client should handle such situations and reconnect if necessary.

## Technical Documentation

### Application Architecture

The application is built on the principle of separation of concerns and consists of the following components:

#### Backend (Go)

The backend is divided into the following layers:

1. **Models** (`internal/models/`) - define data structures
2. **Services** (`internal/services/`) - contain business logic
3. **Handlers** (`internal/handlers/`) - process HTTP requests and WebSocket connections
4. **WebSocket** (`internal/websocket/`) - manage WebSocket connections

#### Frontend (React)

The frontend is organized as follows:

1. **Components** (`src/components/`) - UI components
2. **Services** (`src/services/`) - API interaction

### Detailed Backend Description

#### Data Models (`internal/models/`)

##### TradingPair

```go
type TradingPair struct {
    Symbol          string                   // Pair symbol (e.g., BTCRUB)
    LastPrice       float64                  // Last price
    PriceChange     float64                  // Price change percentage
    OrdersPerSecond float64                  // Orders processed per second
    CandleData      []CandleData             // Historical candle data
    LastCandle      CandleData               // Last candle
    Subscribers     map[*websocket.Conn]bool // WebSocket update subscribers
    Mutex           sync.RWMutex             // Mutex for safe data access
    StopChan        chan struct{}            // Channel for stopping goroutines
    OrderCount      int64                    // Total number of orders processed
    LastOrderTime   time.Time                // Time of the last order count reset
    OrderCountMutex sync.Mutex               // Mutex for order count operations
}
```

##### CandleData

```go
type CandleData struct {
    Time   int64   // Time in milliseconds
    Open   float64 // Opening price
    High   float64 // Highest price
    Low    float64 // Lowest price
    Close  float64 // Closing price
    Volume float64 // Trading volume
}
```

#### Services (`internal/services/`)

##### DataService

Responsible for generating and managing trading pair data:

- `InitializeTradingPairs()` - initializes trading pairs with initial data
- `GenerateInitialCandleData()` - generates historical candle data
- `SimulateTradingData()` - simulates real-time trading data
- `BroadcastUpdate()` - sends updates to all subscribers
- `GetCandleData()` - returns candle data for a pair
- `AddSubscriber()` / `RemoveSubscriber()` - manages subscribers
- `trackOrder()` - tracks order processing and calculates orders per second

#### WebSocket (`internal/websocket/`)

##### WebSocketManager

Manages WebSocket connections:

- `Upgrade()` - upgrades HTTP connection to WebSocket

#### Handlers (`internal/handlers/`)

##### HTTPHandler

Processes HTTP requests:

- `GetTradingPairsHandler()` - returns list of trading pairs
- `GetCandlesHandler()` - returns candle data for a trading pair

##### WebSocketHandler

Processes WebSocket connections:

- `HandleConnection()` - handles WebSocket connections for the specified trading pair

### WebSocket: Implementation Details

#### Connection Establishment

1. Client initiates WebSocket connection via URL `ws://localhost:8080/ws/{symbol}`
2. Server upgrades HTTP connection to WebSocket using `websocketManager.Upgrade()`
3. Client is added to the subscriber list for the specified trading pair

#### Message Format

##### From Server to Client

The server sends updates in JSON format:

```json
{
  "symbol": "BTCRUB",
  "lastPrice": 65200.0,
  "priceChange": 0.3,
  "ordersPerSecond": 2.1,
  "lastCandle": {
    "time": 1677677400000,
    "open": 65100.0,
    "high": 65300.0,
    "low": 65000.0,
    "close": 65200.0,
    "volume": 110.7
  }
}
```

### Order Processing Speed Calculation

The application tracks and calculates the order processing speed (orders per second) for each trading pair:

1. Each price update and candle creation is counted as an order
2. The order count is incremented for each order processed
3. Every second, the orders per second is calculated as: `orderCount / elapsedTime`
4. The order count and timer are reset after each calculation
5. The calculated orders per second is included in WebSocket updates and API responses

### Concurrency and Synchronization

#### Concurrent Access

- `sync.RWMutex` is used for safe access to trading pair data
- Data reading is protected using `RLock()` / `RUnlock()`
- Data writing is protected using `Lock()` / `Unlock()`
- `OrderCountMutex` is used for safe access to order count data

## License

MIT