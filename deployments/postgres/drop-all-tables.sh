#!/bin/sh
set -eu

container_name="${OMNIMAM_POSTGRES_CONTAINER:-omnimam-postgres}"
volume_name="${OMNIMAM_POSTGRES_VOLUME:-deployments_omnimam_postgres_data}"

if [ "${1:-}" != "--force" ]; then
  printf 'This will delete every table in the public schema stored in %s.\n' "$volume_name"
  printf 'Re-run with --force to continue.\n'
  exit 2
fi

if ! docker volume inspect "$volume_name" >/dev/null 2>&1; then
  printf 'PostgreSQL volume not found: %s\n' "$volume_name" >&2
  exit 1
fi

if ! docker container inspect "$container_name" >/dev/null 2>&1; then
  printf 'PostgreSQL container not found: %s\n' "$container_name" >&2
  exit 1
fi

mounted_volume="$({
  docker container inspect \
    --format '{{range .Mounts}}{{if eq .Destination "/var/lib/postgresql/data"}}{{.Name}}{{end}}{{end}}' \
    "$container_name"
} 2>/dev/null)"

if [ "$mounted_volume" != "$volume_name" ]; then
  printf 'Container %s uses volume %s, expected %s.\n' \
    "$container_name" "${mounted_volume:-<none>}" "$volume_name" >&2
  exit 1
fi

if [ "$(docker container inspect --format '{{.State.Running}}' "$container_name")" != "true" ]; then
  printf 'PostgreSQL container is not running: %s\n' "$container_name" >&2
  exit 1
fi

docker exec -i "$container_name" sh -c \
  'exec psql --set=ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB"' <<'SQL'
DO $drop_tables$
DECLARE
  table_name text;
BEGIN
  FOR table_name IN
    SELECT tablename
    FROM pg_catalog.pg_tables
    WHERE schemaname = 'public'
  LOOP
    EXECUTE format('DROP TABLE IF EXISTS %I.%I CASCADE', 'public', table_name);
  END LOOP;
END
$drop_tables$;
SQL

printf 'Deleted every table in the public schema from volume %s.\n' "$volume_name"
