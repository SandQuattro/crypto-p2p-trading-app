import React, {useState} from 'react';
import {createOrder} from '../services/api';
import '../App.css';

const OrderForm = ({ onOrderCreated }) => {
    const [userId, setUserId] = useState('1');
    const [amount, setAmount] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [error, setError] = useState('');
    const [successMessage, setSuccessMessage] = useState('');

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setSuccessMessage('');
        setIsSubmitting(true);

        try {
            const response = await createOrder(userId, amount);
            setSuccessMessage(`Order created successfully! Wallet: ${response.wallet}`);
            setAmount('');

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
            {successMessage && <div className="success-message">{successMessage}</div>}
        </div>
    );
};

export default OrderForm; 