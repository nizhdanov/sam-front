#!/bin/sh
set -eu

create_db() {
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<SQL
CREATE ROLE $2 LOGIN PASSWORD '$3';
CREATE DATABASE $1 OWNER $2;
REVOKE ALL ON DATABASE $1 FROM PUBLIC;
SQL
}

create_db inventory_db "$INVENTORY_DB_USER" "$INVENTORY_DB_PASSWORD"
create_db license_db "$LICENSE_DB_USER" "$LICENSE_DB_PASSWORD"
create_db chat_db "$CHAT_DB_USER" "$CHAT_DB_PASSWORD"
