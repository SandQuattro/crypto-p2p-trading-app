package mocked

import (
	"crypto/rand"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
	"log/slog"
	"math/big"
	"time"

	"github.com/gorilla/websocket"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
)

// Constants to avoid magic numbers.
const (
	// Random number generation.
	maxRandomBits      = 53  // Maximum bits for random number generation (JavaScript's Number.MAX_SAFE_INTEGER).
	defaultRandomValue = 0.5 // Default value when random generation fails.

	// Trading pair initial prices.
	btcInitialPrice = 9_551_300.0
	ethInitialPrice = 345_460.0
	solInitialPrice = 24_020.0
	bnbInitialPrice = 64_350.0
	xrpInitialPrice = 144.0

	// Candle data constants.
	maxCandleCount       = 288  // 288 candles of 5 minutes each = 24 hours.
	priceUpdateInterval  = 500  // 500 milliseconds between price updates.
	timestampMultiplier  = 1000 // Convert seconds to milliseconds.
	defaultVolume        = 50   // Default trading volume.
	maxVolumeVariation   = 150  // Maximum volume variation for historical candles.
	smallVolumeVariation = 20   // Small volume variation for new candles.

	// Price simulation constants.
	basePercentage           = 0.95  // Base percentage for initial price calculation.
	maxPriceVariationPercent = 0.04  // Maximum price variation percentage (4%).
	minPriceVariationPercent = 0.02  // Minimum price variation percentage (2%).
	openCloseVariationBase   = 0.995 // Base variation for open/close prices (0.5% below).
	openCloseVariationRange  = 0.01  // Range of variation for open/close prices (1%).
	highPriceVariationBase   = 1.0   // Base multiplier for high price.
	highPriceVariationRange  = 0.005 // Range of variation for high price (0.5%).
	lowPriceVariationBase    = 0.995 // Base multiplier for low price.
	lowPriceVariationRange   = 0.005 // Range of variation for low price (0.5%).

	// Time constants.
	minutesPerCandle     = 5  // Each candle represents 5 minutes.
	hoursPerDay          = 24 // Hours in a day for historical data.
	candleTickerInterval = 1  // 1 second interval for candle ticker.
	demoIntervalSeconds  = 10 // 10-second interval for demonstration.

	// Simulation constants.
	realtimePriceVariationMax = 0.004 // Maximum price variation for real-time updates (0.4%).
	realtimePriceVariationMin = 0.002 // Minimum price variation for real-time updates (0.2%).
	percentMultiplier         = 100   // Multiplier to convert decimal to percentage.
)

type DataService struct {
	TradingPairs map[string]*entities.TradingPair
	logger       *slog.Logger
}

func NewDataService(logger *slog.Logger) *DataService {
	return &DataService{
		TradingPairs: make(map[string]*entities.TradingPair),
		logger:       logger,
	}
}

// NewTradingPair creates a new trading pair.
func NewTradingPair(symbol string, initialPrice float64) *entities.TradingPair {
	return &entities.TradingPair{
		Symbol:          symbol,
		LastPrice:       initialPrice,
		PriceChange:     0,
		OrdersPerSecond: 0,
		CandleData:      make([]entities.CandleData, 0),
		Subscribers:     make(map[*websocket.Conn]bool),
		StopChan:        make(chan struct{}),
		LastOrderTime:   time.Now(),
	}
}

// secureFloat64 generates a random number from 0 to 1 using crypto/rand.
func secureFloat64(logger *slog.Logger) float64 {
	// Generate a random number from 0 to 1<<53
	maxVal := big.NewInt(1 << maxRandomBits)
	n, err := rand.Int(rand.Reader, maxVal)
	if err != nil {
		// In case of error, return 0.5 as a safe default value
		logger.Error("Error generating secure random number", "error", err)
		return defaultRandomValue
	}
	// Convert to float64 from 0 to 1
	return float64(n.Int64()) / float64(maxVal.Int64())
}

// InitializeTradingPairs initializes trading pairs with initial data.
func (s *DataService) InitializeTradingPairs() {
	// Create trading pairs with initial prices
	s.TradingPairs["BTCRUB"] = NewTradingPair("BTCRUB", btcInitialPrice)
	s.TradingPairs["ETHRUB"] = NewTradingPair("ETHRUB", ethInitialPrice)
	s.TradingPairs["SOLRUB"] = NewTradingPair("SOLRUB", solInitialPrice)
	s.TradingPairs["BNBRUB"] = NewTradingPair("BNBRUB", bnbInitialPrice)
	s.TradingPairs["XRPRUB"] = NewTradingPair("XRPRUB", xrpInitialPrice)

	// Generate initial candle data
	for _, pair := range s.TradingPairs {
		s.GenerateInitialCandleData(pair)
		// Start simulation in a separate goroutine
		go s.SimulateTradingData(pair)
	}
}

// updatePriceAndCandle updates the price and current candle.
func (s *DataService) updatePriceAndCandle(
	pair *entities.TradingPair,
	currentCandle *entities.CandleData,
) {
	// Generate a random price variation
	variation := realtimePriceVariationMin +
		(realtimePriceVariationMax-realtimePriceVariationMin)*secureFloat64(s.logger)

	// Randomly decide if the price should go up or down
	if secureFloat64(s.logger) < 0.5 {
		variation = -variation
	}

	// Calculate new price
	pair.Mutex.Lock()
	oldPrice := pair.LastPrice
	newPrice := oldPrice * (1 + variation)
	pair.LastPrice = newPrice

	// Calculate price change percentage
	pair.PriceChange = ((newPrice - oldPrice) / oldPrice) * percentMultiplier

	// Update current candle
	currentCandle.Close = newPrice
	if newPrice > currentCandle.High {
		currentCandle.High = newPrice
	}
	if newPrice < currentCandle.Low {
		currentCandle.Low = newPrice
	}

	// Add a small random volume
	volumeVariation := smallVolumeVariation * secureFloat64(s.logger)
	currentCandle.Volume += volumeVariation

	// Update last candle
	pair.LastCandle = *currentCandle
	pair.Mutex.Unlock()

	// Track order processing - each price update represents an order
	s.trackOrder(pair)
}

// trackOrder increments the order count and calculates orders per second
func (s *DataService) trackOrder(pair *entities.TradingPair) {
	pair.OrderCountMutex.Lock()
	defer pair.OrderCountMutex.Unlock()

	// Increment order count
	pair.OrderCount++

	// Calculate orders per second every second
	now := time.Now()
	duration := now.Sub(pair.LastOrderTime)
	if duration >= time.Second {
		// Calculate orders per second
		ordersPerSecond := float64(pair.OrderCount) / duration.Seconds()

		// Update the orders per second value
		pair.Mutex.Lock()
		pair.OrdersPerSecond = ordersPerSecond
		pair.Mutex.Unlock()

		// Reset counter and timer
		pair.OrderCount = 0
		pair.LastOrderTime = now

		// comment orders logs temporarily
		//s.logger.Info("Orders per second updated",
		//	"symbol", pair.Symbol,
		//	"ordersPerSecond", ordersPerSecond,
		//	"duration", duration.Seconds())
	}
}

// createNewCandle creates a new candle and adds the current one to history.
func (s *DataService) createNewCandle(
	pair *entities.TradingPair,
	currentCandle *entities.CandleData,
	roundedTime time.Time,
) {
	pair.Mutex.Lock()
	defer pair.Mutex.Unlock()

	// Save current candle to history
	if len(pair.CandleData) == 0 || currentCandle.Time > pair.CandleData[len(pair.CandleData)-1].Time {
		pair.CandleData = append(pair.CandleData, *currentCandle)
		// Keep only last 288 candles
		if len(pair.CandleData) > maxCandleCount {
			pair.CandleData = pair.CandleData[len(pair.CandleData)-maxCandleCount:]
		}

		// comment new candle creation temporarily
		//s.logger.Info("Created new candle for pair", "symbol", pair.Symbol,
		//	"time", time.Unix(currentCandle.Time/timestampMultiplier, 0))
	}

	// Create a new current candle
	*currentCandle = entities.CandleData{
		Time:   roundedTime.Unix() * timestampMultiplier,
		Open:   pair.LastPrice,
		High:   pair.LastPrice,
		Low:    pair.LastPrice,
		Close:  pair.LastPrice,
		Volume: defaultVolume + secureFloat64(s.logger)*smallVolumeVariation,
	}

	// Update last candle
	pair.LastCandle = *currentCandle
}

// getRoundedTime returns a time rounded to the demonstration interval.
func getRoundedTime() time.Time {
	now := time.Now()
	// Use a 10-second interval for demonstration
	return time.Date(
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second()/demoIntervalSeconds*demoIntervalSeconds, 0,
		now.Location(),
	)
}

// initializeCurrentCandle gets or creates the current candle.
func (s *DataService) initializeCurrentCandle(
	pair *entities.TradingPair,
) entities.CandleData {
	pair.Mutex.RLock()
	defer pair.Mutex.RUnlock()

	if len(pair.CandleData) > 0 {
		return pair.CandleData[len(pair.CandleData)-1]
	}

	roundedTime := getRoundedTime()
	return entities.CandleData{
		Time:   roundedTime.Unix() * timestampMultiplier,
		Open:   pair.LastPrice,
		High:   pair.LastPrice,
		Low:    pair.LastPrice,
		Close:  pair.LastPrice,
		Volume: defaultVolume,
	}
}

// handlePriceUpdate handles the price ticker update.
func (s *DataService) handlePriceUpdate(pair *entities.TradingPair, currentCandle *entities.CandleData) {
	s.updatePriceAndCandle(pair, currentCandle)
	s.BroadcastUpdate(pair)
}

// handleCandleUpdate handles the candle ticker update.
func (s *DataService) handleCandleUpdate(pair *entities.TradingPair, currentCandle *entities.CandleData) {
	roundedTime := getRoundedTime()

	// Check if we need to create a new candle
	if roundedTime.Unix()*timestampMultiplier > currentCandle.Time {
		s.createNewCandle(pair, currentCandle, roundedTime)
		s.BroadcastUpdate(pair)

		// Creating a new candle also counts as an order
		s.trackOrder(pair)
	}
}

// SimulateTradingData simulates real-time trading data for a pair.
func (s *DataService) SimulateTradingData(pair *entities.TradingPair) {
	// Ticker for price updates (every 500ms)
	priceTicker := time.NewTicker(time.Duration(priceUpdateInterval) * time.Millisecond)
	// Ticker for new candles (every 1 second)
	candleTicker := time.NewTicker(candleTickerInterval * time.Second)
	defer priceTicker.Stop()
	defer candleTicker.Stop()

	// Ensure we have candle data
	pair.Mutex.RLock()
	hasData := len(pair.CandleData) > 0
	pair.Mutex.RUnlock()

	if !hasData {
		s.logger.Info("Generating initial candle data for pair in simulateTradingData",
			"symbol", pair.Symbol)
		s.GenerateInitialCandleData(pair)
	}

	// Current candle
	currentCandle := s.initializeCurrentCandle(pair)

	for {
		select {
		case <-pair.StopChan:
			return
		case <-priceTicker.C:
			s.handlePriceUpdate(pair, &currentCandle)
		case <-candleTicker.C:
			s.handleCandleUpdate(pair, &currentCandle)
		}
	}
}

// BroadcastUpdate sends updates to all subscribers.
func (s *DataService) BroadcastUpdate(pair *entities.TradingPair) {
	pair.Mutex.RLock()
	defer pair.Mutex.RUnlock()

	// If there are no subscribers, exit
	if len(pair.Subscribers) == 0 {
		return
	}

	// Prepare data for sending
	update := map[string]interface{}{
		"symbol":          pair.Symbol,
		"lastPrice":       pair.LastPrice,
		"priceChange":     pair.PriceChange,
		"ordersPerSecond": pair.OrdersPerSecond,
		"lastCandle":      pair.LastCandle,
	}

	// Send update to all subscribers
	for conn := range pair.Subscribers {
		err := conn.WriteJSON(update)
		if err != nil {
			s.logger.Error("Error sending update to subscriber", "error", err)
			conn.Close()
			delete(pair.Subscribers, conn)
		}
	}
}

// GetCandleData returns candle data for a pair.
func (s *DataService) GetCandleData(symbol string) ([]entities.CandleData, error) {
	pair, ok := s.TradingPairs[symbol]
	if !ok {
		return nil, usecases.ErrTradingPairNotFound
	}

	pair.Mutex.RLock()
	defer pair.Mutex.RUnlock()

	// Return a copy of the data to avoid race conditions
	result := make([]entities.CandleData, len(pair.CandleData))
	copy(result, pair.CandleData)

	return result, nil
}

// AddSubscriber adds a subscriber for receiving updates.
func (s *DataService) AddSubscriber(symbol string, conn *websocket.Conn) error {
	pair, ok := s.TradingPairs[symbol]
	if !ok {
		return usecases.ErrTradingPairNotFound
	}

	pair.Mutex.Lock()
	defer pair.Mutex.Unlock()
	pair.Subscribers[conn] = true
	s.logger.Info("Added subscriber for pair", "symbol", symbol, "totalSubscribers", len(pair.Subscribers))
	return nil
}

// RemoveSubscriber removes a subscriber.
func (s *DataService) RemoveSubscriber(symbol string, conn *websocket.Conn) error {
	pair, ok := s.TradingPairs[symbol]
	if !ok {
		return usecases.ErrTradingPairNotFound
	}

	pair.Mutex.Lock()
	defer pair.Mutex.Unlock()
	delete(pair.Subscribers, conn)
	s.logger.Info("Removed subscriber for pair", "symbol", symbol, "remainingSubscribers", len(pair.Subscribers))
	return nil
}
