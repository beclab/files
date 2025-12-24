#!/bin/bash

echo "DB_HOST= $DB_HOST"
echo "DB_PORT= $DB_PORT"
echo "DB_NAME1= $DB_NAME1"
echo "DB_NAME2= $DB_NAME2"
echo "DB_USER= $DB_USER"
echo "DB_PASSWORD= $DB_PASSWORD"

SQL_FILE1="init_pgdata/ccnet.sql"
SQL_FILE2="init_pgdata/seafile.sql"
SQL_FILE3="init_pgdata/patch.sql"

export PGPASSWORD="$DB_PASSWORD"

# check and wait for database initialized
check_db_availability() {
    local db_name=$1
    echo -e "Checking for the availability of PostgreSQL Server deployment"
    echo -n ">> Waiting for ${db_name}..."

    until psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$db_name" -c "SELECT 1" &>/dev/null; do
        sleep 1
        echo -n "-"
    done

    echo -e "\n>> ${db_name} is available"
    sleep 15
}

check_db_availability "$DB_NAME1"
check_db_availability "$DB_NAME2"

# recheck ccnet for sure
check_db_availability "$DB_NAME1"
echo -e "\n>> PostgreSQL DB Server has started"

# check if tables exist to make decision of creating them or not
TABLE_NAME1="emailuser"
TABLE_NAME2="repo"

to_lower() {
    echo "$1" | tr '[:upper:]' '[:lower:]'
}

table_name1_lower=$(to_lower "$TABLE_NAME1")
table_exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME1" -t -c \
"SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_tables WHERE tablename = '$table_name1_lower' AND schemaname = 'public');" \
| tr -d '[:space:]')

if [ "$table_exists" = "f" ]; then
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME1" -f "$SQL_FILE1"
    echo "$DB_NAME1 initialize finished"
else
    echo "No initialization needed for $DB_NAME1"
fi

table_name2_lower=$(to_lower "$TABLE_NAME2")
table_exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME2" -t -c \
"SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_tables WHERE tablename = '$table_name2_lower' AND schemaname = 'public');" \
| tr -d '[:space:]')

if [ "$table_exists" = "f" ]; then
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME2" -f "$SQL_FILE2"
    echo "$DB_NAME2 initialize finished"
else
    echo "No initialization needed for $DB_NAME2"
fi

psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME1" -f "$SQL_FILE3"
    echo "sql patch finished"

unset PGPASSWORD

# old data to new data
./seahub_init
