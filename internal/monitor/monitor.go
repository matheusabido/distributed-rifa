// Package monitor cuida da saúde do sistema: quem está vivo, quem é a API
// mestre e qual banco é o primário.
//
// Resumo das regras:
//
//   - Papéis: existe UMA API mestre; as outras são seguidoras.
//   - MESTRE, a cada 2s: faz Ping em todas as APIs e testa os bancos. Se o
//     banco primário caiu e há réplica viva, pede votos às outras APIs; com a
//     maioria das APIs vivas a favor (sozinha, decide sozinha), promove a
//     réplica (pg_promote(true, 10) + CHECKPOINT).
//   - SEGUIDORA, no seu horário (ver schedule.go): só faz Ping na mestre.
//     NÃO testa os bancos: se uma query dela falhar, a requisição dá erro e
//     resolver é papel da mestre. Se a mestre não responde, começa uma
//     ELEIÇÃO (bully): vence a API viva de MENOR ID.
//   - Ao subir, toda API faz uma eleição. Por isso a API 1, quando volta,
//     retoma o posto de mestre.
//   - Quem descobre uma mudança avisa todas as outras (ReportStatus).
package monitor

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"
	"rifa/gen/rifapb"
	"rifa/internal/config"
	"rifa/internal/peerconn"
)

// Estados possíveis.
const (
	Up      = "up"      // API respondendo
	Down    = "down"    // API ou banco fora do ar
	Unknown = "unknown" // ainda não checamos
	Primary = "primary" // banco que aceita escrita
	Replica = "replica" // banco que só copia o primário
)

const (
	masterInterval = 2 * time.Second // de quanto em quanto tempo a mestre checa tudo
	checkTimeout   = 1 * time.Second
	voteTimeout    = 3 * time.Second  // quem vota testa os bancos antes de responder
	promoteWait    = 10               // segundos que o pg_promote espera a promoção
	promoteTimeout = 20 * time.Second // promoção + checkpoint
)

type peerAPI struct {
	id     int
	name   string // "api2"
	addr   string
	client rifapb.MonitorClient
}

type database struct {
	name string // "db1"
	url  string // conexão direta com ESTE banco
}

type Monitor struct {
	rifapb.UnimplementedMonitorServer

	selfID    int
	totalAPIs int
	peers     []peerAPI
	dbs       []database

	// OnPrimaryChange é chamado quando o banco primário muda (a API usa para
	// fechar as conexões antigas do pool).
	OnPrimaryChange func()

	mu        sync.Mutex
	masterID  int                  // 0 = ainda não sei quem é a mestre
	state     map[string]string    // "api2" -> "down", "db1" -> "primary"...
	changedAt map[string]time.Time // quando cada estado mudou pela última vez
	lastCheck time.Time

	electionMu sync.Mutex // uma eleição por vez
	failoverMu sync.Mutex // uma promoção por vez
}

func New(cfg config.API) (*Monitor, error) {
	m := &Monitor{
		selfID:    cfg.NodeID,
		totalAPIs: cfg.TotalAPIs(),
		state:     map[string]string{},
		changedAt: map[string]time.Time{},
	}
	m.state[apiName(cfg.NodeID)] = Up // eu mesma, claro

	for _, p := range cfg.Peers {
		conn, err := peerconn.Dial(p.Addr)
		if err != nil {
			return nil, err
		}
		name := apiName(p.ID)
		m.peers = append(m.peers, peerAPI{id: p.ID, name: name, addr: p.Addr, client: rifapb.NewMonitorClient(conn)})
		m.state[name] = Unknown
	}
	sort.Slice(m.peers, func(i, j int) bool { return m.peers[i].id < m.peers[j].id })
	for i, host := range cfg.DBHosts {
		name := fmt.Sprintf("db%d", i+1)
		m.dbs = append(m.dbs, database{name: name, url: cfg.DBHostURL(host)})
		m.state[name] = Unknown
	}
	return m, nil
}

func apiName(id int) string { return fmt.Sprintf("api%d", id) }

// ---------------------------------------------------------------------------
// Laço principal
// ---------------------------------------------------------------------------

// Run: faz uma eleição ao subir e depois age conforme o papel:
//
//	mestre    -> checa tudo a cada 2s
//	seguidora -> no seu horário, só confere se a mestre responde
func (m *Monitor) Run(ctx context.Context) {
	log.Printf("[monitor] como mestre, checo a cada %v; como seguidora, nos segundos %v",
		masterInterval, SlotSeconds(m.selfID, m.totalAPIs))

	m.elect(ctx)

	for {
		wait := masterInterval
		if !m.IsMaster() {
			wait = time.Until(NextSlot(time.Now(), m.selfID, m.totalAPIs))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if m.IsMaster() {
			m.masterCheck(ctx)
		} else {
			m.followerCheck(ctx)
		}
		m.mu.Lock()
		m.lastCheck = time.Now().UTC()
		m.mu.Unlock()
	}
}

// followerCheck: a seguidora só confere se a mestre está respondendo.
func (m *Monitor) followerCheck(ctx context.Context) {
	master := m.MasterID()
	p, ok := m.peer(master)
	if !ok {
		log.Printf("[monitor] não sei quem é a mestre: começando eleição")
		m.elect(ctx)
		return
	}
	if st, _ := m.pingAPI(ctx, p); st == Up {
		m.apply(p.name, Up, m.selfID)
		return
	}
	log.Printf("[monitor] a mestre %s não respondeu: começando eleição", p.name)
	m.mu.Lock()
	if m.masterID == master {
		m.masterID = 0
	}
	m.mu.Unlock()
	m.apply(p.name, Down, m.selfID)
	m.elect(ctx)
}

// masterCheck: a mestre testa todas as APIs e todos os bancos AO MESMO TEMPO
// (em paralelo, ~1s no máximo) e, se preciso, conduz o failover do banco.
func (m *Monitor) masterCheck(ctx context.Context) {
	var wg sync.WaitGroup
	for _, p := range m.peers {
		wg.Add(1)
		go func(p peerAPI) {
			defer wg.Done()
			old := m.stateOf(p.name)
			st, theirMaster := m.pingAPI(ctx, p)
			m.apply(p.name, st, m.selfID)
			switch {
			case st == Up && old != Up:
				// API que (re)apareceu: conto quem é a mestre e o estado atual.
				go m.sendSnapshot(p)
			case st == Up && theirMaster != m.selfID:
				// Ela acha que a mestre é outra (ex: não ouviu o meu anúncio).
				// Me anuncio de novo; se eu não tiver prioridade, ela puxa
				// uma eleição.
				log.Printf("[eleição] %s acha que a mestre é a api%d: me anuncio de novo", p.name, theirMaster)
				go m.sendSnapshot(p)
			}
		}(p)
	}
	for _, db := range m.dbs {
		wg.Add(1)
		go func(db database) {
			defer wg.Done()
			m.apply(db.name, checkDB(ctx, db.url), m.selfID)
		}(db)
	}
	wg.Wait()

	if m.IsMaster() { // posso ter perdido o posto durante a checagem
		m.maybeFailover(ctx)
	}
}

// pingAPI devolve se a API respondeu e quem ELA acha que é a mestre.
func (m *Monitor) pingAPI(ctx context.Context, p peerAPI) (state string, theirMaster int) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	// WaitForReady: se a conexão está reconectando, espera (até o timeout)
	// em vez de falhar na hora.
	resp, err := p.client.Ping(ctx, &rifapb.PingRequest{FromId: int32(m.selfID)}, grpc.WaitForReady(true))
	if err != nil {
		return Down, 0
	}
	return Up, int(resp.MasterId)
}

// checkDB abre uma conexão direta com UM banco e pergunta se ele está em
// recovery (réplica) ou não (primário).
func checkDB(ctx context.Context, url string) string {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return Down
	}
	defer conn.Close(context.Background())
	var inRecovery bool
	if err := conn.QueryRow(ctx, `SELECT pg_is_in_recovery()`).Scan(&inRecovery); err != nil {
		return Down
	}
	if inRecovery {
		return Replica
	}
	return Primary
}

// checkAllDBs testa todos os bancos em paralelo, sem registrar nada.
func (m *Monitor) checkAllDBs(ctx context.Context) map[string]string {
	out := make(map[string]string, len(m.dbs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, db := range m.dbs {
		wg.Add(1)
		go func(db database) {
			defer wg.Done()
			st := checkDB(ctx, db.url)
			mu.Lock()
			out[db.name] = st
			mu.Unlock()
		}(db)
	}
	wg.Wait()
	return out
}

// ---------------------------------------------------------------------------
// Mudanças de estado
// ---------------------------------------------------------------------------

// apply registra o estado de um alvo. Se MUDOU e fui eu que descobri
// (reporter == eu), aviso todas as outras APIs.
func (m *Monitor) apply(target, newState string, reporter int) {
	m.mu.Lock()
	old := m.state[target]
	if old == newState {
		m.mu.Unlock()
		return
	}
	m.state[target] = newState
	m.changedAt[target] = time.Now().UTC()
	m.mu.Unlock()

	if reporter == m.selfID {
		log.Printf("[monitor] DESCOBRI: %s %s -> %s. Avisando as outras APIs.", target, old, newState)
		go m.broadcast(target, newState)
	} else {
		log.Printf("[monitor] a api%d me avisou: %s %s -> %s", reporter, target, old, newState)
	}

	if isDB(target) && (old == Primary || newState == Primary) && m.OnPrimaryChange != nil {
		m.OnPrimaryChange()
	}
}

// broadcast chama ReportStatus em todas as outras APIs (as que estão
// fora do ar só dão timeout, sem problema).
func (m *Monitor) broadcast(target, state string) {
	report := &rifapb.StatusReport{
		ReporterId: int32(m.selfID),
		Target:     target,
		State:      state,
		AtUnix:     time.Now().UTC().Unix(),
	}
	for _, p := range m.peers {
		if p.name == target || m.stateOf(p.name) == Down {
			continue // não adianta avisar quem caiu; quando voltar, recebe o snapshot
		}
		go func(p peerAPI) {
			ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
			defer cancel()
			if _, err := p.client.ReportStatus(ctx, report); err != nil {
				log.Printf("[monitor] não consegui avisar %s: %v", p.name, err)
			}
		}(p)
	}
}

// sendSnapshot manda para uma API (que acabou de aparecer) quem é a mestre
// e tudo o que eu sei. Só a mestre chama isto.
func (m *Monitor) sendSnapshot(p peerAPI) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*checkTimeout)
	defer cancel()
	if _, err := p.client.Coordinator(ctx, &rifapb.CoordinatorRequest{MasterId: int32(m.selfID)}); err != nil {
		return
	}
	m.mu.Lock()
	snapshot := make(map[string]string, len(m.state))
	for k, v := range m.state {
		snapshot[k] = v
	}
	m.mu.Unlock()
	for target, st := range snapshot {
		if target == p.name || st == Unknown {
			continue
		}
		_, _ = p.client.ReportStatus(ctx, &rifapb.StatusReport{
			ReporterId: int32(m.selfID), Target: target, State: st, AtUnix: time.Now().UTC().Unix(),
		})
	}
}

// ---------------------------------------------------------------------------
// Hierarquia e eleição (algoritmo do valentão / bully, menor ID vence)
// ---------------------------------------------------------------------------

// MasterID é a mestre que foi eleita (0 = ainda não sei).
func (m *Monitor) MasterID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.masterID
}

func (m *Monitor) IsMaster() bool { return m.MasterID() == m.selfID }

func (m *Monitor) setMaster(id int) {
	m.mu.Lock()
	old := m.masterID
	m.masterID = id
	m.mu.Unlock()
	if old != id {
		log.Printf("[eleição] a API mestre agora é a api%d", id)
	}
}

func (m *Monitor) peer(id int) (peerAPI, bool) {
	for _, p := range m.peers {
		if p.id == id {
			return p, true
		}
	}
	return peerAPI{}, false
}

// elect: pergunto às APIs de ID MENOR que o meu se estão vivas. Se alguma
// responder, ela tem prioridade e vai se anunciar (Coordinator); eu espero.
// Se nenhuma responder, eu viro a mestre e aviso todo mundo.
func (m *Monitor) elect(ctx context.Context) {
	if !m.electionMu.TryLock() {
		return // já tem uma eleição minha em andamento
	}
	defer m.electionMu.Unlock()

	var lower []peerAPI
	for _, p := range m.peers {
		if p.id < m.selfID {
			lower = append(lower, p)
		}
	}

	alive := make([]bool, len(lower))
	var wg sync.WaitGroup
	for i, p := range lower {
		wg.Add(1)
		go func(i int, p peerAPI) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, checkTimeout)
			defer cancel()
			_, err := p.client.Election(cctx, &rifapb.ElectionRequest{FromId: int32(m.selfID)}, grpc.WaitForReady(true))
			alive[i] = err == nil
		}(i, p)
	}
	wg.Wait()

	for i, p := range lower {
		if alive[i] {
			m.apply(p.name, Up, m.selfID)
			log.Printf("[eleição] %s (ID menor) está viva: ela tem prioridade, aguardo o anúncio", p.name)
			return
		}
		m.apply(p.name, Down, m.selfID)
	}

	m.becomeMaster(ctx)
}

// becomeMaster: assumo o posto, aviso todas as APIs e já faço uma checagem
// completa (sem esperar o meu horário).
func (m *Monitor) becomeMaster(ctx context.Context) {
	wasMaster := m.IsMaster()
	m.setMaster(m.selfID)
	if !wasMaster {
		log.Printf("[eleição] nenhuma API de ID menor respondeu: EU sou a mestre")
	}

	var wg sync.WaitGroup
	for _, p := range m.peers {
		wg.Add(1)
		go func(p peerAPI) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, checkTimeout)
			defer cancel()
			if _, err := p.client.Coordinator(cctx, &rifapb.CoordinatorRequest{MasterId: int32(m.selfID)}); err != nil {
				log.Printf("[eleição] não consegui me anunciar para %s: %v", p.name, err)
			}
		}(p)
	}
	wg.Wait()

	if !wasMaster {
		go m.masterCheck(context.WithoutCancel(ctx))
	}
}

// ---------------------------------------------------------------------------
// Recuperação do banco (só a mestre, com votação)
// ---------------------------------------------------------------------------

// maybeFailover: se NENHUM banco é primário e existe uma réplica viva, pede
// votos às outras APIs e, com a maioria, promove a réplica.
//
// Antes, checa os bancos de novo na hora: promover é uma ação séria (não dá
// para desfazer), então não confiamos só na checagem anterior.
func (m *Monitor) maybeFailover(ctx context.Context) {
	if !m.failoverMu.TryLock() {
		return // já tem uma promoção em andamento
	}
	defer m.failoverMu.Unlock()

	if m.anyDB(Primary) || !m.anyDB(Replica) {
		return
	}

	fresh := m.checkAllDBs(ctx)
	var candidate *database
	for i := range m.dbs {
		st := fresh[m.dbs[i].name]
		m.apply(m.dbs[i].name, st, m.selfID)
		if st == Primary {
			return // alarme falso: tem primário
		}
		if st == Replica && candidate == nil {
			candidate = &m.dbs[i]
		}
	}
	if candidate == nil {
		log.Printf("[failover] nenhum banco disponível para promover")
		return
	}

	votes, alive := m.collectVotes(ctx, candidate.name)
	needed := alive/2 + 1
	if votes < needed {
		log.Printf("[failover] votação PERDIDA para promover %s: %d de %d APIs vivas (precisa de %d). Não promovo.",
			candidate.name, votes, alive, needed)
		return
	}
	if !m.IsMaster() {
		log.Printf("[failover] deixei de ser a mestre durante a votação: não promovo")
		return
	}

	log.Printf("[failover] votação GANHA (%d de %d APIs vivas): promovendo %s", votes, alive, candidate.name)
	if err := promote(ctx, candidate.url); err != nil {
		log.Printf("[failover] falha ao promover %s: %v", candidate.name, err)
		return
	}
	m.apply(candidate.name, Primary, m.selfID)
	log.Printf("[failover] %s agora é o banco PRIMÁRIO", candidate.name)
}

// collectVotes pede o voto de todas as outras APIs (em paralelo) e devolve
// quantos SIM recebeu e quantas APIs estão vivas (as que responderam, sim ou
// não, mais eu). Meu voto já conta: eu acabei de conferir os bancos.
//
// A maioria é das VIVAS: com 3 vivas precisa de 2, com 2 vivas precisa de 2,
// e uma API sozinha decide sozinha (1 de 1).
func (m *Monitor) collectVotes(ctx context.Context, candidate string) (votes, alive int) {
	votes, alive = 1, 1

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range m.peers {
		if m.stateOf(p.name) == Down {
			continue
		}
		wg.Add(1)
		go func(p peerAPI) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, voteTimeout)
			defer cancel()
			resp, err := p.client.RequestVote(cctx, &rifapb.VoteRequest{FromId: int32(m.selfID), CandidateDb: candidate})
			if err != nil {
				log.Printf("[failover] %s não votou (conta como fora do ar): %v", p.name, err)
				return
			}
			mu.Lock()
			alive++
			if resp.Granted {
				votes++
			}
			mu.Unlock()
			if resp.Granted {
				log.Printf("[failover] %s votou SIM para promover %s", p.name, candidate)
			} else {
				log.Printf("[failover] %s votou NÃO: %s", p.name, resp.Reason)
			}
		}(p)
	}
	wg.Wait()
	return votes, alive
}

// promote transforma uma réplica em primário:
//
//	SELECT pg_promote(true, 10);  -> sai do modo réplica, esperando até 10s
//	CHECKPOINT;                   -> grava já o novo timeline no pg_control.
//	                                 O entrypoint do banco compara timelines
//	                                 lendo o pg_control para decidir quem é o
//	                                 primário de verdade; sem o checkpoint
//	                                 ele ainda veria o timeline antigo.
//
// Se os 10s estourarem, o pg_promote devolve false mas a promoção continua
// por baixo. Por isso, nesse caso, confiro pg_is_in_recovery() antes de
// desistir.
func promote(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, promoteTimeout)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_promote(true, $1)`, promoteWait).Scan(&ok); err != nil {
		return fmt.Errorf("pg_promote: %w", err)
	}
	if !ok {
		var inRecovery bool
		if err := conn.QueryRow(ctx, `SELECT pg_is_in_recovery()`).Scan(&inRecovery); err != nil || inRecovery {
			return fmt.Errorf("pg_promote não terminou em %ds", promoteWait)
		}
	}
	if _, err := conn.Exec(ctx, `CHECKPOINT`); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	return nil
}

func (m *Monitor) stateOf(target string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state[target]
}

func (m *Monitor) anyDB(state string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, db := range m.dbs {
		if m.state[db.name] == state {
			return true
		}
	}
	return false
}

func isDB(target string) bool { return len(target) > 2 && target[:2] == "db" }

// ---------------------------------------------------------------------------
// Lado servidor do gRPC (as outras APIs chamam estes métodos)
// ---------------------------------------------------------------------------

func (m *Monitor) Ping(ctx context.Context, req *rifapb.PingRequest) (*rifapb.PingResponse, error) {
	return &rifapb.PingResponse{NodeId: int32(m.selfID), MasterId: int32(m.MasterID())}, nil
}

func (m *Monitor) ReportStatus(ctx context.Context, r *rifapb.StatusReport) (*rifapb.StatusAck, error) {
	// Se alguém diz que EU caí, é engano (estou respondendo): ignoro.
	if r.Target != apiName(m.selfID) {
		m.apply(r.Target, r.State, int(r.ReporterId))
	}
	return &rifapb.StatusAck{}, nil
}

// Election: uma API de ID maior está fazendo eleição. Respondo "estou viva"
// e, como tenho prioridade, faço a minha própria eleição (ou, se já sou a
// mestre, só me anuncio de novo).
func (m *Monitor) Election(ctx context.Context, r *rifapb.ElectionRequest) (*rifapb.ElectionResponse, error) {
	from := int(r.FromId)
	if p, ok := m.peer(from); ok {
		m.apply(p.name, Up, from)
	}
	if m.IsMaster() {
		if p, ok := m.peer(from); ok {
			go func() {
				cctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
				defer cancel()
				_, _ = p.client.Coordinator(cctx, &rifapb.CoordinatorRequest{MasterId: int32(m.selfID)})
			}()
		}
	} else {
		go m.elect(context.Background())
	}
	return &rifapb.ElectionResponse{NodeId: int32(m.selfID)}, nil
}

// Coordinator: alguém se anunciou como mestre. Se ela tem ID menor que o
// meu, aceito. Se tem ID MAIOR, ela achou que eu estava fora do ar: faço
// uma eleição para retomar o posto.
func (m *Monitor) Coordinator(ctx context.Context, r *rifapb.CoordinatorRequest) (*rifapb.CoordinatorAck, error) {
	leader := int(r.MasterId)
	switch {
	case leader == m.selfID:
	case leader > m.selfID:
		log.Printf("[eleição] api%d se anunciou mestre, mas eu (ID menor) estou viva: nova eleição", leader)
		go m.elect(context.Background())
	default:
		m.setMaster(leader)
		if p, ok := m.peer(leader); ok {
			m.apply(p.name, Up, leader)
		}
	}
	return &rifapb.CoordinatorAck{}, nil
}

// RequestVote: a mestre quer promover uma réplica. Confiro por conta própria
// (conectando direto nos bancos) antes de concordar.
func (m *Monitor) RequestVote(ctx context.Context, r *rifapb.VoteRequest) (*rifapb.VoteResponse, error) {
	deny := func(format string, a ...any) (*rifapb.VoteResponse, error) {
		reason := fmt.Sprintf(format, a...)
		log.Printf("[failover] voto NÃO para api%d promover %s: %s", r.FromId, r.CandidateDb, reason)
		return &rifapb.VoteResponse{NodeId: int32(m.selfID), Granted: false, Reason: reason}, nil
	}

	if master := m.MasterID(); master != 0 && master != int(r.FromId) {
		return deny("para mim a mestre é a api%d", master)
	}

	states := m.checkAllDBs(ctx)
	for name, st := range states {
		if st == Primary {
			return deny("eu vejo %s como primário", name)
		}
	}
	if states[r.CandidateDb] != Replica {
		return deny("eu vejo %s como %q, não como réplica", r.CandidateDb, states[r.CandidateDb])
	}

	log.Printf("[failover] voto SIM para api%d promover %s", r.FromId, r.CandidateDb)
	return &rifapb.VoteResponse{NodeId: int32(m.selfID), Granted: true}, nil
}

// ---------------------------------------------------------------------------
// Visão do cluster (rota GET /cluster)
// ---------------------------------------------------------------------------

type Item struct {
	Name      string    `json:"nome"`
	State     string    `json:"estado"`
	ChangedAt time.Time `json:"mudou_em,omitempty"`
}

type View struct {
	Me        int       `json:"eu"`
	Master    int       `json:"mestre"`
	MySlots   []int     `json:"meus_segundos"`
	LastCheck time.Time `json:"ultima_checagem"`
	APIs      []Item    `json:"apis"`
	DBs       []Item    `json:"bancos"`
}

func (m *Monitor) View() View {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := View{
		Me:        m.selfID,
		Master:    m.masterID,
		MySlots:   SlotSeconds(m.selfID, m.totalAPIs),
		LastCheck: m.lastCheck,
	}
	for name, st := range m.state {
		item := Item{Name: name, State: st, ChangedAt: m.changedAt[name]}
		if isDB(name) {
			v.DBs = append(v.DBs, item)
		} else {
			v.APIs = append(v.APIs, item)
		}
	}
	sort.Slice(v.APIs, func(i, j int) bool { return v.APIs[i].Name < v.APIs[j].Name })
	sort.Slice(v.DBs, func(i, j int) bool { return v.DBs[i].Name < v.DBs[j].Name })
	return v
}
