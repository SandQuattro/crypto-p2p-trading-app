import React, {useState} from 'react';
import OrderForm from './OrderForm';
import OrdersList from './OrdersList';
import Modal from './Modal';
import '../App.css';

const OrdersPage = ({ lastPrice, symbol }) => {
    const [userId, setUserId] = useState('1');
    const [refreshTrigger, setRefreshTrigger] = useState(0);
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [orderCreated, setOrderCreated] = useState(false);

    const handleOrderCreated = () => {
        // Trigger a refresh of the orders list
        setRefreshTrigger(prev => prev + 1);
        // Отмечаем, что заказ создан и закрываем модальное окно
        setOrderCreated(true);
        setIsModalOpen(false);
    };

    const handleUserIdChange = (e) => {
        setUserId(e.target.value);
    };

    const openModal = () => {
        setIsModalOpen(true);
        setOrderCreated(false); // Сбрасываем флаг при открытии модального окна
    };

    const closeModal = () => {
        setIsModalOpen(false);
        // Не сбрасываем orderCreated при закрытии, чтобы информация сохранялась при повторном открытии
    };

    const resetAndCloseModal = () => {
        setIsModalOpen(false);
        setOrderCreated(false); // Сбрасываем флаг только когда пользователь закрывает окно после просмотра информации
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
                    <option value="123">User 123</option>
                    <option value="456">User 456</option>
                </select>
            </div>

            {symbol && (
                <div className="current-market-info">
                    <p>Current Price for {symbol}: {lastPrice ? lastPrice.toLocaleString('en-US', {
                        minimumFractionDigits: 2,
                        maximumFractionDigits: 2
                    }) : 'Loading...'}</p>
                </div>
            )}

            <button className="add-order-button" onClick={openModal}>
                Create New Order
            </button>

            <div className="orders-list-section">
                <OrdersList userId={userId} refreshTrigger={refreshTrigger} />
            </div>

            <Modal
                isOpen={isModalOpen}
                onClose={orderCreated ? resetAndCloseModal : closeModal}
                title={orderCreated ? "Payment Information" : "Create New Order"}
            >
                <OrderForm
                    onOrderCreated={handleOrderCreated}
                    selectedUserId={userId}
                    orderCreated={orderCreated}
                    currentPrice={lastPrice}
                    currentSymbol={symbol}
                />
                {orderCreated && (
                    <div className="modal-footer">
                        <button className="close-button" onClick={resetAndCloseModal}>
                            Close
                        </button>
                    </div>
                )}
            </Modal>
        </div>
    );
};

export default OrdersPage; 