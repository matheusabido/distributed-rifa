import Icon from './Icon.jsx'
import './Header.css'

export default function Header({ selectedCount, onOpenCart }) {
  return (
    <header className="header">
      <div className="container header__content">
        <a className="brand" href="#top" aria-label="Rifa do Chico, início">
          <span className="brand__mark">R</span>
          <span>
            Rifa do Chico
            <small>UFMT · Sistemas Distribuídos</small>
          </span>
        </a>

        <nav aria-label="Navegação principal">
          <a href="#numeros">Escolher números</a>
          <a href="#historia">A história</a>
        </nav>

        <button
          type="button"
          className="header__cart"
          onClick={onOpenCart}
          disabled={!selectedCount}
          aria-label={`${selectedCount} números selecionados`}
        >
          <Icon name="ticket" size={19} />
          <span>Minha seleção</span>
          <b>{selectedCount}</b>
        </button>
      </div>
    </header>
  )
}
