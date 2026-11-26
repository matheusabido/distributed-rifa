FROM postgres:19

COPY postgresql.conf /etc/postgresql/postgresql.conf
COPY pg_hba.conf /etc/postgresql/pg_hba.conf

COPY init/ docker-entrypoint-initdb.d/
RUN chmod +x /docker-entrypoint-initdb.d/*.sh

# Decide se este container sobe como primário ou réplica (ver o script).
COPY rifa-entrypoint.sh /usr/local/bin/rifa-entrypoint.sh
RUN chmod +x /usr/local/bin/rifa-entrypoint.sh
ENTRYPOINT ["rifa-entrypoint.sh"]
CMD ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]
