import React, {useState} from 'react';
import {createOrder} from '../services/api';
import '../App.css';

const OrderForm = ({ onOrderCreated }) => {
    const [userId, setUserId] = useState('1');
    const [amount, setAmount] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');

    const handleSubmit = async (e) => {
        e.preventDefault();
        setLoading(true);
        setError('');
        setSuccess('');

        try {
            const response = await createOrder(userId, amount);
            setSuccess(`Order created successfully! Wallet: ${response.wallet}`);
            setAmount('');
            if (onOrderCreated) {
                onOrderCreated();
            }
        } catch (error) {
            setError('Failed to create order. Please try again.');
            console.error('Order creation error:', error);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="order-form-container">
            <h2>Create New Order</h2>
            <form onSubmit={handleSubmit} className="order-form">
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
                        step="0.01"
                        min="0.01"
                        required
                        className="form-control"
                        placeholder="Enter amount"
                    />
                </div>
                <button type="submit" className="submit-button" disabled={loading}>
                    {loading ? 'Creating...' : 'Create Order'}
                </button>
                {error && <div className="error-message">{error}</div>}
                {success && <div className="success-message">{success}</div>}
            </form>
        </div>
    );
};

export default OrderForm; 