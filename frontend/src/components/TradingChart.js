import React, {useEffect, useRef} from 'react';
import {createChart} from 'lightweight-charts';
import {API_BASE_URL, BASE_URL} from '../services/api';

const TradingChart = ({ symbol }) => {
  const chartContainerRef = useRef(null);
  const ws = useRef(null);

  // Re-create chart whenever symbol changes
  useEffect(() => {
    if (!chartContainerRef.current || !symbol) return;

    console.log(`Creating new chart for ${symbol}`);

    // Clear container before creating a new chart
    chartContainerRef.current.innerHTML = '';

    // Create new chart
    const chart = createChart(chartContainerRef.current, {
      width: chartContainerRef.current.clientWidth,
      height: 500,
      layout: {
        background: { color: '#1E222D' },
        textColor: '#DDD',
      },
      grid: {
        vertLines: { color: '#2B2B43' },
        horzLines: { color: '#2B2B43' },
      },
      timeScale: {
        borderColor: '#2B2B43',
        timeVisible: true,
        secondsVisible: false,
        tickMarkFormatter: (time) => {
          const date = new Date(time * 1000);
          const hours = date.getHours().toString().padStart(2, '0');
          const minutes = date.getMinutes().toString().padStart(2, '0');
          return minutes === '00' ? `${hours}:00` : `${hours}:${minutes}`;
        },
      },
      crosshair: {
        mode: 0,
      },
    });

    // Create candlestick series
    const candleSeries = chart.addCandlestickSeries({
      upColor: '#26a69a',
      downColor: '#ef5350',
      borderVisible: false,
      wickUpColor: '#26a69a',
      wickDownColor: '#ef5350',
    });

    // Fetch historical candle data
    const fetchCandleData = async () => {
      try {
        console.log(`Fetching candles for ${symbol}`);
        const response = await fetch(`${API_BASE_URL}/candles/${symbol}`);
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        console.log(`Received ${data.length} candles for ${symbol}`);

        // Format data for the chart
        const formattedData = data.map(candle => ({
          time: candle.time / 1000, // Convert from milliseconds to seconds
          open: candle.open,
          high: candle.high,
          low: candle.low,
          close: candle.close,
        }));

        if (formattedData.length > 0) {
          console.log(`Setting ${formattedData.length} candles for ${symbol}`);

          // Use optimized method to set data
          candleSeries.setData(formattedData);

          // Set visible range to 24 hours
          const lastTime = formattedData[formattedData.length - 1].time;
          const oneDayAgo = lastTime - 3600 * 24; // 3600 seconds = 1 hour, * 24 = 24 hours

          // Optimize setting the visible range
          chart.timeScale().setVisibleRange({
            from: oneDayAgo,
            to: lastTime,
          });

          // Force chart update
          chart.applyOptions({
            timeScale: {
              rightOffset: 10,
              barSpacing: 6,
              fixLeftEdge: true,
              lockVisibleTimeRangeOnResize: true,
              rightBarStaysOnScroll: true,
              borderVisible: false,
              visible: true,
              timeVisible: true,
              secondsVisible: false
            }
          });
        } else {
          console.error(`No candle data received for ${symbol}`);
        }
      } catch (error) {
        console.error('Error fetching candle data:', error);
      }
    };

    fetchCandleData();

    // Setup WebSocket connection for real-time updates
    const setupWebSocket = () => {
      // Close existing WebSocket if it exists
      if (ws.current) {
        console.log(`Closing existing WebSocket for ${symbol}`);
        ws.current.close();
      }

      // Extract hostname from BASE_URL (removing http:// protocol)
      const wsHost = BASE_URL.replace(/^http:\/\//, '');

      console.log(`Setting up WebSocket for ${symbol}`);
      const socket = new WebSocket(`ws://${wsHost}/ws/${symbol}`);

      // Optimization: use binary format for WebSocket
      socket.binaryType = "arraybuffer";

      socket.onopen = () => {
        console.log(`WebSocket connected for ${symbol}`);
      };

      socket.onmessage = (event) => {
        try {
          const update = JSON.parse(event.data);

          // Update only if there is new candle data
          if (update.lastCandle) {
            const candle = update.lastCandle;

            // Optimization: check if we need to update the series
            const formattedCandle = {
              time: candle.time / 1000, // Convert from milliseconds to seconds
              open: candle.open,
              high: candle.high,
              low: candle.low,
              close: candle.close,
            };

            // Use more efficient update method
            candleSeries.update(formattedCandle);
          }
        } catch (error) {
          console.error('Error processing WebSocket message:', error);
        }
      };

      socket.onclose = () => {
        console.log(`WebSocket disconnected for ${symbol}`);
      };

      socket.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      ws.current = socket;
    };

    setupWebSocket();

    // Handle window resize
    const handleResize = () => {
      chart.applyOptions({
        width: chartContainerRef.current.clientWidth,
      });
    };

    window.addEventListener('resize', handleResize);

    // Cleanup function
    return () => {
      console.log(`Cleaning up chart for ${symbol}`);
      window.removeEventListener('resize', handleResize);
      if (ws.current) {
        console.log(`Closing WebSocket for ${symbol}`);
        ws.current.close();
      }
      chart.remove();
    };
  }, [symbol]);

  return (
    <div className="chart-container" ref={chartContainerRef} />
  );
};

export default TradingChart;
