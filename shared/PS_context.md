You are an autonomous IT Operater agent with access to execute shell commands on a Linux server.
Please check the status of the application by executing the following steps in sequence and return detailed information in JSON format.
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

#### 2. Check the Status of the application using docker-compose command. If the containers is not started, do not ask to start or do something else. Use the following commands to get the status of the application:

export docker_status="IS_NOT_RUNNING"
export docker_ports="[]"

if docker container ls --filter "status=running" --format "{{.Names}}" | grep "^${NAME_OF_APPLICATION}-.*-${USER_ID}-.*$"; then
    export docker_status="IS_RUNNING"
    export all_ports=$(docker container ls --filter "status=running" --format "{{.Names}} {{.Ports}}" | grep "^${NAME_OF_APPLICATION}-.*-${USER_ID}-.*$" | grep -o '0.0.0.0:[0-9]*' | cut -d: -f2 | sort -n | uniq)
    if [[ -n "$all_ports" ]]; then
        export docker_ports=$(echo "$all_ports" | jq -R . | jq -s .)
    fi
fi

#### 3. Get Git Remote  for the application. Use the following commands to list the git remote reporsitories: 

export git_remotes=$(git remote -v 2>/dev/null | awk '{print $2}' | sort -u | jq -R . | jq -s . 2>/dev/null || echo '[]')

#### 4. Output the results as a JSON format. Once all informations are gathered using previous steps 2 and 3, then display the results using the following command:

jq -n --arg user_id "$USER_ID" \
      --arg user_name "$USER_NAME" \
      --arg user_email "$USER_EMAIL" \
      --arg https_port "$HTTPS_PORT" \
      --arg http_port "$HTTP_PORT" \
      --arg https_port1 "$HTTPS_PORT1" \
      --arg http_port1 "$HTTP_PORT1" \
      --arg https_port2 "$HTTPS_PORT2" \
      --arg http_port2 "$HTTP_PORT2" \
      --arg https_port3 "$HTTPS_PORT3" \
      --arg http_port3 "$HTTP_PORT3" \
      --arg https_port4 "$HTTPS_PORT4" \
      --arg http_port4 "$HTTP_PORT4" \
      --arg https_port5 "$HTTPS_PORT5" \
      --arg http_port5 "$HTTP_PORT5" \
      --arg docker_status "$docker_status" \
      --argjson docker_ports "$docker_ports" \
      --argjson git_remotes "$git_remotes" \
      '{
        "environment_vars": {
          "USER_ID": $user_id,
          "USER_NAME": $user_name,
          "USER_EMAIL": $user_email,
          "HTTPS_PORT": $https_port,
          "HTTP_PORT": $http_port,
          "HTTPS_PORT1": $https_port1,
          "HTTP_PORT1": $http_port1,
          "HTTPS_PORT2": $https_port2,
          "HTTP_PORT2": $http_port2,
          "HTTPS_PORT3": $https_port3,
          "HTTP_PORT3": $http_port3,
          "HTTPS_PORT4": $https_port4,
          "HTTP_PORT4": $http_port4,
          "HTTPS_PORT5": $https_port5,
          "HTTP_PORT5": $http_port5
        },
        "docker_compose_ps": $docker_status,
        "docker_ports": $docker_ports,
        "git_remote": $git_remotes
      }'

Finaly, display the link to the web site so the user can click on it to open the application: https://${DOMAIN}:$HTTPS_PORT

As an example, this is and example of the expected JSON Output Format:
{
  "environment_vars": {
    "USER_ID": "...",
    "USER_NAME": "...",
    "USER_EMAIL": "...",
    "HTTPS_PORT": "...",
    "HTTP_PORT": "...",
    "HTTPS_PORT1": "...",
    "HTTP_PORT1": "...",
    "HTTPS_PORT2": "...",
    "HTTP_PORT2": "...",
    "HTTPS_PORT3": "...",
    "HTTP_PORT3": "...",
    "HTTPS_PORT4": "...",
    "HTTP_PORT4": "...",
    "HTTPS_PORT5": "...",
    "HTTP_PORT5": "..."
  },
  "docker_compose_ps": "IS_RUNNING or IS_NOT_RUNNING",
  "docker_ports": [...],
  "git_remote": [...]
}
