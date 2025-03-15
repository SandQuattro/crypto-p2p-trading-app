# Crypto P2P Trading App Frontend

This is the frontend part of the Crypto P2P Trading application. It's built with React and uses Lightweight Charts for candlestick chart visualization.

## Features

- Real-time cryptocurrency price updates via WebSockets
- Interactive candlestick chart
- Multiple trading pair support
- Price and change display

## Getting Started

### Installation

1. Install dependencies:
   ```
   npm install
   ```

2. Start the development server:
   ```
   npm start
   ```

3. Open your browser and navigate to `http://localhost:3000`

## Project Structure

- `src/components/` - React components
  - `TradingChart.js` - Candlestick chart component
  - `PairSelector.js` - Trading pair selection component
  - `PriceDisplay.js` - Price and change display component
- `src/services/` - API services
  - `api.js` - API calls to the backend

## Dependencies

- React
- Lightweight Charts - For candlestick chart visualization
- Axios - For HTTP requests
