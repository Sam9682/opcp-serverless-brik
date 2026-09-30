You are an autonomous IT Operater agent with access to execute shell commands on a Linux server.
Please display real-time logs from all running Docker containers for the specified user instance.
IMPORTANT : 
- all commands have to be executed in the application located {{APPLICATION_FOLDER}}.
- Execute all steps to deploy with the environment variable USER_ID={{USER_ID}}

#### 1. Calculate HTTP Ports, which are the ports used by the docker containers of the application. Use the following command:

source {{APPLICATION_FOLDER}}/conf/deploy.ini
RANGE_START=${RANGE_START:-6000}
RANGE_RESERVED=${RANGE_RESERVED:-100}
RANGE_PORTS_PER_APPLICATION=${RANGE_PORTS_PER_APPLICATION:-12}
if ! [[ "$USER_ID" =~ ^[0-9]+$ ]]; then
    USER_ID=0
fi
PORT_NAMES=(
    HTTPS_PORT HTTP_PORT
    HTTPS_PORT1 HTTP_PORT1
    HTTPS_PORT2 HTTP_PORT2
    HTTPS_PORT3 HTTP_PORT3
    HTTPS_PORT4 HTTP_PORT4
    HTTPS_PORT5 HTTP_PORT5
)
PORT_RANGE_BEGIN=$((RANGE_START + USER_ID * RANGE_RESERVED))
base=$((PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION))
offset=0
for name in "${PORT_NAMES[@]}"; do
    printf -v "$name" '%s' "$((base + offset))"
    export "$name"
    offset=$((offset + 1))
done

#### 2. Show Logs using docker-compose command. Use the following commands to display the logs of the application:

containers=$(docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*")
if [[ -n "$containers" ]]; then
    docker logs $containers
    echo "Services retrieved logs successfully ✅"
else
    echo "No running containers found for USER_ID: $USER_ID"
fi

**Expected Output:**
- Color-coded output by service
- Timestamps for each log entry

**Summary:** display the output. 