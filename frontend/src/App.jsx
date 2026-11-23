import { useState } from 'react'
import CheckoutModal from './components/CheckoutModal.jsx'
import Footer from './components/Footer.jsx'
import Header from './components/Header.jsx'
import Hero from './components/Hero.jsx'
import MobileCheckoutBar from './components/MobileCheckoutBar.jsx'
import RaffleSection from './components/RaffleSection.jsx'
import Story from './components/Story.jsx'
import { reservedNumbers } from './raffle.js'
import './App.css'

export default function App() {
  const [selected, setSelected] = useState([])
  const [isCheckoutOpen, setIsCheckoutOpen] = useState(false)

  function toggleNumber(number) {
    if (reservedNumbers.has(number)) {
      return
    }

    setSelected((current) => {
      if (current.includes(number)) {
        return current.filter((item) => item !== number)
      }

      return [...current, number].sort((a, b) => a - b)
    })
  }

  function openCheckout() {
    setIsCheckoutOpen(true)
  }

  function closeCheckout() {
    setIsCheckoutOpen(false)
  }

  return (
    <div className="app-shell" id="top">
      <Header selectedCount={selected.length} onOpenCart={openCheckout} />

      <main>
        <Hero />
        <RaffleSection
          selected={selected}
          onToggle={toggleNumber}
          onContinue={openCheckout}
        />
        <Story />
      </main>

      <Footer />
      <MobileCheckoutBar count={selected.length} onContinue={openCheckout} />

      {isCheckoutOpen && (
        <CheckoutModal selected={selected} onClose={closeCheckout} />
      )}
    </div>
  )
}
