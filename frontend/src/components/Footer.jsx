import './Footer.css'

export default function Footer() {
  return (
    <footer>
      <div className="container footer-content">
        <div className="brand brand--footer">
          <span className="brand__mark" aria-hidden="true">R</span>
          <span>
            Rifas UFMT
            <small>Universidade Federal de Mato Grosso</small>
          </span>
        </div>
        <p>Projeto da disciplina de Sistemas Distribuídos.</p>
      </div>
    </footer>
  )
}
