import React from 'react';
import '../App.css';

const Modal = ({ isOpen, onClose, title, children }) => {
    if (!isOpen) return null;

    // Предотвращаем закрытие при клике внутри модального окна
    const handleModalContentClick = (e) => {
        e.stopPropagation();
    };

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal-container" onClick={handleModalContentClick}>
                <div className="modal-header">
                    <h2>{title}</h2>
                    <button className="modal-close-button" onClick={onClose}>×</button>
                </div>
                <div className="modal-content">
                    {children}
                </div>
            </div>
        </div>
    );
};

export default Modal; 