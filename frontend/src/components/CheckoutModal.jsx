import { useState } from 'react'
import { formatCurrency, formatNumber, PRICE } from '../raffle.js'
import Icon from './Icon.jsx'

function CheckoutConfirmation({ selected, onClose }) {
  return (
    <div className="confirmation">
      <span className="confirmation__icon">
        <Icon name="check" size={34} />
      </span>
      <span className="section-kicker">Simulação concluída</span>
      <h2 id="checkout-title">Números reservados!</h2>
      <p>Em uma versão integrada, você receberia agora as instruções de pagamento.</p>
      <div className="confirmation__numbers">
        {selected.map(formatNumber).join(', ')}
      </div>
      <button
        type="button"
        className="button button--primary button--full"
        onClick={onClose}
      >
        Voltar para a rifa
      </button>
    </div>
  )
}

export default function CheckoutModal({ selected, onClose }) {
  const [isConfirmed, setIsConfirmed] = useState(false)

  function handleBackdropClick(event) {
    if (event.target === event.currentTarget) {
      onClose()
    }
  }

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={handleBackdropClick}>
      <div
        className="checkout-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="checkout-title"
      >
        <button type="button" className="modal-close" onClick={onClose} aria-label="Fechar">
          <Icon name="close" />
        </button>

        {isConfirmed ? (
          <CheckoutConfirmation selected={selected} onClose={onClose} />
        ) : (
          <>
            <span className="section-kicker">Quase lá</span>
            <h2 id="checkout-title">Revise sua escolha</h2>
            <p className="modal-intro">
              Esta é uma demonstração visual. Nenhuma cobrança ou reserva real será feita.
            </p>

            <div className="modal-numbers">
              {selected.map((number) => (
                <span key={number}>{formatNumber(number)}</span>
              ))}
            </div>
            <div className="modal-total">
              <span>Total da contribuição</span>
              <strong>{formatCurrency(selected.length * PRICE)}</strong>
            </div>

            <button
              type="button"
              className="button button--primary button--full"
              onClick={() => setIsConfirmed(true)}
            >
              Simular reserva
            </button>
            <button
              type="button"
              className="button button--ghost button--full"
              onClick={onClose}
            >
              Escolher mais números
            </button>
          </>
        )}
      </div>
    </div>
  )
}
