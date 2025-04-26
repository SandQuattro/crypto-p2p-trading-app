import React, {useEffect, useState} from 'react';
import {getWalletBalances, getWalletDetails, getWalletDetailsExtended} from '../services/api';
import {useNotification} from '../context/NotificationContext';
import '../App.css';

const WalletsManagement = ({ userId }) => {
    const { addNotification } = useNotification();
    const [wallets, setWallets] = useState([]);
    const [loading, setLoading] = useState(true);
    const [isBalancesLoading, setIsBalancesLoading] = useState(false);
    const [error, setError] = useState('');
    const [balances, setBalances] = useState({});
    const [refreshTrigger, setRefreshTrigger] = useState(0);

    const loadBalances = async () => {
        setIsBalancesLoading(true);
        try {
            await fetchWalletBalances(userId);
        } catch (balanceError) {
            console.error("Error initiating balance fetch:", balanceError);
            setError(prevError => prevError || 'Failed to load balances.');
        } finally {
            setIsBalancesLoading(false);
        }
    };

    useEffect(() => {
        const fetchWallets = async () => {
            setLoading(true);
            setError('');
            setWallets([]);
            setBalances({});
            setIsBalancesLoading(false);

            let fetchedWallets = [];
            try {
                try {
                    fetchedWallets = await getWalletDetailsExtended(userId);
                } catch (extendedError) {
                    console.warn('Extended wallet details not available, falling back to basic details');
                    fetchedWallets = await getWalletDetails(userId);
                }

                setWallets(fetchedWallets || []);
                setLoading(false);

                if (fetchedWallets && fetchedWallets.length > 0) {
                    await loadBalances();
                }
            } catch (error) {
                console.error('Error fetching wallets:', error);
                setError('Failed to load wallet details. Please try again.');
                setLoading(false);
            }
        };

        fetchWallets();
    }, [userId, refreshTrigger]);

    const fetchWalletBalances = async (currentUserId) => {
        try {
            const data = await getWalletBalances(currentUserId);
            setBalances(data);
        } catch (error) {
            console.error('Error fetching wallet balances:', error);
            setError('Failed to load wallet balances. Please try again.');
        }
    };

    const handleRefreshBalance = async (address) => {
        await loadBalances();
    };

    const handleRefreshAll = () => {
        setRefreshTrigger(prev => prev + 1);
    };

    const formatDate = (dateString) => {
        if (!dateString) return '';
        const date = new Date(dateString);

        // Format as dd.MM.yyyy HH:mm
        const day = date.getDate().toString().padStart(2, '0');
        const month = (date.getMonth() + 1).toString().padStart(2, '0');
        const year = date.getFullYear();
        const hours = date.getHours().toString().padStart(2, '0');
        const minutes = date.getMinutes().toString().padStart(2, '0');

        return `${day}.${month}.${year} ${hours}:${minutes}`;
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

    const truncateAddress = (address) => {
        if (!address) return '';

        // Keep first 15 and last 15 characters, add ellipsis in the middle
        const start = address.substring(0, 15);
        const end = address.substring(address.length - 15);
        return `${start}...${end}`;
    };

    // Add a new function to format balances nicely
    const formatBalance = (balanceStr) => {
        if (!balanceStr || balanceStr === '0') return '0.00';

        // Try to parse the balance as a number
        const balanceNum = parseFloat(balanceStr);
        if (isNaN(balanceNum)) return '0.00';

        // Format with 2 decimal places
        return balanceNum.toFixed(2);
    };

    return (
        <div className="wallets-management-container">
            <h2>Wallets Management</h2>
            <button
                className="refresh-all-button"
                onClick={handleRefreshAll}
                disabled={loading || isBalancesLoading}
            >
                {loading ? 'Loading Wallets...' : isBalancesLoading ? 'Refreshing Balances...' : 'Refresh All Wallets'}
            </button>

            {loading ? (
                <div className="loading">Loading wallets...</div>
            ) : error ? (
                <div className="error-message">{error}</div>
            ) : wallets.length === 0 ? (
                <div className="no-wallets">No wallets found.</div>
            ) : (
                <div className="wallets-table-container">
                    <table className="wallets-table">
                        <thead>
                            <tr>
                                <th style={{ width: '5%', minWidth: '40px' }}>ID</th>
                                <th style={{ width: '10%', minWidth: '80px' }}>User</th>
                                <th style={{ width: '35%', minWidth: '280px' }}>Wallet Address</th>
                                <th style={{ width: '12.5%', minWidth: '100px', textAlign: 'right', paddingRight: '25px' }}>USDT Balance</th>
                                <th style={{ width: '12.5%', minWidth: '100px', textAlign: 'right', paddingRight: '25px' }}>BNB Balance</th>
                                <th style={{ width: '15%', minWidth: '140px' }}>Created Date</th>
                                <th style={{ width: '10%', minWidth: '100px' }}>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {wallets.map((wallet) => {
                                const walletBalance = balances[wallet.address] || {
                                    token_balance_ether: '0',
                                    bnb_balance_ether: '0',
                                    last_checked: '-'
                                };

                                return (
                                    <tr key={wallet.id}>
                                        <td>{wallet.id}</td>
                                        <td className="user-id-cell">{wallet.user_id || userId}</td>
                                        <td>
                                            <div className="wallet-address">
                                                <span
                                                    className="address-text"
                                                    title={wallet.address}
                                                    style={{ cursor: 'pointer' }}
                                                    onClick={() => copyToClipboard(wallet.address)}
                                                >
                                                    {truncateAddress(wallet.address)}
                                                </span>
                                            </div>
                                        </td>
                                        <td className="balance-cell">{isBalancesLoading ? '...' : formatBalance(walletBalance.token_balance_ether)}</td>
                                        <td className="balance-cell">{isBalancesLoading ? '...' : formatBalance(walletBalance.bnb_balance_ether)}</td>
                                        <td>{wallet.created_at ? formatDate(wallet.created_at) : 'N/A'}</td>
                                        <td>
                                            <button
                                                className="refresh-balance-button"
                                                onClick={() => handleRefreshBalance(wallet.address)}
                                                title="Update balance"
                                                disabled={isBalancesLoading}
                                            >
                                                {isBalancesLoading ? '...' : '🔄'}
                                            </button>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            )}
            {Object.keys(balances).length > 0 && !isBalancesLoading && (
                <div className="last-updated">
                    Balances updated: {new Date().toLocaleString()}
                </div>
            )}
            {wallets.length > 0 && !wallets[0].created_at && !loading && (
                <div className="note-message">
                    Note: Creation dates are not available from the backend. Please update the backend API to include this information.
                </div>
            )}
        </div>
    );
};

export default WalletsManagement; 