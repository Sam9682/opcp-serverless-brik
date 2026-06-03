# Application Deployment Guide

This guide describes the deployment operations for the application. An AI agent can use these instructions to install, manage, and verify the application.

## Prerequisites

Before starting deployment, ensure the following are installed:
- Docker (check with `docker --version`)
- Docker Compose (check with `docker-compose --version`)
- OpenSSL for generating secrets
- curl for health checks
- jq for JSON processing

The script should NOT be run as root user.

## Configuration

The deployment script loads configuration from `./conf/deploy.ini` which should define:
- `NAME_OF_APPLICATION`: Application name
- `RANGE_START`: Starting port range (default: 6000)
- `RANGE_RESERVED`: Number of ports reserved per user (default: 100)
- `APPLICATION_IDENTITY_NUMBER`: Application identifier for port calculation
- `RANGE_PORTS_PER_APPLICATION`: Ports per application (default: 4)

Port calculation formula:
```
PORT_RANGE_BEGIN = RANGE_START + USER_ID * RANGE_RESERVED
HTTP_PORT = PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION
HTTPS_PORT = HTTP_PORT + 1
HTTP_PORT2 = HTTPS_PORT + 1
HTTPS_PORT2 = HTTP_PORT2 + 1
```

## Operation 1: Check Prerequisites

**Purpose**: Verify that Docker and Docker Compose are installed and accessible.

**Steps**:
1. Check if Docker is installed: `command -v docker`
2. Check if Docker Compose is installed: `command -v docker-compose`
3. Exit with error if either is missing

**Command**: This is part of the `start` operation.

## Operation 2: Generate Secrets and Configuration

**Purpose**: Create secure environment file and nginx configuration.

**Steps for Secrets**:
1. Check if `.env.prod` file exists
2. If not exists:
   - Generate secure DB password: `openssl rand -base64 32 | tr -d "=+/" | cut -c1-25`
   - Generate JWT secret: `openssl rand -base64 32 | tr -d "=+/" | cut -c1-32`
   - Create `.env.prod` with DATABASE_URL, JWT_SECRET, DOMAIN, API_URL, SSL_EMAIL, REACT_APP_API_URL
   - Set file permissions to 600

**Steps for Nginx Config**:
1. Check if `conf/nginx.conf.template` exists
2. Replace `${USER_ID}` placeholder with actual USER_ID value
3. Save as `conf/nginx.conf`

**Command**: This is part of the `start` operation.

## Operation 3: Setup SSL Certificates

**Purpose**: Configure SSL certificates for HTTPS access.

**Steps**:
1. Create `ssl/` directory if it doesn't exist
2. Remove any existing directories that should be files (ssl/fullchain.pem, ssl/privkey.pem)
3. Check for existing certificates in multiple locations:
   - If `~/.ssh/fullchain_domain.crt` AND `~/.ssh/privateKey_domain.key` exist:
     - Copy to `ssl/fullchain.pem` and `ssl/privkey.pem`
   - Else if `ssl/fullchain_domain.crt` AND `ssl/privateKey_domain.key` exist:
     - Copy to `ssl/fullchain.pem` and `ssl/privkey.pem`
4. Else if certbot is installed:
   - Stop nginx if running: `sudo systemctl stop nginx`
   - Obtain certificate: `sudo certbot certonly --standalone -d $DOMAIN --email $EMAIL --agree-tos --non-interactive --quiet`
   - Copy certificates from `/etc/letsencrypt/live/$DOMAIN/` to `ssl/`
5. Else create self-signed certificate:
   - `openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout ssl/privkey.pem -out ssl/fullchain.pem -subj "/C=US/ST=State/L=City/O=Organization/CN=$DOMAIN"`
6. Verify certificates are files (not directories) and set proper permissions

**Command**: This is part of the `start` operation.

## Operation 4: Deploy Services

**Purpose**: Build and start Docker containers with proper configuration.

**Steps**:
1. Stop existing services:
   ```bash
   HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID \
   docker-compose -p "-$USER_ID-$HTTPS_PORT" -f docker-compose.yml down
   ```

2. Build Docker images:
   ```bash
   HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID \
   docker-compose -p "-$USER_ID-$HTTPS_PORT" -f docker-compose.yml build --no-cache --build-arg PIP_UPGRADE=1
   ```

3. Start services:
   ```bash
   HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID \
   docker-compose -p "-$USER_ID-$HTTPS_PORT" -f docker-compose.yml --env-file .env.prod up -d
   ```

4. Wait 3 seconds for services to initialize

5. Check if services are running by container name pattern:
   ```bash
   docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*"
   ```

**Command**: `./deployApp.sh start [USER_ID] [USER_NAME] [USER_EMAIL] [DESCRIPTION]`

## Operation 5: Verify Deployment

**Purpose**: Confirm services are running and accessible.

**Steps**:
1. Check if services are running by container name pattern:
   ```bash
   docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*"
   ```

2. Wait 10 seconds

3. Test API health endpoint:
   ```bash
   curl -f -s "https://www.softfluid.fr:${HTTPS_PORT}/"
   ```

4. Return success if services are running (health check is optional)

**Command**: This is part of the `start` operation, or check status with `./deployApp.sh ps [USER_ID]`

## Operation 6: Verify Application Compliance

**Purpose**: Verify if an application is compliant with the ai-swautomorph platform architecture requirements.

**Overview**: The VERIFY_APP_COMPLIANCE feature analyzes an application's structure and configuration against the ai-swautomorph platform reference architecture located at `~/ai-swautomorph`. It generates a detailed compliance report identifying what components are missing, non-compliant, or correctly configured.

**How It Works**:
1. Analyzes the ai-swautomorph platform requirements (docker-compose.yml, deployApp.sh, configurations)
2. Inspects the target application structure at APPLICATION_FOLDER
3. Compares each component against platform requirements
4. Generates a comprehensive compliance report with:
   - Compliant components ✅
   - Missing components ❌
   - Non-compliant components ⚠️
   - Configuration issues 🔧
   - Deployment script compliance 📜
5. Calculates an overall compliance score
6. Provides actionable recommendations with priority levels

**Context Template**: Uses `VERIFY_APP_COMPLIANCE_context.md` which verifies:
- docker-compose.yml structure (ports, volumes, networks, naming)
- deployApp.sh operations (start, stop, restart, ps, logs)
- Configuration files (conf/deploy.ini, nginx.conf.template)
- Environment variable handling
- SSL certificate management
- Port calculation formulas
- Container and project naming patterns
- Directory structure (ssl/, conf/, scripts/)

**Output**: Detailed compliance report including:
- Overall compliance score (percentage)
- Status (COMPLIANT/PARTIALLY COMPLIANT/NON-COMPLIANT)
- Count of critical, high, medium, and low priority issues
- Specific recommendations for each issue
- References to swautomorph examples

**Use Case**: Run before attempting to deploy an application through the swautomorph platform to identify what needs to be fixed.

## Operation 7: Make Application Compliant

**Purpose**: Automatically modify an application to make it fully compliant with the ai-swautomorph platform architecture.

**Overview**: The MAKE_APP_COMPLIANT feature performs automated remediation to bring an application into compliance with swautomorph requirements. It uses the reference architecture at `~/ai-swautomorph` to create or update all necessary deployment infrastructure files.

**How It Works**:
1. Runs compliance verification to document current state
2. Creates a git branch for compliance changes
3. Copies or creates required files from swautomorph reference:
   - docker-compose.yml with proper port and USER_ID handling
   - deployApp.sh with all required operations
   - conf/deploy.ini with application configuration
   - conf/nginx.conf.template for reverse proxy
   - SSL directory structure
   - Environment configuration templates
   - Backup scripts
4. Adapts configurations for the specific application
5. Sets proper file permissions
6. Tests configuration loading
7. Commits changes with detailed message
8. Generates compliance report showing all changes

**Context Template**: Uses `MAKE_APP_COMPLIANT_context.md` which:
- Preserves application-specific code and business logic
- Only modifies deployment infrastructure files
- Follows swautomorph naming conventions exactly
- Ensures proper port calculation: `PORT_RANGE_BEGIN = RANGE_START + USER_ID * RANGE_RESERVED`
- Configures container naming: `${NAME_OF_APPLICATION}-.*-${USER_ID}-.*`
- Sets up all required operations: start, stop, restart, ps, logs
- Handles SSL certificates and nginx configuration
- Creates backup functionality

**Critical Requirements**:
- Does NOT modify application business logic
- Does NOT change database schemas or data
- Does NOT alter application-specific environment variables
- Does NOT remove existing functionality
- ONLY adds or updates deployment infrastructure

**Output**: Compliance remediation report including:
- Branch name and commit hash
- List of files created/modified
- Compliance status for each component
- Next steps for testing deployment
- Specific deployment commands to verify

**Deployment Test Commands**:
```bash
cd APPLICATION_FOLDER
./deployApp.sh start 0 testuser test@example.com "Test deployment"
./deployApp.sh ps 0
./deployApp.sh logs 0
./deployApp.sh stop 0
```

**Use Case**: After running VERIFY_APP_COMPLIANCE and identifying issues, use this operation to automatically fix all compliance problems and prepare the application for swautomorph deployment.

## Operation 8: Specify AI Context for Modifications

**Purpose**: Transform brief user requests into detailed, actionable specifications for application modifications.

**Overview**: The SPECIFY feature acts as an AI assistant that helps users who want to modify an application but aren't sure how to write comprehensive specifications. It analyzes brief descriptions and generates detailed technical specifications that can be used with the MODIFY action.

**How It Works**:
1. User provides a brief description (1-2 sentences) of desired changes
2. AI analyzes the request using the `SPECIFY_context.md` template
3. AI generates a comprehensive specification including:
   - Clear objectives and scope
   - Technical requirements and implementation details
   - Acceptance criteria
   - Constraints and considerations
   - Code examples when helpful
4. User copies the generated specification and uses it with the MODIFY action

**Context Template**: The feature uses `SPECIFY_context.md` which contains:
- Instructions for analyzing user requests
- Guidelines for generating detailed specifications
- Output format requirements
- Application context variables (APPLICATION_NAME, APPLICATION_FOLDER, REPO_GITHUB_URL)

**Benefits**:
- Saves time by eliminating manual specification writing
- Reduces errors by ensuring all important aspects are covered
- Produces better results through detailed, structured requirements
- Serves as a learning tool for understanding technical specifications

**Example Use Cases**:
- "Add a contact form to the homepage"
- "Change the login button color to blue"
- "Fix the bug where users can't upload images"

**Output Format**: Specifications include a summary box with:
- Type (Feature/Bug Fix/Enhancement/Refactoring)
- Complexity level (Low/Medium/High)
- Estimated impact (files and components affected)
- Risk level (Low/Medium/High)

## Additional Operations

### Check Status
**Command**: `./deployApp.sh ps [USER_ID]`

Returns JSON with:
- Environment variables (USER_ID, USER_NAME, USER_EMAIL, HTTP_PORT, HTTPS_PORT, HTTP_PORT2, HTTPS_PORT2)
- Docker compose status (IS_RUNNING or IS_NOT_RUNNING)
- Active ports (extracted from running containers)
- Git remote URLs

**Implementation**:
- Uses `docker container ls` with name pattern filtering
- Extracts ports from running containers matching `${NAME_OF_APPLICATION}-.*-${USER_ID}-.*`
- Returns all information in structured JSON format

### Stop Services
**Command**: `./deployApp.sh stop [USER_ID]`

Stops all running containers for the specified user by:
- Finding containers matching name pattern `${NAME_OF_APPLICATION}-.*-${USER_ID}-.*`
- Stopping and removing containers using `docker stop` and `docker rm`

### Restart Services
**Command**: `./deployApp.sh restart [USER_ID]`

Restarts all containers without rebuilding by:
- Finding containers matching name pattern `${NAME_OF_APPLICATION}-.*-${USER_ID}-.*`
- Using `docker restart` command on matching containers

### View Logs
**Command**: `./deployApp.sh logs [USER_ID]`

Shows logs from all containers by:
- Finding containers matching name pattern `${NAME_OF_APPLICATION}-.*-${USER_ID}-.*`
- Displaying logs for each container separately with container name headers
- Shows last 50 lines per container using `docker logs --tail=50`

## Example Usage

```bash
# Deploy application for user ID 1
./deployApp.sh start 1 john john@example.com "John's Instance"

# Check status
./deployApp.sh ps 1

# View logs
./deployApp.sh logs 1

# Stop services
./deployApp.sh stop 1
```

## Firewall Configuration (Optional)

If UFW is available, the deployment configures:
- Allow HTTP_PORT/tcp
- Allow HTTPS_PORT/tcp
- Allow HTTP_PORT2/tcp (if exists)
- Allow HTTPS_PORT2/tcp (if exists)

Note: The script no longer resets firewall rules or changes default policies to avoid disrupting existing configurations.

## Backup

A backup script is created at `./scripts/backup.sh` that:
- Backs up the SQLite database using docker-compose exec
- Copies database from container to host
- Keeps last 7 backups
- Can be run manually or via cron
- Uses the calculated project name pattern for container identification