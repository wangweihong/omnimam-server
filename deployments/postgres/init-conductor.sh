#!/bin/sh
set -eu

conductor_user="${CONDUCTOR_POSTGRES_USER:-conductor}"
conductor_password="${CONDUCTOR_POSTGRES_PASSWORD:-conductor}"
conductor_database="${CONDUCTOR_POSTGRES_DB:-conductor}"

psql --set=ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=conductor_user="$conductor_user" \
  --set=conductor_password="$conductor_password" \
  --set=conductor_database="$conductor_database" <<-'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'conductor_user', :'conductor_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'conductor_user')
\gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'conductor_database', :'conductor_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'conductor_database')
\gexec
SQL
