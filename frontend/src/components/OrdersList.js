import React, {useEffect, useState} from 'react';
import {deleteOrder, getTransactionIdForWallet, getUserOrders, getWalletDetails} from '../services/api';
import {useNotification} from '../context/NotificationContext';
import '../App.css';

const OrdersList = ({ userId, refreshTrigger }) => {
    const { addNotification } = useNotification();
    const [orders, setOrders] = useState([]);
    const [wallets, setWallets] = useState({});
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [transactionIds, setTransactionIds] = useState({});
    const [expandedOrders, setExpandedOrders] = useState({});

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

        // Keep first and last 8 characters, add ellipsis in the middle
        const start = address.substring(0, 8);
        const end = address.substring(address.length - 8);
        return `${start}...${end}`;
    };

    const copyToClipboard = (text) => {
        navigator.clipboard.writeText(text)
            .then(() => {
                addNotification('Wallet address copied to clipboard!', 'info');
            })
            .catch(err => {
                console.error('Failed to copy text: ', err);
                addNotification('Failed to copy wallet address.', 'error');
            });
    };

    // Функция для расчета срока действия заказа (3 часа от создания)
    const calculateExpiryDate = (createdAt) => {
        if (!createdAt) return '';
        const expiryDate = new Date(createdAt);
        expiryDate.setHours(expiryDate.getHours() + 3); // 3 часа от времени создания

        return formatDate(expiryDate.toISOString());
    };

    // Функция для переключения развернутого состояния ордера
    const toggleOrderExpand = (orderId) => {
        setExpandedOrders(prev => ({
            ...prev,
            [orderId]: !prev[orderId]
        }));
    };

    // Function to handle order deletion
    const handleDeleteOrder = async (orderId) => {
        if (!window.confirm("Are you sure you want to delete this pending order?")) {
            return;
        }

        try {
            await deleteOrder(orderId); // Call the API function
            // Refresh the list by removing the deleted order from state
            setOrders(prevOrders => prevOrders.filter(order => order.id !== orderId));
            // Optionally close the expanded view if it was open
            setExpandedOrders(prev => {
                const newExpanded = { ...prev };
                delete newExpanded[orderId];
                return newExpanded;
            });
            addNotification("Order deleted successfully.", 'info');
        } catch (error) {
            console.error('Error deleting order:', error);
            const errorMessage = `Failed to delete order ${orderId}. Please try again.`;
            setError(errorMessage);
            addNotification(errorMessage, 'error');
        }
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
                                <th style={{ width: '15%', minWidth: '100px' }}>Wallet Address</th>
                                <th style={{ width: '10%', minWidth: '100px', textAlign: 'center' }}>Status</th>
                                <th style={{ width: '10%', minWidth: '80px' }}>Created</th>
                                <th style={{ width: '10%', minWidth: '80px' }}>Updated</th>
                                <th style={{ width: '5%', minWidth: '60px' }}>Actions</th>
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
                                const isPending = order.status === 'pending';
                                const isExpanded = expandedOrders[order.id];

                                return (
                                    <React.Fragment key={order.id}>
                                        <tr
                                            className={isPending ? "pending-order" : ""}
                                            onClick={isPending ? () => toggleOrderExpand(order.id) : undefined}
                                            style={isPending ? { cursor: 'pointer' } : {}}
                                        >
                                            <td>{order.id}</td>
                                            <td>{order.amount}</td>
                                            <td>
                                                <div className="wallet-address">
                                                    {walletAddress !== "Address unavailable" ? (
                                                        <span
                                                            className="address-text"
                                                            title="Click to copy address"
                                                            onClick={(e) => {
                                                                e.stopPropagation();
                                                                copyToClipboard(walletAddress);
                                                            }}
                                                            style={{ cursor: 'pointer' }}
                                                        >
                                                            {truncateAddress(walletAddress)}
                                                        </span>
                                                    ) : (
                                                        "Address unavailable"
                                                    )}
                                                </div>
                                            </td>
                                            <td>
                                                <div className="status-container">
                                                    <span
                                                        className={`status-badge ${getStatusClass(order.status)}`}
                                                        title={order.status === 'completed' && transactionId ?
                                                            `Transaction ID: ${transactionId}` : ''}
                                                    >
                                                        {order.status}
                                                    </span>
                                                    {isPending && (
                                                        <span className="expand-toggle">
                                                            {isExpanded ? '▼' : '▶'}
                                                        </span>
                                                    )}
                                                </div>
                                            </td>
                                            <td>{formatDate(order.created_at)}</td>
                                            <td>{formatDate(order.updated_at)}</td>
                                            <td>
                                                {isPending && (
                                                    <button
                                                        className="delete-button"
                                                        onClick={(e) => {
                                                            e.stopPropagation();
                                                            handleDeleteOrder(order.id);
                                                        }}
                                                        title="Delete Order"
                                                    >
                                                        🗑️
                                                    </button>
                                                )}
                                            </td>
                                        </tr>

                                        {isPending && isExpanded && (
                                            <tr className="payment-details-row">
                                                <td colSpan="7">
                                                    <div className="payment-info-container">
                                                        <div className="wallet-address-container">
                                                            <span className="wallet-address-label">Payment address:</span>
                                                            <div className="wallet-address">
                                                                <span
                                                                    className="address-text"
                                                                    title="Click to copy address"
                                                                    onClick={() => copyToClipboard(walletAddress)}
                                                                    style={{ cursor: 'pointer' }}
                                                                >
                                                                    {walletAddress}
                                                                </span>
                                                            </div>
                                                        </div>

                                                        <div className="payment-notice warning">
                                                            <div className="notice-icon">⚠️</div>
                                                            <div className="notice-text">
                                                                Please note that we may block the payment and request additional information. You agree to provide us with such additional information or documents to comply with the request of us. In case of non-cooperation from your side, we will not be able to complete the payment or return it to you if you decide to proceed with a refund.
                                                            </div>
                                                        </div>

                                                        <div className="payment-notice warning">
                                                            <div className="notice-icon">⚠️</div>
                                                            <div className="notice-text">
                                                                The top-up will be successful if the full amount is paid in a single transaction. Be sure to consider the commission and payment currency when making the transfer.
                                                            </div>
                                                        </div>

                                                        <div className="payment-notice success">
                                                            <div className="notice-icon">👍</div>
                                                            <div className="notice-text">
                                                                Checks are credited to the balance automatically, usually within 10-30 minutes after the transaction is confirmed in blockchain.
                                                            </div>
                                                        </div>

                                                        <div className="payment-notice expiry">
                                                            <div className="notice-icon">⏰</div>
                                                            <div className="notice-text">
                                                                Expires on {calculateExpiryDate(order.created_at)}
                                                            </div>
                                                        </div>
                                                    </div>
                                                </td>
                                            </tr>
                                        )}
                                    </React.Fragment>
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