import { useEffect, useState } from 'react'
import { request } from '../api.js'
import Modal from '../components/Modal.jsx'
import RaffleEditor from '../components/RaffleEditor.jsx'
import ResourceNotice from '../components/ResourceNotice.jsx'
import StatusBadge from '../components/StatusBadge.jsx'
import useApiResource from '../hooks/useApiResource.js'
import useRaffleEvents from '../hooks/useRaffleEvents.js'
import { formatCurrency, formatNumber } from '../raffle.js'

const actions = {
  draft: [{ route: 'abrir', label: 'Abrir' }],
  open: [{ route: 'fechar', label: 'Fechar' }],
  closed: [{ route: 'abrir', label: 'Reabrir' }, { route: 'sortear', label: 'Sortear' }],
  drawn: [],
}

function BuyersModal({ raffle, token, onClose, onUnauthorized }) {
  const { data, error, status, reload } = useApiResource(`/rifas/${raffle.id}/compradores`, token)

  useEffect(() => {
    if (status === 401) onUnauthorized()
  }, [status, onUnauthorized])

  return (
    <Modal title="Compradores" onClose={onClose}>
      <p className="modal-intro">{raffle.titulo}</p>
      {(!data || error) && <ResourceNotice error={error} reload={reload} />}
      {data?.length === 0 && <p className="feedback">Ainda não há números vendidos.</p>}
      {data?.length > 0 && (
        <ul className="buyer-list">
          {data.map((buyer) => (
            <li key={buyer.numero}>
              <strong>{formatNumber(buyer.numero)} · {buyer.nome}</strong>
              <span>{buyer.contato}</span>
            </li>
          ))}
        </ul>
      )}
    </Modal>
  )
}

export default function MyRafflesPage({ token, onUnauthorized }) {
  const { data, error, status, reload, setState } = useApiResource('/minhas-rifas', token)
  const [editor, setEditor] = useState(null)
  const [buyers, setBuyers] = useState(null)
  const [action, setAction] = useState(null)
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState('')
  const [winner, setWinner] = useState(null)

  useEffect(() => {
    if (status === 401) onUnauthorized()
  }, [status, onUnauthorized])

  useRaffleEvents(() => {
    void reload()
  })

  async function ownerRequest(path, method, body) {
    try {
      return await request(path, { method, body, token })
    } catch (error) {
      if (error.status === 401) onUnauthorized()
      throw error
    }
  }

  function updateList(raffle) {
    setState((current) => {
      const items = current.data || []
      const exists = items.some((item) => item.id === raffle.id)
      const next = exists
        ? items.map((item) => item.id === raffle.id ? { ...item, ...raffle } : item)
        : [raffle, ...items]
      return { data: next, error: '' }
    })
    void reload()
  }

  async function save(body) {
    const raffle = editor.raffle
    const saved = await ownerRequest(raffle ? `/rifas/${raffle.id}` : '/rifas', raffle ? 'PUT' : 'POST', body)
    updateList(saved)
    setEditor(null)
  }

  async function confirmAction() {
    if (working) return
    setWorking(true)
    setActionError('')

    try {
      const result = await ownerRequest(`/rifas/${action.raffle.id}/${action.route}`, 'POST')
      updateList(result.rifa || result)
      if (result.vencedor) setWinner(result.vencedor)
      setAction(null)
    } catch (error) {
      setActionError(error.message)
      if (error.status === 409) void reload()
    } finally {
      setWorking(false)
    }
  }

  return (
    <>
      <section className="page-banner">
        <div className="container">
          <a className="back-link" href="#/">Voltar às rifas</a>
          <h1>Minhas rifas</h1>
        </div>
      </section>
      <section className="container page-content" aria-label="Gerenciar rifas">
        <div className="owner-toolbar">
          <button type="button" className="button button--primary" onClick={() => setEditor({})}>
            Criar rifa
          </button>
        </div>
        {(!data || error) && <ResourceNotice error={error} reload={reload} />}
        {data?.length === 0 && <p className="feedback">Você ainda não criou nenhuma rifa.</p>}
        {data?.length > 0 && (
          <div className="raffle-cards">
            {data.map((raffle) => (
              <article className="raffle-card" key={raffle.id}>
                <StatusBadge status={raffle.status} />
                <h2>{raffle.titulo}</h2>
                <p>{raffle.premio}</p>
                <p>{formatCurrency(raffle.preco_centavos)} por número</p>
                {raffle.numero_sorteado != null && (
                  <p>Número sorteado <strong>{formatNumber(raffle.numero_sorteado)}</strong></p>
                )}
                <div className="owner-actions">
                  {raffle.status !== 'drawn' && (
                    <button type="button" className="button button--ghost" onClick={() => setEditor({ raffle })}>
                      Editar
                    </button>
                  )}
                  {(actions[raffle.status] || []).map((item) => (
                    <button
                      type="button"
                      key={item.route}
                      className="button button--primary"
                      onClick={() => {
                        setActionError('')
                        setAction({ ...item, raffle })
                      }}
                    >
                      {item.label}
                    </button>
                  ))}
                  <button type="button" className="button button--ghost" onClick={() => setBuyers(raffle)}>
                    Compradores
                  </button>
                  {raffle.status !== 'draft' && (
                    <a className="button button--ghost" href={`#/rifas/${raffle.id}`}>Ver rifa</a>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      {editor && <RaffleEditor raffle={editor.raffle} onSave={save} onClose={() => setEditor(null)} />}
      {buyers && (
        <BuyersModal raffle={buyers} token={token} onClose={() => setBuyers(null)} onUnauthorized={onUnauthorized} />
      )}
      {action && (
        <Modal title={`${action.label} rifa`} busy={working} onClose={() => setAction(null)}>
          <div className="stacked-form">
            <p>{action.raffle.titulo}</p>
            {actionError && <p className="feedback feedback--error" role="alert">{actionError}</p>}
            <button type="button" className="button button--primary" onClick={confirmAction} disabled={working}>
              {working ? 'Aguarde…' : 'Confirmar'}
            </button>
          </div>
        </Modal>
      )}
      {winner && (
        <Modal title="Resultado do sorteio" onClose={() => setWinner(null)}>
          <div className="stacked-form">
            <p>Número sorteado <strong>{formatNumber(winner.numero)}</strong></p>
            <p>{winner.nome}</p>
            <p>{winner.contato}</p>
          </div>
        </Modal>
      )}
    </>
  )
}
