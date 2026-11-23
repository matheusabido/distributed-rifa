import { formatCurrency, formatNumber, formatSelectionCount, PRICE } from '../raffle.js'
import Icon from './Icon.jsx'

export default function SelectionSummary({ selected, onContinue }) {
  const count = selected.length

  return (
    <aside className="summary-card" aria-label="Resumo da seleção">
      <div className="summary-card__top">
        <span className="summary-icon">
          <Icon name="ticket" size={24} />
        </span>
        <div>
          <small>Seu resumo</small>
          <h3>{count ? formatSelectionCount(count) : 'Nenhum número'}</h3>
        </div>
      </div>

      <div className={count ? 'selected-list' : 'selected-list empty'}>
        {count > 0 ? (
          selected.map((number) => <span key={number}>{formatNumber(number)}</span>)
        ) : (
          <p>Seus números escolhidos aparecerão aqui.</p>
        )}
      </div>

      <div className="summary-row">
        <span>Valor por número</span>
        <strong>{formatCurrency(PRICE)}</strong>
      </div>
      <div className="summary-total">
        <span>Total</span>
        <strong>{formatCurrency(count * PRICE)}</strong>
      </div>

      <button
        type="button"
        className="button button--primary button--full"
        disabled={count === 0}
        onClick={onContinue}
      >
        Continuar
      </button>
      <p className="secure-note">
        <Icon name="shield" size={15} /> Reserva segura por 15 minutos
      </p>
    </aside>
  )
}
