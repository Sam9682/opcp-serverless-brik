You are an autonomous IT Operater agent with access to execute shell commands on a Linux server.
Please clean the PostgreSQL database by executing the following steps in sequence. 
IMPORTANT : 
- all commands have to be executed in the application located {{APPLICATION_FOLDER}}.
- Execute all steps with the environment variable USER_ID={{USER_ID}}
- The database is PostgreSQL and configuration details should be retrieved from docker-compose.yaml postgresql section and related scripts
- This operation will drop and recreate the database, removing all data
- A backup will be created before cleaning for safety

**Execute these steps:**

#### 1. Check Prerequisites, docker and docker-compose have to be installed on the current server. You can use the following commands:

command -v docker || exit 1
command -v docker-compose || exit 1

#### 2. Retrieve Database Configuration from docker-compose.yaml. Extract PostgreSQL connection details:

source ./conf/deploy.ini
if ! [[ "$USER_ID" =~ ^[0-9]+$ ]]; then
    USER_ID=0
fi

# Extract database configuration from docker-compose.yaml
DB_CONTAINER=$(docker-compose -f docker-compose.yml ps -q postgres 2>/dev/null || docker-compose -f docker-compose.yml ps -q db 2>/dev/null)
DB_NAME=$(grep -A 10 "postgres:" docker-compose.yml | grep "POSTGRES_DB" | cut -d "=" -f2 | tr -d " \"'")
DB_USER=$(grep -A 10 "postgres:" docker-compose.yml | grep "POSTGRES_USER" | cut -d "=" -f2 | tr -d " \"'")
DB_PASSWORD=$(grep -A 10 "postgres:" docker-compose.yml | grep "POSTGRES_PASSWORD" | cut -d "=" -f2 | tr -d " \"'")

# If variables are empty, try from .env.prod
if [ -z "$DB_NAME" ] || [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ]; then
    source .env.prod 2>/dev/null || true
fi

#### 3. Verify Database Container is Running:

if [ -z "$DB_CONTAINER" ]; then
    echo "ERROR: PostgreSQL container not found"
    exit 1
fi

docker ps | grep $DB_CONTAINER > /dev/null
if [ $? -ne 0 ]; then
    echo "ERROR: PostgreSQL container is not running"
    exit 1
fi

#### 4. Create Safety Backup Before Cleaning:

BACKUP_DIR="./backups"
mkdir -p $BACKUP_DIR
SAFETY_BACKUP="${BACKUP_DIR}/pre_clean_backup_$(date +%Y%m%d_%H%M%S).sql.gz"

docker exec $DB_CONTAINER pg_dump -U $DB_USER $DB_NAME | gzip > $SAFETY_BACKUP

# Verify backup was created
if [ ! -f "$SAFETY_BACKUP" ]; then
    echo "ERROR: Safety backup failed, aborting clean operation"
    exit 1
fi

echo "Safety backup created: $SAFETY_BACKUP"

#### 5. Stop Application Services (to prevent database conflicts):

export PORT_RANGE_BEGIN=$((RANGE_START+USER_ID*RANGE_RESERVED))
export HTTP_PORT=$((PORT_RANGE_BEGIN+APPLICATION_IDENTITY_NUMBER*RANGE_PORTS_PER_APPLICATION))
export HTTPS_PORT=$((HTTP_PORT+1))
export HTTP_PORT2=$(($HTTPS_PORT+1))
export HTTPS_PORT2=$(($HTTP_PORT2+1))

HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker-compose -p "$NAME_OF_APPLICATION-$USER_ID-$HTTPS_PORT" -f docker-compose.yml --env-file .env.prod stop

#### 6. Terminate Active Database Connections:

docker exec $DB_CONTAINER psql -U $DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();"

#### 7. Drop and Recreate Database:

docker exec $DB_CONTAINER psql -U $DB_USER -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;"

if [ $? -eq 0 ]; then
    echo "Database $DB_NAME dropped successfully"
else
    echo "ERROR: Failed to drop database"
    exit 1
fi

docker exec $DB_CONTAINER psql -U $DB_USER -d postgres -c "CREATE DATABASE $DB_NAME;"

if [ $? -eq 0 ]; then
    echo "Database $DB_NAME created successfully"
else
    echo "ERROR: Failed to create database"
    exit 1
fi

#### 8. Apply Initial Schema (if initialization scripts exist):

if [ -d "./scripts/init" ]; then
    for script in ./scripts/init/*.sql; do
        if [ -f "$script" ]; then
            echo "Applying initialization script: $script"
            docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME < $script
        fi
    done
fi

#### 9. Restart Application Services:

HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker-compose -p "$NAME_OF_APPLICATION-$USER_ID-$HTTPS_PORT" -f docker-compose.yml --env-file .env.prod up -d

#### 10. Verify Database is Clean and Accessible:

sleep 5
docker exec $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';"

Finally, display the clean operation information:
- Database: $DB_NAME
- Safety backup: $SAFETY_BACKUP
- Backup size: $(du -h $SAFETY_BACKUP | cut -f1)
- Status: Database cleaned and recreated successfully
- Note: All data has been removed. Use the safety backup to restore if needed.
