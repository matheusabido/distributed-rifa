import { formatCurrency, formatSelectionCount, PRICE } from '../raffle.js'

export default function MobileCheckoutBar({ count, onContinue }) {
  if (count === 0) {
    return null
  }

  return (
    <div className="mobile-checkout-bar">
      <div>
        <small>{formatSelectionCount(count)}</small>
        <strong>{formatCurrency(count * PRICE)}</strong>
      </div>
      <button type="button" onClick={onContinue}>
        Continuar
      </button>
    </div>
  )
}
