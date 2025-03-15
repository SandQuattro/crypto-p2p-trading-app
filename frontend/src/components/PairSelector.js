import React from 'react';

const PairSelector = ({ pairs, selectedPair, onSelectPair }) => {
  return (
    <div className="trading-pairs">
      {pairs.map(pair => (
        <button
          key={pair}
          className={`pair-button ${selectedPair === pair ? 'active' : ''}`}
          onClick={() => onSelectPair(pair)}
        >
          {pair}
        </button>
      ))}
    </div>
  );
};

export default PairSelector;
