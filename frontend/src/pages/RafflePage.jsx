import { useState } from 'react'
import CheckoutModal from '../components/CheckoutModal.jsx'
import RaffleSection from '../components/RaffleSection.jsx'
import ResourceNotice from '../components/ResourceNotice.jsx'
import StatusBadge from '../components/StatusBadge.jsx'
import useApiResource from '../hooks/useApiResource.js'
import useRaffleEvents from '../hooks/useRaffleEvents.js'
import { formatCurrency, formatNumber } from '../raffle.js'

export default function RafflePage({ raffleId, accountEmail }) {
  const { data, error, reload } = useApiResource(`/rifas/${raffleId}`)
  const [selected, setSelected] = useState([])
  const [checkoutOpen, setCheckoutOpen] = useState(false)
  const [liveSold, setLiveSold] = useState([])
  const [winner, setWinner] = useState(null)

  function markSold(numbers) {
    const list = [numbers].flat().filter((number) => number != null).map(Number)
    if (list.length === 0) return
    setLiveSold((current) => [...new Set([...current, ...list])])
    setSelected((current) => current.filter((number) => !list.includes(number)))
  }

  function toggleSelected(number) {
    setSelected((current) =>
      current.includes(number) ? current.filter((item) => item !== number) : [...current, number],
    )
  }

  useRaffleEvents((event) => {
    if (!event) {
      void reload()
      return
    }

    if (String(event.rifa_id) !== raffleId) return

    if (event.tipo === 'ticket_bought') {
      markSold(event.dados?.numero)
    } else if (event.tipo === 'raffle_drawn') {
      setWinner(event.dados)
    }

    void reload()
  })

  const back = <a className="back-link" href="#/">Voltar às rifas</a>

  if (!data) {
    return (
      <div className="container page-content">
        {back}
        <ResourceNotice error={error} reload={reload} />
      </div>
    )
  }

  const raffle = data.rifa
  const sold = new Set([...(data.vendidos || []).map((ticket) => Number(ticket.numero)), ...liveSold])
  const winnerNumber = winner?.numero ?? raffle.numero_sorteado
  const isOpen = raffle.status === 'open' && winnerNumber == null
  const selection = isOpen ? selected.filter((number) => !sold.has(number)) : []
  const unavailable = !isOpen ? 'Esta rifa não está aberta para compras.' : ''

  return (
    <>
      <section className="page-banner">
        <div className="container">
          {back}
          <StatusBadge status={winnerNumber == null ? raffle.status : 'drawn'} />
          <h1>{raffle.titulo}</h1>
          {raffle.descricao && <p className="raffle-description">{raffle.descricao}</p>}
          <dl className="detail-facts">
            <div>
              <dt>Prêmio</dt>
              <dd>{raffle.premio}</dd>
            </div>
            <div>
              <dt>Valor por número</dt>
              <dd>{formatCurrency(raffle.preco_centavos)}</dd>
            </div>
            <div>
              <dt>Números vendidos</dt>
              <dd>{sold.size} de {raffle.total_numeros}</dd>
            </div>
          </dl>
        </div>
      </section>
      <div className="container page-content">
        {error && <ResourceNotice error={error} reload={reload} />}
        {winnerNumber != null && (
          <p className="feedback" role="status">
            Número sorteado <strong>{formatNumber(winnerNumber)}</strong>
            {winner?.nome && <> · {winner.nome}</>}
          </p>
        )}
        <RaffleSection
          total={raffle.total_numeros}
          sold={sold}
          selected={new Set(selection)}
          isOpen={isOpen}
          onToggle={toggleSelected}
        />
      </div>
      {selection.length > 0 && !checkoutOpen && (
        <div className="selection-bar" role="region" aria-label="Números selecionados">
          <div className="container selection-bar__content">
            <span>
              <strong>{selection.length}</strong> {selection.length > 1 ? 'números selecionados' : 'número selecionado'}
              {' · '}{formatCurrency(raffle.preco_centavos * selection.length)}
            </span>
            <div className="selection-bar__actions">
              <button type="button" className="button button--ghost" onClick={() => setSelected([])}>
                Limpar
              </button>
              <button type="button" className="button button--primary" onClick={() => setCheckoutOpen(true)}>
                Comprar
              </button>
            </div>
          </div>
        </div>
      )}
      {checkoutOpen && (
        <CheckoutModal
          raffle={raffle}
          numbers={selection}
          accountEmail={accountEmail}
          unavailable={unavailable}
          onClose={() => setCheckoutOpen(false)}
          onResult={({ bought, sold: takenNumbers }) => {
            markSold([...bought, ...takenNumbers])
            void reload()
          }}
        />
      )}
    </>
  )
}
