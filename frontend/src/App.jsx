import { useEffect, useState } from 'react'
import AuthModal from './components/AuthModal.jsx'
import Footer from './components/Footer.jsx'
import Header from './components/Header.jsx'
import CatalogPage from './pages/CatalogPage.jsx'
import MyRafflesPage from './pages/MyRafflesPage.jsx'
import RafflePage from './pages/RafflePage.jsx'
import './App.css'

const sessionKey = 'rifa.session'

function readSession() {
  try {
    const data = JSON.parse(sessionStorage.getItem(sessionKey))
    return typeof data?.token === 'string' ? data : null
  } catch {
    return null
  }
}

function getRoute() {
  return window.location.hash.slice(1) || '/'
}

export default function App() {
  const [route, setRoute] = useState(getRoute)
  const [session, setSession] = useState(readSession)
  const [authMode, setAuthMode] = useState(null)
  const [authMessage, setAuthMessage] = useState('')
  const raffleId = /^\/rifas\/(\d+)\/?$/.exec(route)?.[1]

  useEffect(() => {
    function navigate() {
      setRoute(getRoute())
      window.scrollTo(0, 0)
    }

    window.addEventListener('hashchange', navigate)
    return () => window.removeEventListener('hashchange', navigate)
  }, [])

  function signIn(nextSession) {
    sessionStorage.setItem(sessionKey, JSON.stringify(nextSession))
    setSession(nextSession)
    setAuthMode(null)
    setAuthMessage('')
    window.location.hash = '/minhas-rifas'
  }

  function signOut() {
    sessionStorage.removeItem(sessionKey)
    setSession(null)
  }

  function expireSession() {
    signOut()
    setAuthMessage('Sua sessão expirou. Entre novamente para continuar.')
    setAuthMode('login')
  }

  let page = <CatalogPage />

  if (raffleId) {
    page = <RafflePage key={raffleId} raffleId={raffleId} accountEmail={session?.email} />
  } else if (route === '/minhas-rifas') {
    page = session ? (
      <MyRafflesPage key={session.usuario_id} token={session.token} onUnauthorized={expireSession} />
    ) : (
      <section className="container page-content">
        <h1>Minhas rifas</h1>
        <p className="feedback">Entre para acessar suas rifas.</p>
        <button type="button" className="button button--primary" onClick={() => setAuthMode('login')}>
          Entrar
        </button>
      </section>
    )
  }

  return (
    <div className="app-shell" id="top">
      <Header session={session} onOpenAuth={setAuthMode} onSignOut={signOut} />
      <main>{page}</main>
      <Footer />

      {authMode && (
        <AuthModal
          initialMode={authMode}
          message={authMessage}
          onClose={() => {
            setAuthMode(null)
            setAuthMessage('')
          }}
          onAuthenticated={signIn}
        />
      )}
    </div>
  )
}
