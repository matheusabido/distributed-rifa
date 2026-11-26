import { useState } from 'react'
import { request } from '../api.js'
import { formatCurrency, formatNumber } from '../raffle.js'
import Modal from './Modal.jsx'

const buyerEmailKey = 'rifa.buyerEmail'

function readBuyerEmail() {
  try {
    return sessionStorage.getItem(buyerEmailKey) || ''
  } catch {
    return ''
  }
}

// A API exige "nome", que aparece publicamente na grade e nos eventos.
// Usamos só a parte antes do @ para não expor o e-mail completo.
function publicName(email) {
  return email.split('@')[0] || email
}

export default function CheckoutModal({ raffle, numbers, accountEmail, unavailable, onResult, onClose }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [bought, setBought] = useState(null)
  const [guestEmail, setGuestEmail] = useState(readBuyerEmail)
  const [editingEmail, setEditingEmail] = useState(() => !accountEmail && !readBuyerEmail())
  const buyerEmail = accountEmail || guestEmail
  const sorted = [...numbers].sort((a, b) => a - b)
  const labels = sorted.map(formatNumber).join(', ')
  const plural = sorted.length > 1

  async function submit(event) {
    event.preventDefault()
    if (busy || unavailable || sorted.length === 0) return

    let email = buyerEmail
    if (editingEmail) {
      email = new FormData(event.currentTarget).get('email').trim()
      if (!email) {
        setError('Informe seu e-mail.')
        return
      }
      sessionStorage.setItem(buyerEmailKey, email)
      setGuestEmail(email)
      setEditingEmail(false)
    }

    setBusy(true)
    setError('')

    const body = { nome: publicName(email), contato: email }
    const results = await Promise.allSettled(
      sorted.map((number) =>
        request(`/rifas/${raffle.id}/numeros/${number}/comprar`, { method: 'POST', body }),
      ),
    )

    const succeeded = []
    const soldByOthers = []
    const failures = []
    results.forEach((result, index) => {
      const number = sorted[index]
      if (result.status === 'fulfilled') {
        succeeded.push(number)
      } else if (result.reason?.status === 409 && result.reason.message.toLowerCase().includes('já vendido')) {
        soldByOthers.push(number)
      } else {
        failures.push(result.reason?.message || 'Não foi possível concluir a compra.')
      }
    })

    onResult({ bought: succeeded, sold: soldByOthers })

    if (succeeded.length > 0 && soldByOthers.length === 0 && failures.length === 0) {
      setBought(succeeded)
    } else {
      const parts = []
      if (succeeded.length) parts.push(`Comprados: ${succeeded.map(formatNumber).join(', ')}.`)
      if (soldByOthers.length) parts.push(`Já vendidos para outra pessoa: ${soldByOthers.map(formatNumber).join(', ')}.`)
      if (failures.length) parts.push([...new Set(failures)].join(' '))
      setError(parts.join(' '))
    }

    setBusy(false)
  }

  if (bought) {
    return (
      <Modal title="Compra confirmada" onClose={onClose}>
        <div className="stacked-form">
          <p role="status">
            {bought.length > 1 ? 'Os números ' : 'O número '}
            <strong>{bought.map(formatNumber).join(', ')}</strong>
            {bought.length > 1 ? ' foram comprados' : ' foi comprado'} com sucesso.
          </p>
          <button type="button" className="button button--primary" onClick={onClose}>Voltar à rifa</button>
        </div>
      </Modal>
    )
  }

  return (
    <Modal
      title={plural ? `Comprar ${sorted.length} números` : `Comprar número ${labels}`}
      onClose={onClose}
      busy={busy}
    >
      <form className="stacked-form" onSubmit={submit}>
        {plural && <p>Números: <strong>{labels}</strong></p>}
        <div className="purchase-total">
          <span>{plural ? `Total (${sorted.length} × ${formatCurrency(raffle.preco_centavos)})` : 'Valor do número'}</span>
          <strong>{formatCurrency(raffle.preco_centavos * sorted.length)}</strong>
        </div>

        {editingEmail ? (
          <label>
            E-mail
            <input name="email" type="email" autoComplete="email" defaultValue={guestEmail} required autoFocus />
          </label>
        ) : (
          <p className="buyer-identity">
            Comprando como <strong>{buyerEmail}</strong>
            {!accountEmail && (
              <button type="button" className="link-button" onClick={() => setEditingEmail(true)} disabled={busy}>
                Trocar
              </button>
            )}
          </p>
        )}

        {(error || unavailable) && (
          <p className="feedback feedback--error" role="alert">{error || unavailable}</p>
        )}
        <button
          type="submit"
          className="button button--primary"
          disabled={busy || Boolean(unavailable) || sorted.length === 0}
        >
          {busy ? 'Confirmando…' : 'Confirmar compra'}
        </button>
      </form>
    </Modal>
  )
}
