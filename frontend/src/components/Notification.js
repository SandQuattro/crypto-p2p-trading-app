// frontend/src/components/Notification.js
import React, {useEffect, useState} from 'react';
import PropTypes from 'prop-types';
import '../App.css'; // Убедимся, что стили подключены

const Notification = ({ message, type = 'info', duration = 5000, onClose }) => {
    const [isVisible, setIsVisible] = useState(false); // Start as not visible
    const [isClosing, setIsClosing] = useState(false); // State to manage closing transition

    // Effect for showing the notification
    useEffect(() => {
        // Trigger appear animation shortly after mount
        const appearTimeout = setTimeout(() => {
            setIsVisible(true);
        }, 50); // Small delay to ensure transition triggers

        return () => clearTimeout(appearTimeout);
    }, []);

    // Effect for hiding the notification
    useEffect(() => {
        if (!duration) return; // Don't auto-close if duration is 0 or null

        const timer = setTimeout(() => {
            closeNotification();
        }, duration);

        return () => clearTimeout(timer);
    }, [duration]);

    const closeNotification = () => {
        setIsClosing(true); // Start closing transition
        // Remove the component after the animation duration
        setTimeout(onClose, 300); // Matches CSS transition duration
    };

    // Combine visibility and closing state for className
    const notificationClass = `notification notification-${type} ${isVisible && !isClosing ? 'visible' : ''}`;

    return (
        <div className={notificationClass}>
            {message}
            <button onClick={closeNotification} className="notification-close-btn">&times;</button>
        </div>
    );
};

Notification.propTypes = {
    message: PropTypes.string.isRequired,
    type: PropTypes.oneOf(['info', 'warn', 'error']),
    duration: PropTypes.number,
    onClose: PropTypes.func.isRequired,
};

export default Notification; 