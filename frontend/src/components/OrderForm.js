import React, {useEffect, useState} from 'react';
import {createOrder} from '../services/api';
import {useNotification} from '../context/NotificationContext';
import '../App.css';

// Константа для срока действия заказа в часах
const ORDER_EXPIRY_HOURS = 3;

const OrderForm = ({ onOrderCreated, selectedUserId, orderCreated }) => {
    const { addNotification } = useNotification();
    const [userId, setUserId] = useState(selectedUserId || '1');
    const [amount, setAmount] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [error, setError] = useState('');
    const [successMessage, setSuccessMessage] = useState('');
    const [orderWallet, setOrderWallet] = useState('');
    const [showPaymentInfo, setShowPaymentInfo] = useState(false);
    const [expiryDate, setExpiryDate] = useState('');

    // Для сохранения информации о платеже после успешного создания заказа
    const [paymentInfo, setPaymentInfo] = useState(null);

    // Update userId when selectedUserId prop changes
    useEffect(() => {
        if (selectedUserId) {
            setUserId(selectedUserId);
        }
    }, [selectedUserId]);

    // Отслеживаем изменение orderCreated и устанавливаем showPaymentInfo соответственно
    useEffect(() => {
        if (orderCreated && paymentInfo) {
            setOrderWallet(paymentInfo.wallet);
            setExpiryDate(paymentInfo.expiryDate);
            setShowPaymentInfo(true);
            setSuccessMessage(`Order created successfully! Wallet: ${paymentInfo.wallet}`);
        } else if (orderCreated && !orderWallet && !paymentInfo) {
            // Если orderCreated=true, но информации о кошельке нет,
            // создаем "заглушку" с сообщением об ошибке
            setError('Payment information not available. Please create a new order.');
            setShowPaymentInfo(false);
        }
    }, [orderCreated, paymentInfo, orderWallet]);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setSuccessMessage('');
        setShowPaymentInfo(false);
        setIsSubmitting(true);

        try {
            const response = await createOrder(userId, amount);

            // Устанавливаем срок действия
            const expiry = new Date();
            expiry.setHours(expiry.getHours() + ORDER_EXPIRY_HOURS);
            const formattedExpiryDate = expiry.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            });

            // Сохраняем информацию о платеже
            setPaymentInfo({
                wallet: response.wallet,
                expiryDate: formattedExpiryDate
            });

            setOrderWallet(response.wallet);
            setExpiryDate(formattedExpiryDate);
            setAmount('');
            setShowPaymentInfo(true);
            setSuccessMessage(`Order created successfully! Wallet: ${response.wallet}`);

            // Notify parent component that order was created
            if (onOrderCreated) {
                onOrderCreated();
            }
        } catch (error) {
            setError('Failed to create order. Please try again.');
            console.error('Error creating order:', error);
        } finally {
            setIsSubmitting(false);
        }
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

    // Если заказ был создан и пришел флаг orderCreated или есть сохранённая платёжная информация и showPaymentInfo активен, показываем только информацию о платеже
    if ((orderCreated || showPaymentInfo) && orderWallet) {
        return (
            <div className="order-form">
                {successMessage && <div className="success-message">{successMessage}</div>}
                <div className="payment-info-container">
                    <div className="wallet-address-container">
                        <span className="wallet-address-label">Payment address:</span>
                        <div className="wallet-address">
                            <span className="address-text">{orderWallet}</span>
                            <button
                                className="copy-button"
                                onClick={() => copyToClipboard(orderWallet)}
                                title="Copy full address"
                            >
                                📋
                            </button>
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
                            Заказ действителен в течение {ORDER_EXPIRY_HOURS} часов (до {expiryDate})
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="order-form">
            <form onSubmit={handleSubmit}>
                <div className="form-group">
                    <label htmlFor="userId">User ID:</label>
                    <input
                        type="text"
                        id="userId"
                        value={userId}
                        onChange={(e) => setUserId(e.target.value)}
                        required
                        className="form-control"
                        disabled={!!selectedUserId} // Disable editing if user ID is provided from parent
                    />
                </div>
                <div className="form-group">
                    <label htmlFor="amount">Amount (USDT):</label>
                    <input
                        type="number"
                        id="amount"
                        value={amount}
                        onChange={(e) => setAmount(e.target.value)}
                        placeholder="Enter amount"
                        required
                        min="0.1"
                        step="0.1"
                        className="form-control"
                    />
                </div>
                <button
                    type="submit"
                    className="submit-button"
                    disabled={isSubmitting}
                >
                    {isSubmitting ? 'Creating...' : 'Create Order'}
                </button>
            </form>

            {error && <div className="error-message">{error}</div>}

            <div className="payment-info-container">
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
                        После создания заказ будет действителен в течение {ORDER_EXPIRY_HOURS} часов
                    </div>
                </div>
            </div>
        </div>
    );
};

export default OrderForm; 