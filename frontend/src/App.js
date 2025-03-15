import React, {useEffect, useState} from 'react';
import TradingChart from './components/TradingChart';
import PairSelector from './components/PairSelector';
import PriceDisplay from './components/PriceDisplay';
import {fetchTradingPairs} from './services/api';

function App() {
  const [tradingPairs, setTradingPairs] = useState([]);
  const [selectedPair, setSelectedPair] = useState('');
  const [pairData, setPairData] = useState({
    lastPrice: 0,
    priceChange: 0,
    ordersPerSecond: 0
  });
  const [isLoaded, setIsLoaded] = useState(false);

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

    const ws = new WebSocket(`ws://localhost:8080/ws/${selectedPair}`);

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

  return (
    <div className="app-container">
      <header className="header">
        <h1>Crypto P2P Trading</h1>
      </header>
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
    </div>
  );
}

export default App;
