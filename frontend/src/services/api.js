import axios from 'axios';

// Use environment variable for API base URL, default to localhost for development
export const BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';
export const API_BASE_URL = `${BASE_URL}/data`;

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

// Create a new order
export const createOrder = async (userId, amount) => {
  try {
    const response = await axios.post(`${BASE_URL}/create_order?user_id=${userId}&amount=${amount}`);
    return response.data;
  } catch (error) {
    console.error('Error creating order:', error);
    throw error;
  }
};

// Get user orders
export const getUserOrders = async (userId) => {
  try {
    const response = await axios.get(`${BASE_URL}/orders/user?user_id=${userId}`);
    return response.data;
  } catch (error) {
    console.error(`Error fetching orders for user ${userId}:`, error);
    throw error;
  }
};

// Get user wallets
export const getUserWallets = async (userId) => {
  try {
    const response = await axios.get(`${BASE_URL}/wallets/user?user_id=${userId}`);
    return response.data;
  } catch (error) {
    console.error(`Error fetching wallets for user ${userId}:`, error);
    throw error;
  }
};

// Get wallet details with IDs
export const getWalletDetails = async (userId) => {
  try {
    const response = await axios.get(`${BASE_URL}/wallets/ids?user_id=${userId}`);
    return response.data;
  } catch (error) {
    console.error(`Error fetching wallet details for user ${userId}:`, error);
    throw error;
  }
};

// Get extended wallet details with creation date
export const getWalletDetailsExtended = async (userId) => {
  try {
    const response = await axios.get(`${BASE_URL}/wallets/extended?user_id=${userId}`);
    return response.data;
  } catch (error) {
    console.error(`Error fetching extended wallet details for user ${userId}:`, error);
    throw error;
  }
};

// Get wallet transactions
export const getWalletTransactions = async (walletAddress) => {
  try {
    const response = await axios.get(`${BASE_URL}/transactions/wallet?wallet=${walletAddress}`);
    return response.data;
  } catch (error) {
    console.error(`Error fetching transactions for wallet ${walletAddress}:`, error);
    throw error;
  }
};

// Get transaction ID for a wallet (returns the most recent confirmed transaction)
export const getTransactionIdForWallet = async (walletAddress) => {
  try {
    const transactions = await getWalletTransactions(walletAddress);
    // Find the most recent confirmed transaction
    const confirmedTransaction = transactions.find(tx => tx.confirmed);
    return confirmedTransaction ? confirmedTransaction.tx_hash : null;
  } catch (error) {
    console.error(`Error fetching transaction ID for wallet ${walletAddress}:`, error);
    return null;
  }
};

// Get balances for all wallets
export const getWalletBalances = async () => {
  try {
    const response = await axios.get(`${BASE_URL}/wallet/balances`);
    return response.data;
  } catch (error) {
    console.error('Error fetching wallet balances:', error);
    throw error;
  }
};

// Get balance for a specific wallet
export const getWalletBalance = async (address) => {
  try {
    const response = await axios.get(`${BASE_URL}/wallet/balance?address=${address}`);
    return response.data;
  } catch (error) {
    console.error(`Error fetching balance for wallet ${address}:`, error);
    throw error;
  }
};
