import React, {useState} from 'react';
import OrderForm from './OrderForm';
import OrdersList from './OrdersList';
import Modal from './Modal';
import '../App.css';

const OrdersPage = () => {
    const [userId, setUserId] = useState('1');
    const [refreshTrigger, setRefreshTrigger] = useState(0);
    const [isModalOpen, setIsModalOpen] = useState(false);

    const handleOrderCreated = () => {
        // Trigger a refresh of the orders list
        setRefreshTrigger(prev => prev + 1);
        // Close the modal after order creation
        setIsModalOpen(false);
    };

    const handleUserIdChange = (e) => {
        setUserId(e.target.value);
    };

    const openModal = () => {
        setIsModalOpen(true);
    };

    const closeModal = () => {
        setIsModalOpen(false);
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

            <button className="add-order-button" onClick={openModal}>
                Create New Order
            </button>

            <div className="orders-list-section">
                <OrdersList userId={userId} refreshTrigger={refreshTrigger} />
            </div>

            <Modal isOpen={isModalOpen} onClose={closeModal} title="Create New Order">
                <OrderForm onOrderCreated={handleOrderCreated} selectedUserId={userId} />
            </Modal>
        </div>
    );
};

export default OrdersPage; 