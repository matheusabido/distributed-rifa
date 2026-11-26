import { useState } from 'react'
import ResourceNotice from '../components/ResourceNotice.jsx'
import StatusBadge from '../components/StatusBadge.jsx'
import useApiResource from '../hooks/useApiResource.js'
import useRaffleEvents from '../hooks/useRaffleEvents.js'
import { formatCurrency } from '../raffle.js'
import './CatalogPage.css'

const filters = [
  { status: 'open', label: 'Abertas' },
  { status: 'closed', label: 'Encerradas' },
  { status: 'drawn', label: 'Sorteadas' },
]

function RaffleCard({ raffle }) {
  const sold = Number(raffle.vendidos) || 0

  return (
    <article className="raffle-card">
      <StatusBadge status={raffle.status} />
      <h2>{raffle.titulo}</h2>
      {raffle.descricao && <p className="raffle-card__description">{raffle.descricao}</p>}
      <dl className="raffle-card__facts">
        <div>
          <dt>Prêmio</dt>
          <dd>{raffle.premio}</dd>
        </div>
        <div>
          <dt>Valor por número</dt>
          <dd>{formatCurrency(raffle.preco_centavos)}</dd>
        </div>
      </dl>
      <div className="raffle-card__progress">
        <progress
          max={raffle.total_numeros}
          value={sold}
          aria-label="Progresso de vendas"
        />
        <p>{sold} de {raffle.total_numeros} números vendidos</p>
      </div>
      <a className="button button--primary button--full" href={`#/rifas/${raffle.id}`}>
        Ver detalhes
      </a>
    </article>
  )
}

function RaffleList({ status }) {
  const { data, error, reload } = useApiResource(status === 'open' ? '/rifas' : `/rifas?status=${status}`)

  useRaffleEvents(() => {
    void reload()
  })

  if (!data) return <ResourceNotice error={error} reload={reload} />

  return (
    <>
      {error && <ResourceNotice error={error} reload={reload} />}
      {data.length ? (
        <div className="raffle-cards">
          {data.map((raffle) => <RaffleCard key={raffle.id} raffle={raffle} />)}
        </div>
      ) : (
        <p className="feedback" role="status">Nenhuma rifa nesta categoria.</p>
      )}
    </>
  )
}

export default function CatalogPage() {
  const [status, setStatus] = useState('open')

  return (
    <>
      <section className="page-banner">
        <div className="container">
          <p className="page-banner__context">UFMT · Sistemas Distribuídos</p>
          <h1>Rifas solidárias</h1>
          <p>Escolha uma rifa, conheça a causa e encontre seu número da sorte.</p>
        </div>
      </section>
      <section className="container page-content" aria-label="Catálogo de rifas">
        <div className="catalog-filters" role="group" aria-label="Filtrar por situação">
          {filters.map((filter) => (
            <button
              type="button"
              key={filter.status}
              aria-pressed={status === filter.status}
              onClick={() => setStatus(filter.status)}
            >
              {filter.label}
            </button>
          ))}
        </div>
        <RaffleList key={status} status={status} />
      </section>
    </>
  )
}
