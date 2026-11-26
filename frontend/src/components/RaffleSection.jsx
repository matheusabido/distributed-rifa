import { useState } from 'react'
import { formatNumber } from '../raffle.js'

export default function RaffleSection({ total, sold, selected, isOpen, onToggle }) {
  const [onlyAvailable, setOnlyAvailable] = useState(false)
  const [page, setPage] = useState(1)
  const pageCount = Math.ceil(total / 100)
  const currentPage = Math.min(page, pageCount)
  const firstNumber = (currentPage - 1) * 100 + 1
  const lastNumber = Math.min(firstNumber + 99, total)
  const numbers = Array.from({ length: lastNumber - firstNumber + 1 }, (_, index) => firstNumber + index)
  const visible = onlyAvailable && isOpen
    ? numbers.filter((number) => !sold.has(number))
    : numbers

  return (
    <section aria-labelledby="raffle-title">
      <div className="section-heading">
        <div>
          <h2 id="raffle-title">Números da rifa</h2>
          <p>{isOpen ? 'Toque nos números disponíveis para selecionar e depois em Comprar.' : 'Esta rifa está fora do período de compras.'}</p>
        </div>
        {isOpen && (
          <div className="filter-control" role="group" aria-label="Filtrar números">
            <button
              type="button"
              className={!onlyAvailable ? 'active' : ''}
              aria-pressed={!onlyAvailable}
              onClick={() => setOnlyAvailable(false)}
            >
              Todos
            </button>
            <button
              type="button"
              className={onlyAvailable ? 'active' : ''}
              aria-pressed={onlyAvailable}
              onClick={() => setOnlyAvailable(true)}
            >
              Disponíveis
            </button>
          </div>
        )}
      </div>

      <div className="number-legend">
        <span><i className="legend-dot legend-dot--available" />{isOpen ? 'Disponível' : 'Não vendido'}</span>
        {isOpen && <span><i className="legend-dot legend-dot--selected" />Selecionado</span>}
        <span><i className="legend-dot legend-dot--sold" />Vendido</span>
      </div>

      {pageCount > 1 && (
        <nav className="pagination" aria-label="Páginas de números">
          <button
            type="button"
            className="button button--ghost"
            disabled={currentPage === 1}
            onClick={() => setPage(currentPage - 1)}
          >
            Anterior
          </button>
          <span aria-live="polite">{firstNumber}–{lastNumber} de {total}</span>
          <button
            type="button"
            className="button button--ghost"
            disabled={currentPage === pageCount}
            onClick={() => setPage(currentPage + 1)}
          >
            Próxima
          </button>
        </nav>
      )}

      {visible.length === 0 ? (
        <p className="feedback" role="status">Nenhum número disponível nesta página.</p>
      ) : (
        <div className="number-grid" role="group" aria-label="Números da rifa">
          {visible.map((number) => {
            const isSold = sold.has(number)
            const isSelected = !isSold && selected.has(number)
            const status = isSold ? 'vendido' : isSelected ? 'selecionado' : isOpen ? 'disponível' : 'não vendido'
            const className = ['number-button', isSold && 'sold', isSelected && 'selected'].filter(Boolean).join(' ')

            return (
              <button
                key={number}
                type="button"
                className={className}
                disabled={!isOpen || isSold}
                aria-label={`Número ${number}, ${status}`}
                aria-pressed={isOpen && !isSold ? isSelected : undefined}
                onClick={() => onToggle(number)}
              >
                {formatNumber(number)}
              </button>
            )
          })}
        </div>
      )}
    </section>
  )
}
