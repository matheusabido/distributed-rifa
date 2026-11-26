import { useState } from 'react'
import { request } from '../api.js'
import { formatCurrency, formatNumber } from '../raffle.js'
import Modal from './Modal.jsx'

export default function CheckoutModal({ raffle, number, unavailable, onResult, onClose }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const label = formatNumber(number)

  async function submit(event) {
    event.preventDefault()
    if (busy || unavailable) return

    const values = new FormData(event.currentTarget)
    const nome = values.get('nome').trim()
    const contato = values.get('contato').trim()

    if (!nome || !contato) {
      setError('Informe seu nome e contato.')
      return
    }

    setBusy(true)
    setError('')

    try {
      await request(`/rifas/${raffle.id}/numeros/${number}/comprar`, {
        method: 'POST',
        body: { nome, contato },
      })
      setConfirmed(true)
      onResult(number)
    } catch (error) {
      setError(error.message)
      if (error.status === 409) {
        const numberSold = error.message.toLowerCase().includes('já vendido')
        onResult(numberSold ? number : null)
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal title={confirmed ? 'Compra confirmada' : `Comprar número ${label}`} onClose={onClose} busy={busy}>
      {confirmed ? (
        <div className="stacked-form">
          <p role="status">O número <strong>{label}</strong> foi comprado com sucesso.</p>
          <button type="button" className="button button--primary" onClick={onClose}>Voltar à rifa</button>
        </div>
      ) : (
        <form className="stacked-form" onSubmit={submit}>
          <div className="purchase-total">
            <span>Valor do número</span>
            <strong>{formatCurrency(raffle.preco_centavos)}</strong>
          </div>
          <label>
            Nome
            <input name="nome" autoComplete="name" required />
          </label>
          <label>
            Contato
            <input name="contato" placeholder="Telefone ou e-mail" required />
          </label>
          {(error || unavailable) && (
            <p className="feedback feedback--error" role="alert">{error || unavailable}</p>
          )}
          <button type="submit" className="button button--primary" disabled={busy || Boolean(unavailable)}>
            {busy ? 'Confirmando…' : 'Confirmar compra'}
          </button>
        </form>
      )}
    </Modal>
  )
}
