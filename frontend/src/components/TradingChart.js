import React, {useEffect, useRef} from 'react';
import {createChart} from 'lightweight-charts';

const TradingChart = ({ symbol, candleData, lastCandle }) => {
  const chartContainerRef = useRef(null);
  const chartRef = useRef(null);
  const candleSeriesRef = useRef(null);

  // Создаем график при изменении symbol
  useEffect(() => {
    if (!chartContainerRef.current || !symbol) return;

    console.log(`Creating new chart for ${symbol}`);

    // Очищаем контейнер перед созданием нового графика
    chartContainerRef.current.innerHTML = '';

    // Создаем новый график
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

    // Создаем серию свечей
    const candleSeries = chart.addCandlestickSeries({
      upColor: '#26a69a',
      downColor: '#ef5350',
      borderVisible: false,
      wickUpColor: '#26a69a',
      wickDownColor: '#ef5350',
    });

    // Сохраняем ссылки для использования в других эффектах
    chartRef.current = chart;
    candleSeriesRef.current = candleSeries;

    // Обработка изменения размера окна
    const handleResize = () => {
      chart.applyOptions({
        width: chartContainerRef.current.clientWidth,
      });
    };

    window.addEventListener('resize', handleResize);

    // Очистка при размонтировании
    return () => {
      console.log(`Cleaning up chart for ${symbol}`);
      window.removeEventListener('resize', handleResize);
      chart.remove();
      chartRef.current = null;
      candleSeriesRef.current = null;
    };
  }, [symbol]);

  // Обновляем данные свечей при получении новых
  useEffect(() => {
    if (!candleSeriesRef.current || !candleData || candleData.length === 0) return;

    console.log(`Setting ${candleData.length} candles for ${symbol}`);

    // Устанавливаем данные
    candleSeriesRef.current.setData(candleData);

    // Устанавливаем видимый диапазон (3 часа)
    if (chartRef.current && candleData.length > 0) {
      const lastTime = candleData[candleData.length - 1].time;
      const threeHoursAgo = lastTime - 3600 * 3; // 3600 секунд = 1 час, * 3 = 3 часа

      chartRef.current.timeScale().setVisibleRange({
        from: threeHoursAgo,
        to: lastTime,
      });

      // Принудительное обновление графика
      chartRef.current.applyOptions({
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
    }
  }, [candleData, symbol]);

  // Обновляем последнюю свечу при изменении
  useEffect(() => {
    if (!candleSeriesRef.current || !lastCandle) return;

    // Обновляем только последнюю свечу
    candleSeriesRef.current.update(lastCandle);
  }, [lastCandle]);

  return (
    <div className="chart-container" ref={chartContainerRef} />
  );
};

export default TradingChart;
