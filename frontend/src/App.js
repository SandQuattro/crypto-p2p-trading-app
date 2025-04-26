import React, {useEffect, useState} from 'react';
import TradingChart from './components/TradingChart';
import PairSelector from './components/PairSelector';
import PriceDisplay from './components/PriceDisplay';
import Navigation from './components/Navigation';
import OrdersPage from './components/OrdersPage';
import WalletsPage from './components/WalletsPage';
import {fetchTradingPairs} from './services/api';
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

  useEffect(() => {
    if (!selectedPair) return;

    // Log the environment variable to check if it's being picked up during build
    console.log('REACT_APP_API_URL (App.js for WS):', process.env.REACT_APP_API_URL);

    // Determine WebSocket URL based on API_URL
    const apiUrl = process.env.REACT_APP_API_URL || 'http://localhost:8080';
    // Replace http with ws, or https with wss
    const wsUrl = apiUrl.replace(/^(http)/, 'ws');

    console.log("Connecting WebSocket to:", `${wsUrl}/ws/${selectedPair}`); // Debug log

    const ws = new WebSocket(`${wsUrl}/ws/${selectedPair}`);

    ws.onopen = () => {
      console.log('WebSocket connection opened');
    };

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      setPairData({
        lastPrice: data.lastPrice,
        priceChange: data.priceChange,
        ordersPerSecond: data.ordersPerSecond || 0
      });
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    return () => {
      ws.close();
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
          />
        </div>
      )}
    </div>
  );

  return (
    <div className="app-container">
      <header className="header">
        <h1>Crypto P2P Trading</h1>
        <Navigation activeTab={activeTab} onTabChange={handleTabChange} />
      </header>

      {activeTab === 'trading' ? renderTradingView()
        : activeTab === 'orders' ? <OrdersPage />
          : <WalletsPage />}
    </div>
  );
}

export default App;
