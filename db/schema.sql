-- Tabelas da rifa. Roda UMA vez, na criação do banco primário
-- (o docker-compose monta este arquivo em /docker-entrypoint-initdb.d/).
-- A réplica recebe tudo por cópia, não roda este arquivo.
--
-- DATAS: todas são TIMESTAMP *sem* fuso horário e SEMPRE em UTC.
-- O default "now() AT TIME ZONE 'utc'" pega a hora atual já convertida para
-- UTC, independente do fuso configurado no servidor. A API também força a
-- sessão em UTC (ver internal/store/store.go). Quem mostra a data no fuso
-- do usuário é o frontend.

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,              -- bcrypt, nunca a senha pura
    created_at    TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc')
);

CREATE TABLE IF NOT EXISTS raffles (
    id            BIGSERIAL PRIMARY KEY,
    owner_id      BIGINT NOT NULL REFERENCES users(id),
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    prize         TEXT NOT NULL,
    price_cents   BIGINT NOT NULL CHECK (price_cents >= 0),  -- dinheiro em centavos
    total_numbers INT NOT NULL DEFAULT 200 CHECK (total_numbers > 0),
    status        TEXT NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft', 'open', 'closed', 'drawn')),
    winner_number INT,                                       -- NULL até sortear
    created_at    TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc')
);

CREATE TABLE IF NOT EXISTS tickets (
    raffle_id     BIGINT NOT NULL REFERENCES raffles(id),
    number        INT NOT NULL,
    buyer_name    TEXT NOT NULL,
    buyer_contact TEXT NOT NULL,
    bought_at     TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    -- A chave primária (raffle_id, number) é o que IMPEDE vender o mesmo
    -- número duas vezes: o segundo INSERT falha com violação de unicidade.
    PRIMARY KEY (raffle_id, number)
);
