import React, {useEffect, useRef, useState} from 'react';
import TradingChart from './components/TradingChart';
import PairSelector from './components/PairSelector';
import PriceDisplay from './components/PriceDisplay';
import Navigation from './components/Navigation';
import OrdersPage from './components/OrdersPage';
import WalletsPage from './components/WalletsPage';
import {API_BASE_URL, BASE_URL, fetchTradingPairs} from './services/api';
import {NotificationProvider} from './context/NotificationContext';
import NotificationContainer from './components/NotificationContainer';
import './App.css';

function App() {
  const [tradingPairs, setTradingPairs] = useState([]);
  const [selectedPair, setSelectedPair] = useState('');
  const [pairData, setPairData] = useState({
    lastPrice: 0,
    priceChange: 0,
    ordersPerSecond: 0
  });
  const [isLoaded, setIsLoaded] = useState(false);
  const [activeTab, setActiveTab] = useState('trading');
  const [candleData, setCandleData] = useState([]);
  const [lastCandle, setLastCandle] = useState(null);
  const ws = useRef(null);

  useEffect(() => {
    const loadTradingPairs = async () => {
      try {
        const pairs = await fetchTradingPairs();
        setTradingPairs(pairs);

        if (pairs.length > 0) {
          const btcPair = pairs.find(p => p.symbol === 'BTCRUB');
          const initialPair = btcPair || pairs[0];

          setPairData({
            lastPrice: initialPair.lastPrice,
            priceChange: initialPair.priceChange,
            ordersPerSecond: initialPair.ordersPerSecond || 0
          });

          setTimeout(() => {
            setSelectedPair(initialPair.symbol);
            setIsLoaded(true);
          }, 500);
        }
      } catch (error) {
        console.error('Error loading trading pairs:', error);
      }
    };

    loadTradingPairs();
  }, []);

  // Fetch historical candle data when symbol changes
  useEffect(() => {
    const fetchCandleData = async () => {
      if (!selectedPair) return;

      try {
        console.log(`Fetching candles for ${selectedPair}`);
        const response = await fetch(`${API_BASE_URL}/candles/${selectedPair}`);
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        console.log(`Received ${data.length} candles for ${selectedPair}`);

        // Format data for the chart
        const formattedData = data.map(candle => ({
          time: candle.time / 1000, // Convert from milliseconds to seconds
          open: candle.open,
          high: candle.high,
          low: candle.low,
          close: candle.close,
        }));

        setCandleData(formattedData);
      } catch (error) {
        console.error('Error fetching candle data:', error);
      }
    };

    fetchCandleData();
  }, [selectedPair]);

  // Setup WebSocket connection
  useEffect(() => {
    if (!selectedPair) return;

    console.log('REACT_APP_API_URL (App.js for WS):', process.env.REACT_APP_API_URL);

    // Close existing WebSocket if it exists
    if (ws.current) {
      console.log(`Closing existing WebSocket for ${selectedPair}`);
      ws.current.close();
    }

    // Determine WebSocket URL based on API_URL
    let wsProtocol = 'ws://';
    let wsHost = BASE_URL;
    if (BASE_URL.startsWith('https://')) {
      wsProtocol = 'wss://';
      wsHost = BASE_URL.substring(8); // Remove 'https://'
    } else if (BASE_URL.startsWith('http://')) {
      wsHost = BASE_URL.substring(7); // Remove 'http://'
    }

    const wsUrl = `${wsProtocol}${wsHost}/ws/${selectedPair}`;
    console.log(`Setting up WebSocket for ${selectedPair} to ${wsUrl}`);
    const socket = new WebSocket(wsUrl);

    // Optimization: use binary format for WebSocket
    socket.binaryType = "arraybuffer";

    socket.onopen = () => {
      console.log(`WebSocket connected for ${selectedPair}`);
    };

    socket.onmessage = (event) => {
      try {
        const update = JSON.parse(event.data);

        // Update price data
        if (update.lastPrice !== undefined) {
          setPairData({
            lastPrice: update.lastPrice,
            priceChange: update.priceChange,
            ordersPerSecond: update.ordersPerSecond || 0
          });
        }

        // Update candle data
        if (update.lastCandle) {
          const candle = update.lastCandle;

          const formattedCandle = {
            time: candle.time / 1000, // Convert from milliseconds to seconds
            open: candle.open,
            high: candle.high,
            low: candle.low,
            close: candle.close,
          };

          setLastCandle(formattedCandle);
        }
      } catch (error) {
        console.error('Error processing WebSocket message:', error);
      }
    };

    socket.onclose = () => {
      console.log(`WebSocket disconnected for ${selectedPair}`);
    };

    socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.current = socket;

    // Cleanup function
    return () => {
      if (ws.current) {
        console.log(`Closing WebSocket for ${selectedPair}`);
        ws.current.close();
      }
    };
  }, [selectedPair]);

  const handleSelectPair = (symbol) => {
    setSelectedPair(symbol);

    const pair = tradingPairs.find(p => p.symbol === symbol);
    if (pair) {
      setPairData({
        lastPrice: pair.lastPrice,
        priceChange: pair.priceChange,
        ordersPerSecond: pair.ordersPerSecond || 0
      });
    }
  };

  const handleTabChange = (tab) => {
    setActiveTab(tab);
  };

  const renderTradingView = () => (
    <div className="trading-container">
      <PairSelector
        pairs={tradingPairs
          .map(p => p.symbol)
          .sort((a, b) => {
            // Fixed order of pairs
            const order = {
              'BTCRUB': 1,
              'ETHRUB': 2,
              'SOLRUB': 3,
              'BNBRUB': 4,
              'XRPRUB': 5
            };
            return (order[a] || 999) - (order[b] || 999);
          })}
        selectedPair={selectedPair}
        onSelectPair={handleSelectPair}
      />
      <PriceDisplay
        symbol={selectedPair}
        lastPrice={pairData.lastPrice}
        priceChange={pairData.priceChange}
        ordersPerSecond={pairData.ordersPerSecond}
      />
      {isLoaded && selectedPair && (
        <div className="chart-container">
          <TradingChart
            key={selectedPair}
            symbol={selectedPair}
            candleData={candleData}
            lastCandle={lastCandle}
          />
        </div>
      )}
    </div>
  );

  return (
    <NotificationProvider>
      <div className="app-container">
        <NotificationContainer />
        <header className="header">
          <h1>Crypto P2P Trading</h1>
          <Navigation activeTab={activeTab} onTabChange={handleTabChange} />
        </header>

        {activeTab === 'trading' ? renderTradingView()
          : activeTab === 'orders' ? <OrdersPage lastPrice={pairData.lastPrice} symbol={selectedPair} />
            : <WalletsPage lastPrice={pairData.lastPrice} symbol={selectedPair} />}
      </div>
    </NotificationProvider>
  );
}

export default App;
