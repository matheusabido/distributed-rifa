import { useState } from 'react'
import { request } from '../api.js'
import Modal from './Modal.jsx'

export default function AuthModal({ initialMode, message, onClose, onAuthenticated }) {
  const [mode, setMode] = useState(initialMode)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const isRegister = mode === 'registrar'
  const title = isRegister ? 'Criar conta' : 'Entrar'

  async function submit(event) {
    event.preventDefault()
    if (busy) return

    const values = new FormData(event.currentTarget)
    const email = values.get('email').trim()
    setBusy(true)
    setError('')

    try {
      const session = await request(`/auth/${mode}`, {
        method: 'POST',
        body: { email, senha: values.get('senha') },
      })
      onAuthenticated({ ...session, email })
    } catch (error) {
      setError(error.message)
    } finally {
      setBusy(false)
    }
  }

  function switchMode() {
    setMode(isRegister ? 'login' : 'registrar')
    setError('')
  }

  return (
    <Modal title={title} onClose={onClose} busy={busy}>
      <p className="modal-intro">Acesse sua conta para gerenciar suas rifas.</p>
      {message && <p className="feedback" role="status">{message}</p>}
      <form key={mode} className="stacked-form" onSubmit={submit}>
        <label>
          E-mail
          <input name="email" type="email" autoComplete="email" required />
        </label>
        <label>
          Senha
          <input
            name="senha"
            type="password"
            autoComplete={isRegister ? 'new-password' : 'current-password'}
            minLength={isRegister ? 6 : undefined}
            required
          />
        </label>
        {error && <p className="feedback feedback--error" role="alert">{error}</p>}
        <button type="submit" className="button button--primary" disabled={busy}>
          {busy ? 'Aguarde…' : title}
        </button>
        <button type="button" className="button button--ghost" onClick={switchMode} disabled={busy}>
          {isRegister ? 'Já tenho conta' : 'Criar conta'}
        </button>
      </form>
    </Modal>
  )
}
