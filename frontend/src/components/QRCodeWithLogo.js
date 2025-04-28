import React from 'react';
import {QRCodeSVG} from 'qrcode.react';

const QRCodeWithLogo = ({ value, size = 150, logoSize = 40, onClick }) => {
    const logoWidth = logoSize;
    const logoHeight = logoSize;

    return (
        <div style={{ cursor: 'pointer' }} onClick={onClick}>
            <QRCodeSVG
                value={value}
                size={size}
                level="H"
                includeMargin={true}
                imageSettings={{
                    src: '/assets/img/bnb-logo.svg',
                    excavate: true,
                    width: logoWidth,
                    height: logoHeight
                }}
            />
        </div>
    );
};

export default QRCodeWithLogo; 