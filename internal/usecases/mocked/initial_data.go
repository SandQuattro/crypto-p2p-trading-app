package mocked

import (
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"math"
	"time"
)

// GenerateInitialCandleData generates initial candle data for a trading pair.
func (s *DataService) GenerateInitialCandleData(pair *entities.TradingPair) {
	now := time.Now()
	// Round to the beginning of the current 5-minute interval
	currentInterval := time.Date(
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute()-now.Minute()%minutesPerCandle, 0, 0,
		now.Location(),
	)
	startTime := currentInterval.Add(-hoursPerDay * time.Hour) // 24 hours ago

	// Create slice with required capacity for optimization
	pair.Mutex.Lock()
	defer pair.Mutex.Unlock()

	pair.CandleData = make([]entities.CandleData, 0, maxCandleCount)

	// Base price for the first candle
	basePrice := pair.LastPrice * basePercentage

	// Generate candles for the last 24 hours (5-minute candles)
	for i := range make([]int, maxCandleCount) { // 288 candles of 5 minutes each = 24 hours
		candleTime := startTime.Add(time.Duration(i) * minutesPerCandle * time.Minute)

		// Create a small price change for each candle
		priceChange := basePrice * (secureFloat64(s.logger)*maxPriceVariationPercent -
			minPriceVariationPercent) // -2% to +2%
		basePrice += priceChange

		// Create candle with random fluctuations
		openPrice := basePrice * (openCloseVariationBase +
			secureFloat64(s.logger)*openCloseVariationRange)
		closePrice := basePrice * (openCloseVariationBase +
			secureFloat64(s.logger)*openCloseVariationRange)
		high := math.Max(openPrice, closePrice) * (highPriceVariationBase +
			secureFloat64(s.logger)*highPriceVariationRange)
		low := math.Min(openPrice, closePrice) * (lowPriceVariationBase -
			secureFloat64(s.logger)*lowPriceVariationRange)
		volume := defaultVolume + secureFloat64(s.logger)*maxVolumeVariation

		candle := entities.CandleData{
			Time:   candleTime.Unix() * timestampMultiplier, // milliseconds
			Open:   openPrice,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: volume,
		}

		pair.CandleData = append(pair.CandleData, candle)
	}

	// Set last candle
	if len(pair.CandleData) > 0 {
		pair.LastCandle = pair.CandleData[len(pair.CandleData)-1]
		pair.LastPrice = pair.LastCandle.Close
	}

	s.logger.Info("Generated candles", "symbol", pair.Symbol, "count", len(pair.CandleData))
}
