import React, {useEffect, useState} from 'react';
import {getUserOrders, getWalletDetails} from '../services/api';
import '../App.css';

const OrdersList = ({ userId, refreshTrigger }) => {
    const [orders, setOrders] = useState([]);
    const [wallets, setWallets] = useState({});
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');

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
                                <th>ID</th>
                                <th>Amount (USDT)</th>
                                <th>Wallet Address</th>
                                <th>Status</th>
                                <th>Created</th>
                                <th>Updated</th>
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

                                return (
                                    <tr key={order.id}>
                                        <td>{order.id}</td>
                                        <td>{order.amount}</td>
                                        <td className="wallet-address">
                                            {walletAddress !== "Address unavailable" ? (
                                                <>
                                                    <span className="address-text">{`${walletAddress.substring(0, 8)}...${walletAddress.substring(walletAddress.length - 6)}`}</span>
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
                                        </td>
                                        <td>
                                            <span className={`status-badge ${getStatusClass(order.status)}`}>
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