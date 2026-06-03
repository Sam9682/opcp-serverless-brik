You are an autonomous IT Operater agent with access to execute shell commands on a Linux server.
Please backup the PostgreSQL database by executing the following steps in sequence. 
IMPORTANT : 
- all commands have to be executed in the application located {{APPLICATION_FOLDER}}.
- Execute all steps with the environment variable USER_ID={{USER_ID}}
- The database is PostgreSQL and configuration details should be retrieved from docker-compose.yaml postgresql section and related scripts
- Backups are stored in OVH S3 using IPv4 addressing
- The S3 bucket path follows the pattern: s3://bucket-name/{{APPLICATION_NAME}}/{{IPv4_ADDRESS}}/backups/

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

#### 4. Create Backup Directory and Generate Backup File:

BACKUP_DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="./backups"
mkdir -p $BACKUP_DIR
BACKUP_FILE="${BACKUP_DIR}/${NAME_OF_APPLICATION}_${USER_ID}_${BACKUP_DATE}.sql.gz"

#### 5. Execute PostgreSQL Backup using pg_dump:

docker exec $DB_CONTAINER pg_dump -U $DB_USER $DB_NAME | gzip > $BACKUP_FILE

# Verify backup file was created
if [ ! -f "$BACKUP_FILE" ]; then
    echo "ERROR: Backup file was not created"
    exit 1
fi

#### 6. Upload Backup to OVH S3:

S3_BUCKET="your-bucket-name"
S3_PATH="s3://${S3_BUCKET}/${NAME_OF_APPLICATION}/${SERVER_IP}/backups/"

s3cmd put $BACKUP_FILE $S3_PATH --acl-private

# Verify upload was successful
if [ $? -eq 0 ]; then
    echo "Backup successfully uploaded to ${S3_PATH}"
else
    echo "ERROR: Failed to upload backup to S3"
    exit 1
fi

#### 7. Record Backup in Deployment History:

# Get deployment ID for this application
DEPLOYMENT_ID=$(docker exec postgres psql -U $DB_USER -d ai_swautomorph -t -c \
  "SELECT id FROM deployments WHERE user_id = $USER_ID AND application_name = '$NAME_OF_APPLICATION' ORDER BY updated_at DESC LIMIT 1" | xargs)

# Add backup entry to deployment history
if [ ! -z "$DEPLOYMENT_ID" ]; then
    python3 /home/ubuntu/ai-swautomorph/scripts/add_backup_to_deployment.py \
      --deployment-id $DEPLOYMENT_ID \
      --backup-file "$(basename $BACKUP_FILE)" \
      --s3-location "${S3_PATH}$(basename $BACKUP_FILE)" \
      --backup-size "$(du -h $BACKUP_FILE | cut -f1)" \
      --server-ip "$SERVER_IP" \
      --user-id "$USER_ID"
    
    if [ $? -eq 0 ]; then
        echo "✓ Backup recorded in deployment history"
    else
        echo "⚠ Warning: Failed to record backup in deployment history (backup still successful)"
    fi
else
    echo "⚠ Warning: Deployment not found, backup not recorded in history (backup still successful)"
fi

#### 8. Clean up old local backups (keep last 7 days):

find $BACKUP_DIR -name "*.sql.gz" -type f -mtime +7 -delete

Finally, display the backup information:
- Backup file: $BACKUP_FILE
- S3 location: ${S3_PATH}$(basename $BACKUP_FILE)
- Backup size: $(du -h $BACKUP_FILE | cut -f1)
- Deployment ID: $DEPLOYMENT_ID
- Recorded in history: Yes (visible in Deployments Management dashboard)
