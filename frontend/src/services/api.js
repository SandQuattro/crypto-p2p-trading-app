import axios from 'axios';

const API_BASE_URL = 'http://localhost:8080/api';

// Fetch all available trading pairs
export const fetchTradingPairs = async () => {
  try {
    const response = await axios.get(`${API_BASE_URL}/pairs`);
    return response.data;
  } catch (error) {
    console.error('Error fetching trading pairs:', error);
    throw error;
  }
};

// Fetch candle data for a specific trading pair
export const fetchCandleData = async (symbol) => {
  try {
    const response = await axios.get(`${API_BASE_URL}/candles/${symbol}`);
    return response.data;
  } catch (error) {
    console.error(`Error fetching candle data for ${symbol}:`, error);
    throw error;
  }
};
