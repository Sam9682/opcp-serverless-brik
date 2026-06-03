---
inclusion: auto
---

# Product Overview

This is the ai-swautomorph platform deployment system - a multi-tenant application deployment framework that enables automated deployment, management, and compliance verification of containerized applications on Linux servers.

## Core Capabilities

- Multi-user deployment with isolated port ranges per user
- Automated SSL certificate management (Let's Encrypt or self-signed)
- Docker-based containerization with nginx reverse proxy
- Compliance verification and automated remediation for platform standards
- AI-assisted specification generation for application modifications
- Database backup and restore functionality
- Comprehensive deployment operations (start, stop, restart, status, logs)

## Key Features

- Port calculation based on USER_ID to prevent conflicts
- Standardized deployment script (deployApp.sh) with consistent operations
- Configuration-driven deployment via deploy.ini
- SSL/TLS support with automatic certificate handling
- Firewall configuration (UFW) integration
- Git submodule-based shared deployment scripts
- AI context templates for autonomous operations

## Target Users

System administrators and DevOps engineers deploying and managing multiple application instances on shared infrastructure with user isolation requirements.
