import './Header.css'

export default function Header({ session, onOpenAuth, onSignOut }) {
  return (
    <header className="header">
      <div className="container header__content">
        <a className="brand" href="#/" aria-label="Rifas UFMT, início">
          <span className="brand__mark" aria-hidden="true">R</span>
          <span>
            Rifas UFMT
            <small>Sistemas Distribuídos</small>
          </span>
        </a>

        <nav className="account-nav" aria-label="Conta">
          {session ? (
            <>
              <a className="button button--primary" href="#/minhas-rifas">Minhas rifas</a>
              <button type="button" className="button button--ghost" onClick={onSignOut}>Sair</button>
            </>
          ) : (
            <>
              <button type="button" className="button button--ghost" onClick={() => onOpenAuth('login')}>
                Entrar
              </button>
              <button type="button" className="button button--primary" onClick={() => onOpenAuth('registrar')}>
                Criar conta
              </button>
            </>
          )}
        </nav>
      </div>
    </header>
  )
}
