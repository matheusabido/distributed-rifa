import { NUMBER_COUNT, PRICE, formatCurrency } from '../raffle.js'
import CatIllustration from './CatIllustration.jsx'
import Icon from './Icon.jsx'

export default function Hero() {
  return (
    <section className="hero" aria-labelledby="hero-title">
      <div className="hero__glow hero__glow--one" />
      <div className="hero__glow hero__glow--two" />

      <div className="container hero__content">
        <div className="hero__copy">
          <div className="eyebrow">
            <Icon name="heart" size={16} /> Uma rifa que faz a diferença
          </div>
          <h1 id="hero-title">
            Ajude o <span>Chico</span> a voltar a ronronar.
          </h1>
          <p>
            Participe da nossa rifa solidária e contribua com o tratamento desse
            pequeno guerreiro. Cada número é um gesto de carinho.
          </p>

          <div className="hero__actions">
            <a className="button button--white" href="#numeros">
              Escolher meu número
            </a>
            <a className="text-link" href="#historia">
              Conhecer a história <Icon name="arrowDown" size={17} />
            </a>
          </div>

          <div className="hero__meta">
            <div>
              <strong>{formatCurrency(PRICE)}</strong>
              <span>por número</span>
            </div>
            <div>
              <strong>R$ 500</strong>
              <span>prêmio via Pix</span>
            </div>
            <div>
              <strong>{NUMBER_COUNT}</strong>
              <span>números no total</span>
            </div>
          </div>
        </div>

        <div className="hero__art" aria-label="Ilustração de um gato chamado Chico">
          <div className="art-card">
            <span className="art-card__badge">
              <Icon name="spark" size={16} /> Nosso paciente
            </span>
            <CatIllustration />
            <div className="art-card__caption">
              <span>Chico</span>
              <small>Em tratamento veterinário</small>
            </div>
          </div>
          <div className="floating-note">
            <span className="floating-note__icon">
              <Icon name="heart" size={18} />
            </span>
            <div>
              <strong>Você pode ajudar!</strong>
              <small>Escolha um número abaixo</small>
            </div>
          </div>
        </div>
      </div>

      <div className="hero__wave" />
    </section>
  )
}
