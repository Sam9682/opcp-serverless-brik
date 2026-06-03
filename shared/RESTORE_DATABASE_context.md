You are an autonomous IT Operater agent with access to execute shell commands on a Linux server.
Please restore the PostgreSQL database by executing the following steps in sequence. 
IMPORTANT : 
- all commands have to be executed in the application located {{APPLICATION_FOLDER}}.
- Execute all steps with the environment variable USER_ID={{USER_ID}}
- The database is PostgreSQL and configuration details should be retrieved from docker-compose.yaml postgresql section and related scripts
- Backups are stored in OVH S3 using IPv4 addressing
- The S3 bucket path follows the pattern: s3://bucket-name/{{APPLICATION_NAME}}/{{IPv4_ADDRESS}}/backups/
- {{BACKUP_FILE}} parameter should specify which backup file to restore (optional, defaults to latest)

**Execute these steps:**

#### 1. Check Prerequisites, docker, docker-compose, and s3cmd have to be installed on the current server. You can use the following commands:

command -v docker || exit 1
command -v docker-compose || exit 1
command -v s3cmd || exit 1

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

#### 3. Get Server IPv4 Address:

SERVER_IP=$(curl -s -4 ifconfig.me || hostname -I | awk '{print $1}')

#### 4. List Available Backups from OVH S3:

S3_BUCKET="your-bucket-name"
S3_PATH="s3://${S3_BUCKET}/${NAME_OF_APPLICATION}/${SERVER_IP}/backups/"

echo "Available backups:"
s3cmd ls $S3_PATH

#### 5. Download Backup File from S3:

RESTORE_DIR="./restore"
mkdir -p $RESTORE_DIR

# If BACKUP_FILE is not specified, get the latest backup
if [ -z "$BACKUP_FILE" ]; then
    BACKUP_FILE=$(s3cmd ls $S3_PATH | sort | tail -n 1 | awk '{print $4}')
    echo "No backup file specified, using latest: $BACKUP_FILE"
fi

LOCAL_BACKUP_FILE="${RESTORE_DIR}/$(basename $BACKUP_FILE)"
s3cmd get $BACKUP_FILE $LOCAL_BACKUP_FILE

# Verify download was successful
if [ ! -f "$LOCAL_BACKUP_FILE" ]; then
    echo "ERROR: Failed to download backup file from S3"
    exit 1
fi

#### 6. Stop Application Services (to prevent database conflicts):

HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker-compose -p "$NAME_OF_APPLICATION-$USER_ID-$HTTPS_PORT" -f docker-compose.yml --env-file .env.prod stop

#### 7. Create Backup of Current Database (safety measure):

SAFETY_BACKUP="${RESTORE_DIR}/pre_restore_backup_$(date +%Y%m%d_%H%M%S).sql.gz"
docker exec $DB_CONTAINER pg_dump -U $DB_USER $DB_NAME | gzip > $SAFETY_BACKUP
echo "Safety backup created: $SAFETY_BACKUP"

#### 8. Drop and Recreate Database:

docker exec $DB_CONTAINER psql -U $DB_USER -c "DROP DATABASE IF EXISTS $DB_NAME;"
docker exec $DB_CONTAINER psql -U $DB_USER -c "CREATE DATABASE $DB_NAME;"

#### 9. Restore Database from Backup File:

gunzip -c $LOCAL_BACKUP_FILE | docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME

# Verify restore was successful
if [ $? -eq 0 ]; then
    echo "Database restored successfully"
else
    echo "ERROR: Failed to restore database"
    echo "Attempting to restore from safety backup..."
    gunzip -c $SAFETY_BACKUP | docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME
    exit 1
fi

#### 10. Restart Application Services:

HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker-compose -p "$NAME_OF_APPLICATION-$USER_ID-$HTTPS_PORT" -f docker-compose.yml --env-file .env.prod up -d

#### 11. Verify Database Connection:

sleep 5
docker exec $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "SELECT version();"

Finally, display the restore information:
- Restored from: $BACKUP_FILE
- Local file: $LOCAL_BACKUP_FILE
- Safety backup: $SAFETY_BACKUP
- Database: $DB_NAME
- Status: Restoration completed successfully
