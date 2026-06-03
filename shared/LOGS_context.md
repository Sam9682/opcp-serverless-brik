You are an autonomous IT Operater agent with access to execute shell commands on a Linux server.
Please display real-time logs from all running Docker containers for the specified user instance.
IMPORTANT : 
- all commands have to be executed in the application located {{APPLICATION_FOLDER}}.
- Execute all steps to deploy with the environment variable USER_ID={{USER_ID}}

#### 1. Calculate HTTP Ports, which are the ports used by the docker containers of the application. Use the following command:

source ./conf/deploy.ini
if ! [[ "$USER_ID" =~ ^[0-9]+$ ]]; then
    USER_ID=0
fi
export PORT_RANGE_BEGIN=$((RANGE_START+USER_ID*RANGE_RESERVED))
export HTTP_PORT=$((PORT_RANGE_BEGIN+APPLICATION_IDENTITY_NUMBER*RANGE_PORTS_PER_APPLICATION))
export HTTPS_PORT=$((HTTP_PORT+1))
export HTTP_PORT2=$(($HTTPS_PORT+1))
export HTTPS_PORT2=$(($HTTP_PORT2+1))

#### 2. Show Logs using docker-compose command. Use the following commands to display the logs of the application:

containers=$(docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*")
if [[ -n "$containers" ]]; then
    docker logs $containers
    log_info "Services retrieved logs successfully ✅"
else
    log_warn "No running containers found for USER_ID: $USER_ID"
fi

**Expected Output:**
- Color-coded output by service
- Timestamps for each log entry

**Summary:** display the output. 