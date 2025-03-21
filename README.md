## Order Processing System

The application includes a complete order processing system for cryptocurrency transactions

## Expected user flow

- User creating order to exchange BEP-20 (BSC Network) USDT for anything, it can be fiat currency, digital product, etc...
- We're creating unique deposit digital wallet (Secure HD(Hierarchical Deterministic) wallet generation using BIP32/BIP39) for user, if it does not have yet
- All keys are located in one HD wallet. We can easily backup them using seed phrase
- We are monitoring blockchain, to see transactions to this wallet and order amount validation
- When we see transaction to our deposit addresses, we know exactly, who did the transfer

# Step-by-Step Manual to Test the User Flow

Here's a comprehensive guide to test your cryptocurrency order processing system with the enhanced wallet management features:

## Prerequisites

1. Make sure your application is running (either via Docker or manual setup)
2. Have access to a BSC wallet with some BEP-20 USDT for testing (can use Metamask with BSC Testnet)
3. Have a tool like Postman or curl for making API requests

## Testing Flow on TestNet

# Step-by-Step Plan to Test Your BSC Testnet Wallet

## 1. Get Test BNB from a BSC Testnet Faucet

1. Go to the BNB Smart Chain Testnet Faucet: https://testnet.bnbchain.org/faucet-smart
2. Enter your wallet address: `0x`
3. Solve the captcha and click "Give me BNB"
4. The faucet will send you some test BNB (usually 0.1 to 1 BNB)

Alternative faucets if the official one isn't working:
- https://faucet.quicknode.com/bnb-chain/bnb-testnet
- https://testnet.binance.org/faucet-smart

## 2. Check Your Wallet Balance

1. Check your wallet on the BSC Testnet Explorer:
   - Go to https://testnet.bscscan.com/
   - Enter your wallet address `0x` in the search bar
   - You should see your balance and any transactions

2. Create an endpoint in your app to check balance (optional):
   ```bash
   # Run this command to check the balance
   curl "http://localhost:8080/transactions/wallet?wallet=0x6C83946e958Db8A7F8a49981A412E5f59be27E3B"
   ```

## 3. Send Test BNB to Another Wallet

1. Create a second wallet for testing:
   ```bash
   curl -X POST "http://localhost:8080/generate_wallet?user_id=456"
   ```
   (Save the returned wallet address)

2. Send some BNB from your first wallet to the second wallet:
   ```bash
   curl -X POST "http://localhost:8080/transfer" \
   -H "Content-Type: application/json" \
   -d '{
     "from_wallet_id": 17,
     "to_address": "ADDRESS_OF_SECOND_WALLET",
     "amount": "0.01",
     "priority": "medium"
   }'
   ```

3. Check both wallets to confirm the transaction:
   ```bash
   curl "http://localhost:8080/transactions/wallet?wallet=0x6C83946e958Db8A7F8a49981A412E5f59be27E3B"
   curl "http://localhost:8080/transactions/wallet?wallet=ADDRESS_OF_SECOND_WALLET"
   ```

## 4. Get Test USDT on BSC Testnet

1. You'll need to interact with a testnet USDT contract
2. The contract address is already configured in your app: `0x337610d27c682E347C9cD60BD4b3b107C9d34dDd`
3. You can get test USDT by:
   - Swapping some test BNB for USDT on PancakeSwap Testnet (https://pancake.kiemtienonline360.com/)
   - Or using a USDT test faucet if available

## 5. Transfer Test USDT Between Wallets

1. After you have test USDT, transfer some to the second wallet:
   ```bash
   curl -X POST "http://localhost:8080/transfer" \
   -H "Content-Type: application/json" \
   -d '{
     "from_wallet_id": 17,
     "to_address": "ADDRESS_OF_SECOND_WALLET",
     "amount": "1.0",
     "token_address": "0x337610d27c682E347C9cD60BD4b3b107C9d34dDd",
     "priority": "medium"
   }'
   ```

## 6. Monitor Transaction Progress

1. Check the transaction status on BSC Testnet Explorer:
   - When you execute a transfer, you'll get a transaction hash
   - Go to https://testnet.bscscan.com/ and search for that hash
   - You can see details like status, block confirmations, gas used, etc.

2. View your application logs to see transaction processing:
   ```bash
   docker logs crypto-p2p-trading-app | grep -i "transaction"
   ```

## 7. Test Error Handling

1. Try a transaction with insufficient funds:
   ```bash
   curl -X POST "http://localhost:8080/transfer" \
   -H "Content-Type: application/json" \
   -d '{
     "from_wallet_id": 17,
     "to_address": "ADDRESS_OF_SECOND_WALLET",
     "amount": "1000.0",
     "priority": "medium"
   }'
   ```

2. Check how your app handles the error

## 8. Test Different Priority Levels

1. Try sending with different priority levels (affects gas price):
   ```bash
   # Low priority
   curl -X POST "http://localhost:8080/transfer" \
   -H "Content-Type: application/json" \
   -d '{
     "from_wallet_id": 17,
     "to_address": "ADDRESS_OF_SECOND_WALLET",
     "amount": "0.01",
     "priority": "low"
   }'
   
   # High priority
   curl -X POST "http://localhost:8080/transfer" \
   -H "Content-Type: application/json" \
   -d '{
     "from_wallet_id": 17,
     "to_address": "ADDRESS_OF_SECOND_WALLET",
     "amount": "0.01",
     "priority": "high"
   }'
   ```

## 9. Verify All Operations in Debug Mode

1. Check the logs to confirm that all operations are using testnet:
   ```bash
   docker logs crypto-p2p-trading-app | grep -i "testnet\|debug mode"
   ```

## Testing Flow On MainNet Production

### Step 1: Create a User Order

1. **Make a request to create an order**:

   ```bash
   curl -X POST "http://localhost:8080/create_order?user_id=1&amount=1"
   ```

2. **Expected response**:

   ```json
   {
     "status": "success",
     "wallet": "0x123abc..." 
   }
   ```

3. **Save the wallet address** from the response for later use

### Step 2: Verify Order Creation

1. **Check the user's orders**:

   ```bash
   curl -X GET "http://localhost:8080/orders/user?user_id=1"
   ```

2. **Expected response**:

   ```json
   [
     {
       "id": 1,
       "user_id": 1,
       "wallet": "0x123abc...",
       "amount": "1",
       "status": "pending"
     }
   ]
   ```

3. **Verify** that the order status is "pending" and the wallet address matches the one from Step 1

### Step 3: Generate Additional Wallet (Optional)

1. **Generate another wallet for the same user**:

   ```bash
   curl -X POST "http://localhost:8080/generate_wallet?user_id=1"
   ```

2. **Expected response**:

   ```json
   {
     "status": "success",
     "wallet": "0x456def..." 
   }
   ```

3. **Note** that this wallet address should be different from the first one

### Step 4: Verify User Wallets

1. **Check all wallets for the user**:

   ```bash
   curl -X GET "http://localhost:8080/wallets/user?user_id=1"
   ```

2. **Expected response**:

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

3. **Verify** that both wallet addresses are listed

### Step 5: Send USDT to the Order's Wallet

1. **Using Metamask or another BSC wallet**:
   - Connect to BSC network (or BSC Testnet for testing)
   - Add the USDT token contract if not already added
   - Send the exact amount of USDT (1 in this example) to the wallet address from Step 1

2. Here we have to stop and wait for user to send crypto to generated deposit wallet!

3. **Wait for the transaction** to be confirmed on the blockchain (usually takes a few seconds to minutes)

### Step 6: Monitor Transaction Status

1. **Check the transactions for the wallet**:

   ```bash
   curl -X GET "http://localhost:8080/transactions/wallet?wallet=0x123abc..."
   ```

2. **Expected response** (initially):

   ```json
   [
     {
       "id": 1,
       "tx_hash": "0xabc123...",
       "wallet": "0x123abc...",
       "amount": "1",
       "block_number": 12345678,
       "confirmed": false,
       "created_at": "2023-03-15T12:34:56Z"
     }
   ]
   ```

3. **Wait for confirmations** (the system requires some confirmations based on config parameter)

4. **Check again after a few minutes**:

   ```bash
   curl -X GET "http://localhost:8080/transactions/wallet?wallet=0x123abc..."
   ```

5. **Expected response** (after confirmations):

   ```json
   [
     {
       "id": 1,
       "tx_hash": "0xabc123...",
       "wallet": "0x123abc...",
       "amount": "1",
       "block_number": 12345678,
       "confirmed": true,
       "created_at": "2023-03-15T12:34:56Z"
     }
   ]
   ```

!! Note that transactions are processed every minute by a ticker in the pollAndProcess method of the BinanceSmartChain struct

### Step 7: Verify Order Status Update

1. **Check the user's orders again**:

   ```bash
   curl -X GET "http://localhost:8080/orders/user?user_id=1"
   ```

2. **Expected response**:

   ```json
   [
     {
       "id": 1,
       "user_id": 1,
       "wallet": "0x123abc...",
       "amount": "1",
       "status": "completed"
     }
   ]
   ```

3. **Verify** that the order status has changed from "pending" to "completed"

### Step 8: Test Multiple Orders with Same Wallet

1. **Create another order for the same user**:

   ```bash
   curl -X POST "http://localhost:8080/create_order?user_id=1&amount=5.0"
   ```

2. **Expected response**:

   ```json
   {
     "status": "success",
     "wallet": "0x123abc..." 
   }
   ```

3. **Note** that the system creates new deposit wallet for the same user but new order! This is a good practice in p2p systems!
It's for better tracking and identification.

4. **Verify the new order**:

   ```bash
   curl -X GET "http://localhost:8080/orders/user?user_id=1"
   ```

5. **Expected response**:

   ```json
   [
     {
       "id": 1,
       "user_id": 1,
       "wallet": "0x123abc...",
       "amount": "1",
       "status": "completed"
     },
     {
       "id": 2,
       "user_id": 1,
       "wallet": "0x123abc...",
       "amount": "5.0",
       "status": "pending"
     }
   ]
   ```

### Step 9: Test Multiple Users

1. **Create an order for a different user**:

   ```bash
   curl -X POST "http://localhost:8080/create_order?user_id=2&amount=15.0"
   ```

2. **Expected response**:

   ```json
   {
     "status": "success",
     "wallet": "0x789ghi..." 
   }
   ```

3. **Note** that a new wallet is generated for the new user

4. **Verify the new user's order**:

   ```bash
   curl -X GET "http://localhost:8080/orders/user?user_id=2"
   ```

5. **Expected response**:

   ```json
   [
     {
       "id": 3,
       "user_id": "2",
       "wallet": "0x789ghi...",
       "amount": "15.0",
       "status": "pending"
     }
   ]
   ```

## Monitoring and Debugging

### Check Application Logs

If you're running with Docker:

```bash
make docker-logs
```

Look for:

- Wallet generation events
- Blockchain monitoring messages
- Transaction detection and processing
- Order status updates

### Check Database State

If you have direct access to the PostgreSQL database:

1. Connect to the database:

   ```bash
   psql postgres://user:password@localhost:5432/exchange
   ```

2. Check wallets table:

   ```sql
   SELECT * FROM wallets;
   ```

3. Check orders table:

   ```sql
   SELECT * FROM orders;
   ```

4. Check transactions table:

   ```sql
   SELECT * FROM transactions;
   ```

## Troubleshooting

### Transaction Not Detected

1. Verify the transaction on a BSC block explorer (like BscScan)
2. Check that you sent to the correct wallet address
3. Ensure you sent BEP-20 USDT (not another token)
4. Check application logs for blockchain monitoring messages

### Order Status Not Updating

1. Check if the transaction has enough confirmations
2. Verify the transaction amount matches the order amount
3. Check application logs for any errors in transaction processing
4. Restart the application if necessary to trigger a recheck of pending transactions

### Wallet Generation Issues

1. Check application logs for errors during wallet generation
2. Verify the wallet seed phrase is correctly configured
3. Ensure the database is accessible and properly configured

## Advanced Testing

### Test Transaction Confirmation Thresholds

1. Modify the `RequiredConfirmations` constant in the code to a higher value
2. Restart the application
3. Create a new order and send USDT
4. Observe how the system waits for the specified number of confirmations

### Test Wallet Index Sequencing

1. Create multiple orders for the same user, forcing new wallet generation each time
2. Check the database to verify that wallet indices are sequential
3. Verify that each user has their own sequence of indices

This step-by-step guide should help thoroughly test cryptocurrency order processing system with the enhanced wallet management features.

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
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    wallet_id BIGINT NOT NULL,
    amount VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_orders_wallet FOREIGN KEY (wallet_id)
        REFERENCES wallets(id)
        ON DELETE CASCADE

CREATE INDEX IF NOT EXISTS idx_orders_wallet_id ON orders(wallet_id);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

-- Wallets table
create table if not exists wallets
(
    id              bigserial primary key,
    user_id         bigint       not null,
    address         varchar(255) not null unique,
    derivation_path varchar(255) not null,
    wallet_index    integer                  default 0,
    created_at      timestamp with time zone default CURRENT_TIMESTAMP,
    constraint unique_user_wallet_index
        unique (user_id, wallet_index)
);

create index idx_wallets_address on public.wallets (address);
create index idx_wallets_index on public.wallets (wallet_index);
create index idx_wallets_user_id on public.wallets (user_id);
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

### Handling Mismatched Transaction Amounts

The system handles cases where the transaction amount doesn't exactly match the order amount:

1. **Insufficient Amount**: If a user sends less than the order amount:
   - The transaction is detected and recorded in the database
   - The transaction is confirmed after receiving the required confirmations
   - The order remains in "pending" status, waiting for additional funds
   - No partial completion is currently supported

2. **Excess Amount**: If a user sends more than the order amount:
   - The transaction is detected, recorded, and confirmed
   - The order is marked as "completed"
   - Excess funds remain in the wallet but are not automatically applied to other orders
   - The system does not currently provide automatic refunds for excess amounts

This behavior is controlled by the amount comparison logic in the order processing system, which only completes an order when the received amount is greater than or equal to the order amount.

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
