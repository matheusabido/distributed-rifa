import Icon from './Icon.jsx'

const storyPoints = [
  'Ajuda direta no tratamento',
  'Sorteio após todos os números',
  'Resultado divulgado aos participantes',
]

export default function Story() {
  return (
    <section className="story-section" id="historia" aria-labelledby="story-title">
      <div className="container story-grid">
        <div className="story-mark" aria-hidden="true">
          <span>🐾</span>
          <div>
            <strong>100%</strong>
            <small>do valor arrecadado ajuda no tratamento</small>
          </div>
        </div>
        <div className="story-copy">
          <span className="section-kicker">Por que essa rifa existe?</span>
          <h2 id="story-title">Uma corrente de carinho pelo Chico</h2>
          <p>
            O Chico precisa continuar seu tratamento veterinário e cada contribuição
            nos aproxima desse objetivo. Esta rifa nasceu como um projeto da
            disciplina de Sistemas Distribuídos da UFMT e, acima de tudo, como uma
            forma de unir tecnologia e solidariedade.
          </p>
          <div className="story-points">
            {storyPoints.map((point) => (
              <span key={point}>
                <Icon name="check" size={16} /> {point}
              </span>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
