// Package monitor cuida da saúde do sistema: quem está vivo, quem é a API
// mestre e qual banco é o primário.
//
// Resumo das regras:
//
//   - Hierarquia: API 1 > API 2 > API 3. A MESTRE é a de menor ID que está
//     viva. Se a 1 cair, a 2 assume; quando a 1 volta, ela retoma.
//   - Horários: cada API checa num segundo diferente (ver schedule.go). Com 3
//     APIs: API 1 em 1, 7, 13...; API 2 em 3, 9, 15...; API 3 em 5, 11, 17...
//   - Na sua vez, a API faz Ping (gRPC) nas outras APIs e testa os bancos.
//   - Quem descobrir primeiro uma mudança avisa TODAS as outras (ReportStatus).
//   - Só a MESTRE toma decisão de recuperação: se o banco primário caiu e a
//     réplica está viva, ela promove a réplica a primário.
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

const checkTimeout = 1 * time.Second

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
	state     map[string]string    // "api2" -> "down", "db1" -> "primary"...
	changedAt map[string]time.Time // quando cada estado mudou pela última vez
	lastCheck time.Time

	failoverMu sync.Mutex // garante uma promoção por vez
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

// Run fica rodando para sempre: dorme até o meu horário, checa, repete.
func (m *Monitor) Run(ctx context.Context) {
	log.Printf("[monitor] eu checo nos segundos %v (a cada %v)",
		slotSeconds(m.selfID, m.totalAPIs), period(m.totalAPIs))
	for {
		next := nextSlot(time.Now(), m.selfID, m.totalAPIs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}
		m.checkAll(ctx)
	}
}

// checkAll é "a minha vez de olhar": testa todas as APIs e todos os bancos
// AO MESMO TEMPO (em paralelo), para a checagem durar no máximo ~1s.
func (m *Monitor) checkAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, p := range m.peers {
		wg.Add(1)
		go func(p peerAPI) {
			defer wg.Done()
			m.apply(p.name, m.pingAPI(ctx, p), m.selfID)
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

	m.mu.Lock()
	m.lastCheck = time.Now().UTC()
	m.mu.Unlock()

	if m.IsMaster() {
		m.maybeFailover(ctx)
	}
}

func (m *Monitor) pingAPI(ctx context.Context, p peerAPI) string {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	// WaitForReady: se a conexão está reconectando, espera (até o timeout)
	// em vez de falhar na hora.
	if _, err := p.client.Ping(ctx, &rifapb.PingRequest{FromId: int32(m.selfID)}, grpc.WaitForReady(true)); err != nil {
		return Down
	}
	return Up
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
	oldMaster := m.masterLocked()
	m.state[target] = newState
	m.changedAt[target] = time.Now().UTC()
	newMaster := m.masterLocked()
	m.mu.Unlock()

	if reporter == m.selfID {
		log.Printf("[monitor] DESCOBRI: %s %s -> %s. Avisando as outras APIs.", target, old, newState)
		go m.broadcast(target, newState)
	} else {
		log.Printf("[monitor] a api%d me avisou: %s %s -> %s", reporter, target, old, newState)
	}

	if oldMaster != newMaster {
		log.Printf("[monitor] a API mestre agora é a api%d", newMaster)
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
			continue // não adianta avisar quem caiu; quando voltar, ela checa sozinha
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

// ---------------------------------------------------------------------------
// Hierarquia
// ---------------------------------------------------------------------------

// MasterID é a API de menor ID que não está marcada como fora do ar.
// "unknown" conta como viva: assim a API 2, ao subir, não se declara mestre
// antes de ter checado se a API 1 está de pé.
func (m *Monitor) MasterID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.masterLocked()
}

func (m *Monitor) masterLocked() int {
	master := m.selfID
	for _, p := range m.peers {
		if m.state[p.name] != Down && p.id < master {
			master = p.id
		}
	}
	return master
}

func (m *Monitor) IsMaster() bool { return m.MasterID() == m.selfID }

// ---------------------------------------------------------------------------
// Recuperação do banco (só a mestre)
// ---------------------------------------------------------------------------

// maybeFailover: se NENHUM banco é primário e existe uma réplica viva,
// promove a réplica.
//
// Antes de promover, checa os bancos de novo na hora: promover é uma ação
// séria (não dá para desfazer), então não confiamos só no que os outros
// avisaram.
func (m *Monitor) maybeFailover(ctx context.Context) {
	if !m.failoverMu.TryLock() {
		return // já tem uma promoção em andamento
	}
	defer m.failoverMu.Unlock()

	if m.anyDB(Primary) || !m.anyDB(Replica) {
		return
	}

	var candidate *database
	for i := range m.dbs {
		st := checkDB(ctx, m.dbs[i].url)
		m.apply(m.dbs[i].name, st, m.selfID)
		if st == Primary {
			return // alarme falso: tem primário
		}
		if st == Replica && candidate == nil {
			candidate = &m.dbs[i]
		}
	}
	if candidate == nil {
		log.Printf("[monitor] nenhum banco disponível para promover")
		return
	}

	log.Printf("[monitor] SOU A MESTRE e não há banco primário: promovendo %s", candidate.name)
	if err := promote(ctx, candidate.url); err != nil {
		log.Printf("[monitor] falha ao promover %s: %v", candidate.name, err)
		return
	}
	m.apply(candidate.name, Primary, m.selfID)
	log.Printf("[monitor] %s agora é o banco PRIMÁRIO", candidate.name)
}

// promote transforma uma réplica em primário. pg_promote(true, 10) espera
// até 10s pela promoção terminar.
func promote(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_promote(true, 10)`).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("pg_promote devolveu false")
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
		// Se virei mestre por causa desse aviso, ou se o aviso foi de banco,
		// vejo se preciso agir agora, sem esperar a minha vez.
		if m.IsMaster() {
			go m.maybeFailover(context.Background())
		}
	}
	return &rifapb.StatusAck{}, nil
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
		Master:    m.masterLocked(),
		MySlots:   slotSeconds(m.selfID, m.totalAPIs),
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
