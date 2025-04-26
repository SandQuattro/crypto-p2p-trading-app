// frontend/src/context/NotificationContext.js
import React, {createContext, useCallback, useContext, useState} from 'react';

const NotificationContext = createContext();

let idCounter = 0;

export const NotificationProvider = ({ children }) => {
    const [notifications, setNotifications] = useState([]);

    const addNotification = useCallback((message, type = 'info', duration = 5000) => {
        const id = idCounter++;
        setNotifications((prevNotifications) => [
            ...prevNotifications,
            { id, message, type, duration },
        ]);
    }, []);

    const removeNotification = useCallback((id) => {
        setNotifications((prevNotifications) =>
            prevNotifications.filter((notification) => notification.id !== id)
        );
    }, []);

    return (
        <NotificationContext.Provider value={{ addNotification, removeNotification, notifications }}>
            {children}
        </NotificationContext.Provider>
    );
};

export const useNotification = () => {
    const context = useContext(NotificationContext);
    if (!context) {
        throw new Error('useNotification must be used within a NotificationProvider');
    }
    return context;
}; 