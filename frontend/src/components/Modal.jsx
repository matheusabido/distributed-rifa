import { useEffect, useRef } from 'react'
import Icon from './Icon.jsx'

export default function Modal({ title, onClose, busy = false, children }) {
  const dialog = useRef(null)

  useEffect(() => {
    const element = dialog.current
    element.showModal()
    return () => element.close()
  }, [])

  function cancel(event) {
    event.preventDefault()
    if (!busy) onClose()
  }

  return (
    <dialog
      ref={dialog}
      className="checkout-modal"
      aria-labelledby="modal-title"
      onCancel={cancel}
    >
      <button
        type="button"
        className="modal-close"
        aria-label="Fechar"
        disabled={busy}
        onClick={onClose}
      >
        <Icon name="close" />
      </button>
      <h2 id="modal-title">{title}</h2>
      {children}
    </dialog>
  )
}
