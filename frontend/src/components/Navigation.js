import React from 'react';
import '../App.css';

const Navigation = ({ activeTab, onTabChange }) => {
    return (
        <div className="navigation">
            <button
                className={`nav-button ${activeTab === 'trading' ? 'active' : ''}`}
                onClick={() => onTabChange('trading')}
            >
                Trading Charts
            </button>
            <button
                className={`nav-button ${activeTab === 'orders' ? 'active' : ''}`}
                onClick={() => onTabChange('orders')}
            >
                Orders Management
            </button>
        </div>
    );
};

export default Navigation; 