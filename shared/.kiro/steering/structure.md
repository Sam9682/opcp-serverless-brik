---
inclusion: auto
---

# Project Structure

## Directory Layout

```
.
├── .git/                           # Git repository
├── .kiro/                          # Kiro AI assistant configuration
│   ├── hooks/                      # Agent hooks for automation
│   └── steering/                   # AI steering documents
├── conf/                           # Configuration files
│   ├── deploy.ini                  # Deployment configuration
│   └── nginx.conf.template         # Nginx template with ${USER_ID} placeholder
├── shared/                         # Git submodule (ai-swautomorph-shared)
│   └── deployApp.sh                # Standard deployment script
├── scripts/                        # Utility scripts
│   └── backup.sh                   # Database backup script
├── ssl/                            # SSL certificates (not committed)
│   ├── fullchain.pem               # SSL certificate chain
│   └── privkey.pem                 # Private key
├── deployApp.sh                    # Symbolic link to ./shared/deployApp.sh
├── docker-compose.yml              # Container orchestration
├── .env.prod                       # Production environment (generated, not committed)
├── .gitignore                      # Git ignore rules
└── *_context.md                    # AI operation context templates
```

## Key Files

### Deployment Infrastructure

- `deployApp.sh`: Main deployment script (symlink to shared submodule)
- `docker-compose.yml`: Defines services, ports, volumes, networks
- `conf/deploy.ini`: Application-specific configuration
- `conf/nginx.conf.template`: Nginx reverse proxy template

### AI Context Templates

Context files define autonomous AI operations:

- `START_context.md`: Deploy and start application
- `STOP_context.md`: Stop running services
- `RESTART_context.md`: Restart services
- `PS_context.md`: Check application status
- `LOGS_context.md`: View container logs
- `VERIFY_APP_COMPLIANCE_context.md`: Check platform compliance
- `MAKE_APP_COMPLIANT_context.md`: Auto-fix compliance issues
- `SPECIFY_context.md`: Generate detailed specifications from brief requests
- `MODIFY_CODE_context.md`: Modify application code
- `BACKUP_DATABASE_context.md`: Backup database
- `RESTORE_DATABASE_context.md`: Restore database
- `CLEAN_DATABASE_context.md`: Clean database

### Documentation

- `README.md`: Comprehensive deployment guide
- `README_SPECIFY.md`: SPECIFY feature documentation

## File Naming Conventions

### Context Templates
- Format: `{OPERATION}_context.md`
- Uppercase operation names
- Contains AI instructions for autonomous execution
- Uses template variables: `{{APPLICATION_FOLDER}}`, `{{USER_ID}}`, `{{MESSAGE}}`

### Configuration Files
- `deploy.ini`: INI format with KEY=VALUE pairs
- `nginx.conf.template`: Template with `${USER_ID}` placeholder
- `.env.prod`: Environment variables (generated at deployment)

### Container Names
- Pattern: `${NAME_OF_APPLICATION}-{service}-${USER_ID}-${PORT}`
- Example: `myapp-frontend-0-6001`
- Enables filtering by user: `docker ps --filter "name=myapp-.*-0-.*"`

## Git Submodule Structure

The `shared/` directory is a git submodule pointing to:
- Repository: `https://github.com/Sam9682/ai-swautomorph-shared.git`
- Contains: Standard deployment scripts shared across applications
- Update: `git submodule update --remote shared`

## Ignored Files (.gitignore)

```
.env.prod
ssl/privkey.pem
ssl/fullchain.pem
conf/nginx.conf
```

These files contain secrets or are generated at deployment time.

## Directory Creation

Required directories are created automatically by deployment scripts:
- `ssl/`: Created during SSL setup
- `conf/`: Created during configuration generation
- `scripts/`: Created when backup script is generated

## File Permissions

- Scripts: `chmod +x` (executable)
- `.env.prod`: `chmod 600` (owner read/write only)
- SSL certificates: `chmod 644` (fullchain), `chmod 600` (privkey)

## Configuration Loading Order

1. `conf/deploy.ini` is sourced first
2. Environment variables are calculated (ports)
3. `.env.prod` is loaded by docker-compose
4. `nginx.conf` is generated from template
5. Docker containers start with combined configuration
