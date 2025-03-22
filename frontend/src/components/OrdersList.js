import React, {useEffect, useState} from 'react';
import {getTransactionIdForWallet, getUserOrders, getWalletDetails} from '../services/api';
import '../App.css';

const OrdersList = ({ userId, refreshTrigger }) => {
    const [orders, setOrders] = useState([]);
    const [wallets, setWallets] = useState({});
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [transactionIds, setTransactionIds] = useState({});

    useEffect(() => {
        const fetchData = async () => {
            setLoading(true);
            setError('');

            try {
                // Fetch orders
                const ordersData = await getUserOrders(userId);
                setOrders(ordersData);

                // Fetch wallet details with IDs
                try {
                    const walletDetails = await getWalletDetails(userId);

                    // Create a map of wallet_id to wallet address
                    const walletsMap = {};
                    if (walletDetails && walletDetails.length > 0) {
                        walletDetails.forEach(wallet => {
                            if (wallet.id && wallet.address) {
                                walletsMap[wallet.id] = wallet.address;
                            }
                        });
                        setWallets(walletsMap);

                        // Fetch transaction IDs for completed orders
                        const txIdsMap = {};
                        const completedOrders = ordersData.filter(order => order.status === 'completed');

                        for (const order of completedOrders) {
                            const walletAddress = walletsMap[order.wallet_id];
                            if (walletAddress) {
                                const txId = await getTransactionIdForWallet(walletAddress);
                                if (txId) {
                                    txIdsMap[order.id] = txId;
                                }
                            }
                        }

                        setTransactionIds(txIdsMap);
                    }
                } catch (error) {
                    console.error('Error getting wallet details:', error);
                    setError('Failed to load wallet details. Please try again.');
                }
            } catch (error) {
                setError('Failed to load orders. Please try again.');
                console.error('Error loading orders:', error);
            } finally {
                setLoading(false);
            }
        };

        fetchData();
    }, [userId, refreshTrigger]);

    const getStatusClass = (status) => {
        switch (status) {
            case 'completed':
                return 'status-completed';
            case 'pending':
                return 'status-pending';
            default:
                return '';
        }
    };

    const formatDate = (dateString) => {
        if (!dateString) return '';
        const date = new Date(dateString);

        // Format as dd.MM.yyyy HH:mm:ss
        const day = date.getDate().toString().padStart(2, '0');
        const month = (date.getMonth() + 1).toString().padStart(2, '0');
        const year = date.getFullYear();
        const hours = date.getHours().toString().padStart(2, '0');
        const minutes = date.getMinutes().toString().padStart(2, '0');
        const seconds = date.getSeconds().toString().padStart(2, '0');

        return `${day}.${month}.${year} ${hours}:${minutes}:${seconds}`;
    };

    const truncateAddress = (address) => {
        if (!address || address === "Address unavailable") return address;

        // Keep first 15 and last 15 characters, add ellipsis in the middle
        const start = address.substring(0, 15);
        const end = address.substring(address.length - 15);
        return `${start}...${end}`;
    };

    const copyToClipboard = (text) => {
        navigator.clipboard.writeText(text)
            .then(() => {
                alert('Wallet address copied to clipboard!');
            })
            .catch(err => {
                console.error('Failed to copy text: ', err);
            });
    };

    return (
        <div className="orders-list-container">
            <h2>Your Orders</h2>
            {loading ? (
                <div className="loading">Loading orders...</div>
            ) : error ? (
                <div className="error-message">{error}</div>
            ) : orders.length === 0 ? (
                <div className="no-orders">No orders found. Create your first order!</div>
            ) : (
                <div className="orders-table-container">
                    <table className="orders-table">
                        <thead>
                            <tr>
                                <th style={{ width: '5%', minWidth: '40px' }}>ID</th>
                                <th style={{ width: '10%', minWidth: '80px' }}>Amount<br />(USDT)</th>
                                <th style={{ width: '40%', minWidth: '300px' }}>Wallet Address</th>
                                <th style={{ width: '10%', minWidth: '100px', textAlign: 'center' }}>Status</th>
                                <th style={{ width: '17.5%', minWidth: '140px' }}>Created</th>
                                <th style={{ width: '17.5%', minWidth: '140px' }}>Updated</th>
                            </tr>
                        </thead>
                        <tbody>
                            {orders.map((order) => {
                                // Get wallet address for this order
                                let walletAddress = "Address unavailable";

                                // Try to get the wallet address from our map
                                if (wallets[order.wallet_id]) {
                                    walletAddress = wallets[order.wallet_id];
                                }

                                // Get transaction ID for completed orders
                                const transactionId = order.status === 'completed' ? transactionIds[order.id] : null;

                                return (
                                    <tr key={order.id}>
                                        <td>{order.id}</td>
                                        <td>{order.amount}</td>
                                        <td>
                                            <div className="wallet-address">
                                                {walletAddress !== "Address unavailable" ? (
                                                    <>
                                                        <span className="address-text" title={walletAddress}>
                                                            {truncateAddress(walletAddress)}
                                                        </span>
                                                        <button
                                                            className="copy-button"
                                                            onClick={() => copyToClipboard(walletAddress)}
                                                            title="Copy full address"
                                                        >
                                                            📋
                                                        </button>
                                                    </>
                                                ) : (
                                                    "Address unavailable"
                                                )}
                                            </div>
                                        </td>
                                        <td>
                                            <span
                                                className={`status-badge ${getStatusClass(order.status)}`}
                                                title={order.status === 'completed' && transactionId ?
                                                    `Transaction ID: ${transactionId}` : ''}
                                            >
                                                {order.status}
                                            </span>
                                        </td>
                                        <td>{formatDate(order.created_at)}</td>
                                        <td>{formatDate(order.updated_at)}</td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
};

export default OrdersList; 