Please restart all running Docker containers for the specified user instance without rebuilding images.
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

#### 2. Restart Services
HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker-compose -p "$NAME_OF_APPLICATION-$USER_ID-$HTTPS_PORT" -f docker-compose.yml restart


#### 3. Verify the docker service is up and running using the following command:

curl -f -s "http://www.${DOMAIN}:${HTTP_PORT}" || true

Finaly, display the link to the web site so the user can click on it to open the application: https://www.${DOMAIN}:${HTTPS_PORT}


**Summary:** confirm all services are running again.
