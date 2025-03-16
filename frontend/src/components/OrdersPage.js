import React, {useState} from 'react';
import OrderForm from './OrderForm';
import OrdersList from './OrdersList';
import '../App.css';

const OrdersPage = () => {
    const [userId, setUserId] = useState('1');
    const [refreshTrigger, setRefreshTrigger] = useState(0);

    const handleOrderCreated = () => {
        // Trigger a refresh of the orders list
        setRefreshTrigger(prev => prev + 1);
    };

    const handleUserIdChange = (e) => {
        setUserId(e.target.value);
    };

    return (
        <div className="orders-page">
            <div className="user-selector">
                <label htmlFor="userIdSelect">Select User ID: </label>
                <select
                    id="userIdSelect"
                    value={userId}
                    onChange={handleUserIdChange}
                    className="user-select"
                >
                    <option value="1">User 1</option>
                    <option value="2">User 2</option>
                    <option value="3">User 3</option>
                </select>
            </div>

            <div className="orders-layout">
                <div className="order-form-section">
                    <OrderForm onOrderCreated={handleOrderCreated} />
                </div>
                <div className="orders-list-section">
                    <OrdersList userId={userId} refreshTrigger={refreshTrigger} />
                </div>
            </div>
        </div>
    );
};

export default OrdersPage; 