#!/bin/bash
# ${NAME_OF_APPLICATION} Production Deployment Script

set -e

# Load configuration from deploy.ini
load_config() {
    local config_file="./conf/deploy.ini"
    if [ -f "$config_file" ]; then
        echo "📋 Loading configuration from $config_file"
        # Source the config file, ignoring comments and empty lines
        while IFS='=' read -r key value; do
            # Skip comments and empty lines
            [[ $key =~ ^[[:space:]]*# ]] && continue
            [[ -z $key ]] && continue
            # Remove leading/trailing whitespace and export
            key=$(echo "$key" | xargs)
            value=$(echo "$value" | xargs)
            if [[ -n $key && -n $value ]]; then
                export "$key"="$value"
            fi
        done < "$config_file"
        echo "  ✅ Configuration loaded successfully"
    else
        echo "  ⚠️ Configuration file $config_file not found, using defaults"
    fi
}

# Load configuration first
load_config

# Global Parameters
COMMAND=${1:-help}
USER_ID=${2:-0}
USER_NAME=${3:-"admin"}
USER_EMAIL=${4:-"admin@softfluid.fr"}
DESCRIPTION=${5:-"Basic Admin User"}

RANGE_START=6000
RANGE_RESERVED=100
RANGE_PORTS_PER_APPLICATION=4

# Configuration
DOMAIN=${DOMAIN:-"softfluid.fr"}
EMAIL=${EMAIL:-"admin@softfluid.fr"}
ENV_FILE=".env.prod"

# Calculate ports (convert alphanumeric USER_ID to numeric for port calculation)
calculate_ports() {
    # Ensure USER_ID is numeric, default to 0 if not
    if ! [[ "$USER_ID" =~ ^[0-9]+$ ]]; then
        USER_ID=0
    fi
    
    PORT_RANGE_BEGIN=$((RANGE_START + USER_ID * RANGE_RESERVED))
    HTTP_PORT=$((PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION))
    HTTPS_PORT=$((HTTP_PORT + 1))
    HTTP_PORT2=$((HTTPS_PORT + 1))
    HTTPS_PORT2=$((HTTP_PORT2 + 1))
}

# Display environment variables for operations
show_environment() {
    local operation=$1
    echo "🔍 Starting $operation operation..."
    echo "Environment Variables:"
    echo "  USER_ID=${USER_ID}"
    echo "  USER_NAME=${USER_NAME}"
    echo "  USER_EMAIL=${USER_EMAIL}"
    echo "  HTTP_PORT=${HTTP_PORT}"
    echo "  HTTP_PORT2=${HTTP_PORT2}"
    echo "  HTTPS_PORT=${HTTPS_PORT}"
    echo "  HTTPS_PORT2=${HTTPS_PORT2}"
    echo ""
}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   log_error "This script should not be run as root"
   exit 1
fi

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    log_info "Prerequisites check passed ✅"
}

# Generate secure passwords
generate_secrets() {
    log_info "Generating secure secrets..."
    
    if [[ ! -f "$ENV_FILE" ]]; then
        log_info "Creating production environment file..."
        
        # Generate secure passwords
        DB_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-25)
        JWT_SECRET=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
        
        cat > "$ENV_FILE" << EOF
# Database Configuration (SQLite)
DATABASE_URL=sqlite:///./data/ai_haccp.db

# Security
JWT_SECRET=$JWT_SECRET

# Domain Configuration
DOMAIN=$DOMAIN
API_URL=https://$DOMAIN
SSL_EMAIL=$EMAIL

# Frontend
REACT_APP_API_URL=https://$DOMAIN
EOF
        
        chmod 600 "$ENV_FILE"
        log_info "Environment file created with secure passwords ✅"
    else
        log_warn "Environment file already exists, skipping generation"
    fi
}

# Generate nginx configuration from template
generate_nginx_config() {
    if [[ -f "conf/nginx.conf.template" ]]; then
        log_info "Generating nginx configuration..."
        # Replace ${USER_ID} with actual USER_ID value
        sed "s/\${USER_ID}/$USER_ID/g" conf/nginx.conf.template > conf/nginx.conf
        log_info "nginx.conf generated from template ✅"
    else
        log_warn "nginx.conf.template not found, skipping nginx configuration"
    fi
}

# Setup SSL certificates
setup_ssl() {
    log_info "Setting up SSL certificates..."
    
    if [[ ! -d "ssl" ]]; then
        mkdir -p ssl
    fi

    if [[ -f "ssl/fullchain.pem" && -f "ssl/privkey.pem" ]]; then
        log_info "SSL certificates already exist in ./ssl fodler ✅"

    # Check for existing certificates in ~/.ssh/ or current ssl/ directory
    elif [[ -f "$HOME/.ssh/fullchain_domain.crt" && -f "$HOME/.ssh/privateKey_domain.key" ]]; then
        log_info "Using existing certificates from ~/.ssh/..."
        
        # Remove any existing directories with same names
        rm -rf ssl/fullchain.pem ssl/privkey.pem
        
        # Copy existing certificates
        cp "$HOME/.ssh/fullchain_domain.crt" ssl/fullchain.pem
        cp "$HOME/.ssh/privateKey_domain.key" ssl/privkey.pem
        
        # Set proper permissions
        chmod 644 ssl/fullchain.pem
        chmod 600 ssl/privkey.pem
        
        log_info "Existing certificates copied ✅"
    # Check for certificates in current ssl directory
    elif [[ -f "ssl/fullchain_domain.crt" && -f "ssl/privateKey_domain.key" ]]; then
        log_info "Using existing certificates from ssl/ directory..."
        
        # Remove any existing directories with same names
        rm -rf ssl/fullchain.pem ssl/privkey.pem
        
        # Copy existing certificates
        cp ssl/fullchain_domain.crt ssl/fullchain.pem
        cp ssl/privateKey_domain.key ssl/privkey.pem
        
        # Set proper permissions
        chmod 644 ssl/fullchain.pem
        chmod 600 ssl/privkey.pem
        
        log_info "Local certificates copied ✅"
    # Check if certbot is installed
    elif command -v certbot &> /dev/null; then
        log_info "Obtaining SSL certificate for $DOMAIN..."
        
        # Stop nginx if running
        sudo systemctl stop nginx 2>/dev/null || true
        
        # Get certificate
        sudo certbot certonly --standalone \
            -d "$DOMAIN" \
            --email "$EMAIL" \
            --agree-tos \
            --non-interactive \
            --quiet
        
        # Copy certificates from letsencrypt
        sudo cp "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ssl/
        sudo cp "/etc/letsencrypt/live/$DOMAIN/privkey.pem" ssl/
        sudo chown -R $USER:$USER ssl/
        
        log_info "SSL certificates obtained ✅"
    else
        log_warn "Certbot not found. Creating self-signed certificates for testing..."
        
        # Create self-signed certificate for testing
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout ssl/privkey.pem \
            -out ssl/fullchain.pem \
            -subj "/C=US/ST=State/L=City/O=Organization/CN=$DOMAIN"
        
        log_warn "Self-signed certificate created. Replace with real certificate for production!"
    fi
    
    # Final verification that certificates are files
    if [[ -f "ssl/fullchain.pem" && -f "ssl/privkey.pem" ]]; then
        log_info "SSL certificates verified as files ✅"
        ls -la ssl/fullchain.pem ssl/privkey.pem
    else
        log_error "SSL certificates are not properly configured as files!"
        if [[ -d "ssl/fullchain.pem" ]]; then
            log_error "ssl/fullchain.pem is a directory, not a file"
        fi
        if [[ -d "ssl/privkey.pem" ]]; then
            log_error "ssl/privkey.pem is a directory, not a file"
        fi
        return 1
    fi
}

# Build and deploy
deploy_services() {
    log_info "Building and deploying services..."

    # Stop existing services
    HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker compose -p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}" -f docker-compose.yml down 2>/dev/null || true
    
    # Build images
    log_info "Building Docker images..."
    HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker compose -p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}" -f docker-compose.yml build --no-cache --build-arg PIP_UPGRADE=1
    
    # Start services
    log_info "Starting production services..."
    HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker compose -p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}" -f docker-compose.yml --env-file "$ENV_FILE" up -d
    
    # Wait for services to be ready
    log_info "Waiting for services to start..."
    sleep 3

    # Check service health
    containers=$(docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*")
    if [[ -z "$containers" ]]; then
        log_error "Some services failed to start"
        docker compose -p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}" -f docker-compose.yml logs
    else
        log_info "Services deployed successfully ✅"
    fi
}

# Verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    # Check if services are running by name pattern
    containers=$(docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*")
    if [[ -z "$containers" ]]; then
        log_error "Services are not running properly"
        return 1
    fi
    
    # Test API health endpoint
    sleep 10
    if curl -f -s "https://${DOMAIN}:${HTTPS_PORT}/" > /dev/null; then
        log_info "API health check passed ✅"
    else
        log_warn "API health check failed, but services are running"
    fi
    
    log_info "Deployment verification completed"
}

# Setup firewall
setup_firewall() {
    log_info "Configuring firewall..."
    
    if command -v ufw &> /dev/null; then
        # Configure UFW if available
        sudo ufw allow ${HTTPS_PORT}/tcp
        # if  ${HTTP_PORT} exist then allow it
        if [[ -n "${HTTP_PORT}" ]]; then
            sudo ufw allow ${HTTP_PORT}/tcp
        fi
        # if  ${HTTPS_PORT2} exist then allow it
        if [[ -n "${HTTPS_PORT2}" ]]; then
            sudo ufw allow ${HTTPS_PORT2}/tcp
        fi
        # if  ${HTTP_PORT2} exist then allow it
        if [[ -n "${HTTP_PORT2}" ]]; then
            sudo ufw allow ${HTTP_PORT2}/tcp
        fi
        
        log_info "Firewall configured ✅"
    else
        log_warn "UFW not found, skipping firewall configuration"
    fi
}

# Create backup script
create_backup_script() {
    log_info "Creating backup script..."
    
    mkdir -p ./scripts
    
    cat > ./scripts/backup.sh << 'EOF'
#!/bin/bash
# ${NAME_OF_APPLICATION} Backup Script

BACKUP_DIR="backups"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/ai_haccp_backup_$DATE"

mkdir -p "$BACKUP_DIR"

echo "Creating backup: $BACKUP_FILE"
HTTP_PORT=$HTTP_PORT HTTPS_PORT=$HTTPS_PORT HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 USER_ID=$USER_ID docker compose -p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}" -f docker-compose.yml exec -T api cp /app/data/ai_haccp.db /tmp/backup.db
docker cp $(docker compose -p "-$USER_ID-$HTTPS_PORT" -f docker-compose.yml ps -q api):/tmp/backup.db "$BACKUP_FILE.db"

if [[ $? -eq 0 ]]; then
    echo "Backup created successfully: $BACKUP_FILE"
    
    # Keep only last 7 backups
    ls -t "$BACKUP_DIR"/ai_haccp_backup_*.db | tail -n +8 | xargs -r rm
    echo "Old backups cleaned up"
else
    echo "Backup failed!"
    exit 1
fi
EOF
    
    chmod +x ./scripts/backup.sh
    log_info "Backup script created ✅"
}

# Main deployment process
start_services() {
    log_info "Starting ${NAME_OF_APPLICATION} production deployment..."
    show_environment
    check_prerequisites
    generate_secrets
    generate_nginx_config
    setup_ssl
    deploy_services
    verify_deployment
    setup_firewall
    create_backup_script
    
    log_info "Services started successfully ✅"
}

# Stop services
stop_services() {
    log_info "Stopping ${NAME_OF_APPLICATION} services..."
    # Stop containers by name pattern
    containers=$(docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*")
    if [[ -n "$containers" ]]; then
        docker stop $containers
        docker rm $containers
        log_info "Services stopped successfully ✅"
    else
        log_warn "No running containers found for USER_ID: $USER_ID"
    fi
}

# Restart services
restart_services() {
    log_info "Restarting ${NAME_OF_APPLICATION} services..."
    # Restart containers by name pattern
    containers=$(docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*")
    if [[ -n "$containers" ]]; then
        docker restart $containers
        log_info "Services restarted successfully ✅"
    else
        log_warn "No running containers found for USER_ID: $USER_ID"
    fi
}

# Show logs
show_logs() {
    log_info "Showing ${NAME_OF_APPLICATION} service logs..."
    # Show logs for containers by name pattern
    containers=$(docker ps -q --filter "name=${NAME_OF_APPLICATION}-.*-${USER_ID}-.*")
    if [[ -n "$containers" ]]; then
        # Show logs for each container separately
        for container in $containers; do
            container_name=$(docker inspect --format='{{.Name}}' $container | cut -c2-)
            echo "=== Logs for container: $container_name ==="
            docker logs --tail=50 $container
            echo ""
        done
    else
        log_warn "No running containers found for USER_ID: $USER_ID"
    fi
}

# Show usage help
show_usage() {
    echo "Usage: $0 [COMMAND] [USER_ID] [USER_NAME] [USER_EMAIL] [DESCRIPTION]"
    echo ""
    echo "Commands:"
    echo "  start, -s, --start     Start the application"
    echo "  stop, -k, --stop       Stop the application"
    echo "  restart, -r, --restart Restart the application"
    echo "  ps, -p, --ps           Show application status"
    echo "  logs, -l, --logs       Show application logs"
    echo "  help, -h, --help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 start 1 john john@example.com \"My App\""
    echo "  $0 stop"
    echo "  $0 ps"
}

# Check service status
check_status() {
    # Get actual ports
    actual_http_port="$HTTP_PORT"
    actual_https_port="$HTTPS_PORT"
    actual_http_port2="$HTTP_PORT2"
    actual_https_port2="$HTTPS_PORT2"

    # Check docker status using docker container ls
    docker_status="IS_NOT_RUNNING"
    docker_ports="[]"
    
    if docker container ls --filter "status=running" --format "{{.Names}}" | grep "^${NAME_OF_APPLICATION}-.*-${USER_ID}-.*$"; then
        docker_status="IS_RUNNING"
        # Extract all ports from running containers matching the pattern
        all_ports=$(docker container ls --filter "status=running" --format "{{.Names}} {{.Ports}}" | grep "^${NAME_OF_APPLICATION}-.*-${USER_ID}-.*$" | grep -o '0.0.0.0:[0-9]*' | cut -d: -f2 | sort -n | uniq)
        if [[ -n "$all_ports" ]]; then
            docker_ports=$(echo "$all_ports" | jq -R . | jq -s .)
        fi
    fi
    
    # Get git remote URLs
    git_remotes=$(git remote -v 2>/dev/null | awk '{print $2}' | sort -u | jq -R . | jq -s . 2>/dev/null || echo '[]')
    
    # Output JSON
    jq -n --arg user_id "$USER_ID" \
          --arg user_name "$USER_NAME" \
          --arg user_email "$USER_EMAIL" \
          --arg http_port "$actual_http_port" \
          --arg https_port "$actual_https_port" \
          --arg http_port2 "$actual_http_port2" \
          --arg https_port2 "$actual_https_port2" \
          --arg docker_status "$docker_status" \
          --argjson docker_ports "$docker_ports" \
          --argjson git_remotes "$git_remotes" \
          '{
            "environment_vars": {
              "USER_ID": $user_id,
              "USER_NAME": $user_name,
              "USER_EMAIL": $user_email,
              "HTTP_PORT": $http_port,
              "HTTPS_PORT": $https_port,
              "HTTP_PORT2": $http_port2,
              "HTTPS_PORT2": $https_port2
            },
            "docker_compose_ps": $docker_status,
            "docker_ports": $docker_ports,
            "git_remote": $git_remotes
          }'
}

# Main function - orchestrates the deployment process
main() {
    calculate_ports

    case $COMMAND in
        "ps"|"--ps"|"-p")
            check_status
            exit 0
            ;;
        "stop"|"--stop"|"-k")
            stop_services
            exit 0
            ;;
        "logs"|"--logs"|"-l")
            show_logs
            exit 0
            ;;
        "restart"|"--restart"|"-r")
            restart_services
            exit 0
            ;;
        "start"|"--start"|"-s")
            echo "🚀 ${NAME_OF_APPLICATION} Production Deployment"
            echo "=================================="

            start_services
            exit 0
            ;;
        "help"|"--help"|"-h")
            show_usage
            exit 0
            ;;
        *)
            show_usage
            exit 1
            ;;
    esac
}

# Execute main function
main "$@"
