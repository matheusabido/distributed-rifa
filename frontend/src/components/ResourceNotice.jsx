export default function ResourceNotice({ error, reload }) {
  if (!error) {
    return <p className="feedback" role="status">Carregando…</p>
  }

  return (
    <div className="feedback feedback--error">
      <p role="alert">{error}</p>
      <button type="button" className="button button--ghost" onClick={reload}>
        Tentar novamente
      </button>
    </div>
  )
}
