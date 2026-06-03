---
inclusion: auto
---

# Technology Stack

## Core Technologies

- **Shell Scripting**: Bash scripts for deployment automation
- **Docker**: Container runtime for application isolation
- **Docker Compose**: Multi-container orchestration
- **Nginx**: Reverse proxy and SSL termination
- **OpenSSL**: Certificate generation and cryptographic operations
- **Git**: Version control with submodule support for shared scripts

## System Requirements

- Linux operating system (tested on Ubuntu/Debian)
- Docker (check: `docker --version`)
- Docker Compose (check: `docker-compose --version`)
- OpenSSL for secret generation
- curl for health checks
- jq for JSON processing
- UFW (optional) for firewall management
- Certbot (optional) for Let's Encrypt certificates

## Configuration Files

- `conf/deploy.ini`: Application configuration (ports, naming, ranges)
- `conf/nginx.conf.template`: Nginx configuration template with USER_ID placeholder
- `.env.prod`: Production environment variables (generated, not committed)
- `docker-compose.yml`: Service definitions with port and USER_ID variables
- `.gitignore`: Excludes sensitive files (.env.prod, SSL certificates)

## Port Calculation Formula

```bash
PORT_RANGE_BEGIN = RANGE_START + USER_ID * RANGE_RESERVED
HTTP_PORT = PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION
HTTPS_PORT = HTTP_PORT + 1
HTTP_PORT2 = HTTPS_PORT + 1
HTTPS_PORT2 = HTTP_PORT2 + 1
```

Default values:
- RANGE_START: 6000
- RANGE_RESERVED: 100 ports per user
- RANGE_PORTS_PER_APPLICATION: 4 ports per app

## Common Commands

### Deployment Operations

```bash
# Start application for user
./deployApp.sh start [USER_ID] [USER_NAME] [USER_EMAIL] [DESCRIPTION]

# Check status (returns JSON)
./deployApp.sh ps [USER_ID]

# View logs
./deployApp.sh logs [USER_ID]

# Stop services
./deployApp.sh stop [USER_ID]

# Restart services
./deployApp.sh restart [USER_ID]
```

### Docker Operations

```bash
# List containers by name pattern
docker ps --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*"

# View compose services
docker-compose -p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}" ps

# Build without cache
docker-compose build --no-cache --build-arg PIP_UPGRADE=1
```

### Configuration Testing

```bash
# Load and verify configuration
source ./conf/deploy.ini
echo "App: $NAME_OF_APPLICATION, Ports: $RANGE_START"

# Test nginx config generation
sed "s/\${USER_ID}/$USER_ID/g" conf/nginx.conf.template > conf/nginx.conf
```

### SSL Certificate Management

```bash
# Check existing certificates
ls -la ssl/fullchain.pem ssl/privkey.pem

# Generate self-signed certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout ssl/privkey.pem -out ssl/fullchain.pem \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=$DOMAIN"
```

### Git Submodule Operations

```bash
# Add shared scripts submodule
git submodule add https://github.com/Sam9682/ai-swautomorph-shared.git shared

# Update submodule to latest
git submodule update --remote shared

# Initialize submodules after clone
git submodule update --init --recursive
```

## Container Naming Convention

Containers must follow the pattern: `${NAME_OF_APPLICATION}-*-${USER_ID}-*`

This enables filtering and management by user ID.

## Script Execution Context

- All deployment commands execute from the application root directory
- Scripts should NOT be run as root user
- deployApp.sh is typically a symbolic link to `./shared/deployApp.sh`
- Configuration is loaded from `./conf/deploy.ini` at script start
