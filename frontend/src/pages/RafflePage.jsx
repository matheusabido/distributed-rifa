import { useState } from 'react'
import CheckoutModal from '../components/CheckoutModal.jsx'
import RaffleSection from '../components/RaffleSection.jsx'
import ResourceNotice from '../components/ResourceNotice.jsx'
import StatusBadge from '../components/StatusBadge.jsx'
import useApiResource from '../hooks/useApiResource.js'
import useRaffleEvents from '../hooks/useRaffleEvents.js'
import { formatCurrency, formatNumber } from '../raffle.js'

export default function RafflePage({ raffleId }) {
  const { data, error, reload } = useApiResource(`/rifas/${raffleId}`)
  const [selected, setSelected] = useState(null)
  const [liveSold, setLiveSold] = useState([])
  const [winner, setWinner] = useState(null)

  function markSold(number) {
    if (number != null) setLiveSold((current) => [...new Set([...current, Number(number)])])
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
  const unavailable = sold.has(selected)
    ? 'Este número já foi vendido.'
    : !isOpen ? 'Esta rifa não está aberta para compras.' : ''

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
        <RaffleSection total={raffle.total_numeros} sold={sold} isOpen={isOpen} onSelect={setSelected} />
      </div>
      {selected != null && (
        <CheckoutModal
          raffle={raffle}
          number={selected}
          unavailable={unavailable}
          onClose={() => setSelected(null)}
          onResult={(number) => {
            markSold(number)
            void reload()
          }}
        />
      )}
    </>
  )
}
