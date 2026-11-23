import { useState } from 'react'
import { formatNumber, NUMBER_COUNT, reservedNumbers } from '../raffle.js'
import Icon from './Icon.jsx'
import SelectionSummary from './SelectionSummary.jsx'

const numbers = Array.from({ length: NUMBER_COUNT }, (_, index) => index + 1)

const filters = [
  { value: 'all', label: 'Todos' },
  { value: 'available', label: 'Disponíveis' },
]

const legend = [
  { state: 'available', label: 'Disponível' },
  { state: 'selected', label: 'Selecionado' },
  { state: 'reserved', label: 'Reservado' },
]

function NumberButton({ number, isSelected, onToggle }) {
  const isReserved = reservedNumbers.has(number)
  const className = [
    'number-button',
    isReserved && 'reserved',
    isSelected && 'selected',
  ].filter(Boolean).join(' ')

  let status = 'disponível'
  if (isReserved) {
    status = 'reservado'
  } else if (isSelected) {
    status = 'selecionado'
  }

  return (
    <button
      type="button"
      className={className}
      onClick={() => onToggle(number)}
      disabled={isReserved}
      aria-pressed={isSelected}
      aria-label={`Número ${number}, ${status}`}
    >
      {formatNumber(number)}
      {isSelected && (
        <span className="number-check">
          <Icon name="check" size={10} />
        </span>
      )}
    </button>
  )
}

export default function RaffleSection({ selected, onToggle, onContinue }) {
  const [filter, setFilter] = useState('all')
  const visibleNumbers = filter === 'available'
    ? numbers.filter((number) => !reservedNumbers.has(number))
    : numbers

  return (
    <section className="raffle-section" id="numeros" aria-labelledby="raffle-title">
      <div className="container raffle-layout">
        <div className="raffle-main">
          <div className="section-heading">
            <div>
              <span className="section-kicker">Participe agora</span>
              <h2 id="raffle-title">Escolha seus números</h2>
              <p>
                Selecione quantos quiser. Quanto mais números, mais chances de
                ganhar e mais você ajuda o Chico.
              </p>
            </div>
            <div className="filter-control" role="group" aria-label="Filtrar números">
              {filters.map(({ value, label }) => (
                <button
                  key={value}
                  type="button"
                  className={filter === value ? 'active' : ''}
                  aria-pressed={filter === value}
                  onClick={() => setFilter(value)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div className="number-legend" aria-label="Legenda">
            {legend.map(({ state, label }) => (
              <span key={state}>
                <i className={`legend-dot legend-dot--${state}`} /> {label}
              </span>
            ))}
          </div>

          <div className="number-grid" role="group" aria-label="Números da rifa">
            {visibleNumbers.map((number) => (
              <NumberButton
                key={number}
                number={number}
                isSelected={selected.includes(number)}
                onToggle={onToggle}
              />
            ))}
          </div>
        </div>

        <SelectionSummary selected={selected} onContinue={onContinue} />
      </div>
    </section>
  )
}
