import { useState } from 'react'
import Modal from './Modal.jsx'

export default function RaffleEditor({ raffle, onSave, onClose }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const canChangeTotal = !raffle || raffle.status === 'draft'

  async function submit(event) {
    event.preventDefault()
    if (busy) return

    const values = new FormData(event.currentTarget)
    const titulo = values.get('titulo').trim()
    const premio = values.get('premio').trim()
    const priceCents = Math.round(Number(values.get('preco')) * 100)

    if (!titulo || !premio || !Number.isSafeInteger(priceCents) || priceCents < 0) {
      setError('Informe título, prêmio e um valor válido.')
      return
    }

    const body = {
      titulo,
      premio,
      descricao: values.get('descricao').trim(),
      preco_centavos: priceCents,
    }

    if (canChangeTotal) body.total_numeros = Number(values.get('total_numeros'))
    setBusy(true)
    setError('')

    try {
      await onSave(body)
    } catch (error) {
      setError(error.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal title={raffle ? 'Editar rifa' : 'Criar rifa'} onClose={onClose} busy={busy}>
      <form className="stacked-form" onSubmit={submit}>
        <label>
          Título
          <input name="titulo" defaultValue={raffle?.titulo} required />
        </label>
        <label>
          Descrição
          <textarea name="descricao" rows="3" defaultValue={raffle?.descricao} />
        </label>
        <label>
          Prêmio
          <input name="premio" defaultValue={raffle?.premio} required />
        </label>
        <label>
          Valor por número (R$)
          <input
            name="preco"
            type="number"
            min="0"
            step="0.01"
            defaultValue={raffle ? raffle.preco_centavos / 100 : ''}
            required
          />
        </label>
        {canChangeTotal && (
          <label>
            Total de números
            <input name="total_numeros" type="number" min="1" max="2147483647" defaultValue={raffle?.total_numeros ?? 200} required />
          </label>
        )}
        {error && <p className="feedback feedback--error" role="alert">{error}</p>}
        <button type="submit" className="button button--primary" disabled={busy}>
          {busy ? 'Salvando…' : 'Salvar rifa'}
        </button>
      </form>
    </Modal>
  )
}
