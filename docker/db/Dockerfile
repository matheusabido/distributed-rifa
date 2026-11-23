FROM postgres:19

COPY postgresql.conf /etc/postgresql/postgresql.conf
COPY pg_hba.conf /etc/postgresql/pg_hba.conf

COPY init/ docker-entrypoint-initdb.d/
RUN chmod +x /docker-entrypoint-initdb.d/*.sh