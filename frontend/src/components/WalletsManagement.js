import React, {useEffect, useState} from 'react';
import {deleteWallet, getWalletBalances, getWalletDetails, getWalletDetailsExtended} from '../services/api';
import {useNotification} from '../context/NotificationContext';
import QRCodeWithLogo from './QRCodeWithLogo';
import '../App.css';

const WalletsManagement = ({ userId, lastPrice, symbol }) => {
    const { addNotification } = useNotification();
    const [wallets, setWallets] = useState([]);
    const [loading, setLoading] = useState(true);
    const [isBalancesLoading, setIsBalancesLoading] = useState(false);
    const [error, setError] = useState('');
    const [balances, setBalances] = useState({});
    const [refreshTrigger, setRefreshTrigger] = useState(0);
    const [estimatedValues, setEstimatedValues] = useState({});
    const [qrVisibleFor, setQrVisibleFor] = useState({});

    // Обновляем рассчитанную стоимость при изменении последней цены
    useEffect(() => {
        if (lastPrice && symbol && Object.keys(balances).length > 0) {
            const newEstimatedValues = {};
            const cryptoCurrency = symbol.replace('RUB', '');

            Object.keys(balances).forEach(address => {
                const walletBalance = balances[address];
                if (walletBalance && walletBalance.token_balance_ether) {
                    // Предполагаем, что token_balance_ether это USDT баланс
                    const tokenBalance = parseFloat(walletBalance.token_balance_ether) || 0;

                    // Рассчитываем стоимость криптовалюты в USDT
                    const cryptoEstimatedValue = tokenBalance / lastPrice;
                    newEstimatedValues[address] = {
                        [cryptoCurrency]: cryptoEstimatedValue
                    };
                }
            });

            setEstimatedValues(newEstimatedValues);
        }
    }, [lastPrice, symbol, balances]);

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

        // Keep first and last 12 characters, add ellipsis in the middle
        const start = address.substring(0, 12);
        const end = address.substring(address.length - 12);
        return `${start}...${end}`;
    };

    // Add a new function to format balances nicely
    const formatBalance = (balanceStr, precision = 2) => {
        if (!balanceStr || balanceStr === '0') return precision === 2 ? '0.00' : '0.' + '0'.repeat(precision);

        // Try to parse the balance as a number
        const balanceNum = parseFloat(balanceStr);
        if (isNaN(balanceNum)) return precision === 2 ? '0.00' : '0.' + '0'.repeat(precision);

        // Format with specified decimal places
        return balanceNum.toFixed(precision);
    };

    const toggleQRCode = (walletId) => {
        setQrVisibleFor(prev => ({
            ...prev,
            [walletId]: !prev[walletId]
        }));
    };

    const handleDeleteWallet = async (walletId, walletAddress) => {
        // Проверяем баланс кошелька перед удалением
        const walletBalance = balances[walletAddress];
        const hasBalance = walletBalance &&
            (parseFloat(walletBalance.token_balance_ether) > 0 || parseFloat(walletBalance.bnb_balance_ether) > 0);

        if (hasBalance) {
            addNotification(
                `❌ НЕЛЬЗЯ удалить кошелек с балансом! USDT: ${walletBalance.token_balance_ether}, BNB: ${walletBalance.bnb_balance_ether}. Сначала переведите все средства!`,
                'error'
            );
            return;
        }

        if (!window.confirm(`🚨 КРИТИЧЕСКОЕ ПРЕДУПРЕЖДЕНИЕ! 🚨\n\nВы уверены, что хотите НАВСЕГДА удалить кошелек:\n${walletAddress}\n\n⚠️ ПОСЛЕ УДАЛЕНИЯ ВОССТАНОВИТЬ КОШЕЛЕК БУДЕТ НЕВОЗМОЖНО!\n⚠️ УБЕДИТЕСЬ, что баланс = 0.00 и нет активных заказов!\n\nПродолжить удаление?`)) {
            return;
        }

        try {
            setLoading(true);
            await deleteWallet(walletId);
            addNotification('✅ Кошелек успешно удален!', 'success');
            // Обновляем список кошельков
            setRefreshTrigger(prev => prev + 1);
        } catch (error) {
            console.error('Error deleting wallet:', error);
            addNotification(`❌ Ошибка при удалении кошелька: ${error.message}`, 'error');
        } finally {
            setLoading(false);
        }
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
                                <th style={{ width: '10%', minWidth: '80px' }}>User</th>
                                <th style={{ width: '25%', minWidth: '200px' }}>Wallet Address</th>
                                <th style={{ width: '10%', minWidth: '100px', textAlign: 'left' }}>USDT Balance</th>
                                <th style={{ width: '15%', minWidth: '120px', textAlign: 'left', paddingRight: '25px' }}>BNB Balance</th>
                                <th style={{ width: '8%', minWidth: '60px' }}>Testnet</th>
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

                                const estimatedValue = estimatedValues[wallet.address];
                                const cryptoEstimate = estimatedValue && symbol ? estimatedValue[symbol.replace('RUB', '')] : null;
                                const showQR = qrVisibleFor[wallet.id] || false;

                                return (
                                    <tr key={wallet.id}>
                                        <td className="user-id-cell">{wallet.user_id || userId}</td>
                                        <td>
                                            <div className="wallet-address">
                                                {showQR ? (
                                                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                                                        <QRCodeWithLogo
                                                            value={wallet.address}
                                                            size={140}
                                                            logoSize={35}
                                                            onClick={() => toggleQRCode(wallet.id)}
                                                        />
                                                        <button
                                                            style={{ marginTop: '5px', fontSize: '12px' }}
                                                            onClick={() => toggleQRCode(wallet.id)}
                                                            className="text-view-button"
                                                        >
                                                            Текстовый вид
                                                        </button>
                                                    </div>
                                                ) : (
                                                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                                                        <span
                                                            className="address-text"
                                                            title={wallet.address}
                                                            style={{ cursor: 'pointer' }}
                                                            onClick={() => copyToClipboard(wallet.address)}
                                                        >
                                                            {truncateAddress(wallet.address)}
                                                        </span>
                                                        <button
                                                            style={{ marginTop: '5px', fontSize: '12px' }}
                                                            onClick={() => toggleQRCode(wallet.id)}
                                                            className="qr-view-button"
                                                        >
                                                            QR-код
                                                        </button>
                                                    </div>
                                                )}
                                            </div>
                                        </td>
                                        <td className="balance-cell">{isBalancesLoading ? '...' : formatBalance(walletBalance.token_balance_ether)}</td>
                                        <td className="balance-cell">{isBalancesLoading ? '...' : formatBalance(walletBalance.bnb_balance_ether, 18)}</td>
                                        <td>{wallet.is_testnet ? 'Yes' : 'No'}</td>
                                        <td>{wallet.created_at ? formatDate(wallet.created_at) : 'N/A'}</td>
                                        <td>
                                            <div style={{ display: 'flex', gap: '5px', justifyContent: 'center' }}>
                                                <button
                                                    className="refresh-balance-button"
                                                    onClick={() => handleRefreshBalance(wallet.address)}
                                                    title="Update balance"
                                                    disabled={isBalancesLoading}
                                                >
                                                    {isBalancesLoading ? '...' : '🔄'}
                                                </button>
                                                {(() => {
                                                    const hasBalance = walletBalance &&
                                                        (parseFloat(walletBalance.token_balance_ether) > 0 || parseFloat(walletBalance.bnb_balance_ether) > 0);
                                                    const isDisabled = loading || isBalancesLoading || hasBalance;

                                                    return (
                                                        <button
                                                            className="delete-wallet-button"
                                                            onClick={() => handleDeleteWallet(wallet.id, wallet.address)}
                                                            title={hasBalance ?
                                                                "❌ Нельзя удалить кошелек с балансом! Сначала переведите все средства!" :
                                                                "🗑️ Удалить кошелек (только с нулевым балансом)"}
                                                            disabled={isDisabled}
                                                            style={{
                                                                backgroundColor: hasBalance ? '#6c757d' : '#dc3545',
                                                                color: 'white',
                                                                border: 'none',
                                                                borderRadius: '4px',
                                                                padding: '5px 8px',
                                                                cursor: isDisabled ? 'not-allowed' : 'pointer',
                                                                fontSize: '12px',
                                                                opacity: hasBalance ? 0.5 : 1
                                                            }}
                                                        >
                                                            {hasBalance ? '🔒' : '🗑️'}
                                                        </button>
                                                    );
                                                })()}
                                            </div>
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