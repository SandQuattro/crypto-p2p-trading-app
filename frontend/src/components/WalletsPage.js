import React, {useState} from 'react';
import WalletsManagement from './WalletsManagement';
import '../App.css';

const WalletsPage = ({ lastPrice, symbol }) => {
    const [userId, setUserId] = useState('1');

    const handleUserIdChange = (e) => {
        setUserId(e.target.value);
    };

    return (
        <div className="wallets-page">
            <div className="user-selector">
                <label htmlFor="userIdSelectWallets">Select User ID: </label>
                <select
                    id="userIdSelectWallets"
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
                    }) : 'Загрузка...'}</p>
                </div>
            )}

            <div className="wallets-management-section">
                <WalletsManagement userId={userId} lastPrice={lastPrice} symbol={symbol} />
            </div>
        </div>
    );
};

export default WalletsPage; 