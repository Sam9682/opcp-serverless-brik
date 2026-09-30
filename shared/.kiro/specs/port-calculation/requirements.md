# Requirements Document

## Introduction

The deployment tooling currently calculates and propagates four port values per application instance (HTTP_PORT, HTTPS_PORT, HTTP_PORT2, HTTPS_PORT2), reserving four ports per application. This feature expands the scheme to ten port values, exposed as five HTTP ports (HTTP_PORT1 through HTTP_PORT5) and five HTTPS ports (HTTPS_PORT1 through HTTPS_PORT5), laid out as consecutive interleaved ports starting from a per-application base. The change spans the primary deployment script (`deployApp.sh`) and every Markdown document consumed by the genAI agent that references the port logic, so that port calculation, environment propagation, logging, firewall configuration, project naming, and health checks all operate consistently on the ten-value scheme.

## Glossary

- **Deployment_Script**: The `deployApp.sh` Bash script that calculates ports and orchestrates docker-compose operations.
- **Agent_Doc**: Any Markdown file consumed by the genAI agent that references port logic, specifically the `*_context.md` files (STOP_context.md, CLEAN_DATABASE_context.md, RESTORE_DATABASE_context.md, PS_context.md, MODIFY_CODE_context.md, VERIFY_APP_COMPLIANCE_context.md, START_context.md, LOGS_context.md, RESTART_context.md, MAKE_APP_COMPLIANT_context.md, BACKUP_DATABASE_context.md, SPECIFY_context.md) and the README files (README.md, README_SPECIFY.md).
- **Port_Base**: The computed base port value, equal to `PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION`.
- **PORT_RANGE_BEGIN**: The per-user starting port, equal to `RANGE_START + USER_ID * RANGE_RESERVED`.
- **RANGE_PORTS_PER_APPLICATION**: The number of ports reserved per application instance.
- **HTTP_PORTn**: The nth HTTP port variable, where n ranges from 1 to 5 (HTTP_PORT1, HTTP_PORT2, HTTP_PORT3, HTTP_PORT4, HTTP_PORT5).
- **HTTPS_PORTn**: The nth HTTPS port variable, where n ranges from 1 to 5 (HTTPS_PORT1, HTTPS_PORT2, HTTPS_PORT3, HTTPS_PORT4, HTTPS_PORT5).
- **Ten_Port_Set**: The complete set of the ten port variables HTTP_PORT1 through HTTP_PORT5 and HTTPS_PORT1 through HTTPS_PORT5.

## Requirements

### Requirement 1

**User Story:** As a deployment operator, I want ten consecutive interleaved ports calculated per application instance, so that each instance can expose five HTTP and five HTTPS services from a single contiguous port block.

#### Acceptance Criteria

1. THE Deployment_Script SHALL set RANGE_PORTS_PER_APPLICATION to 10.
2. THE Deployment_Script SHALL compute Port_Base as `PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION`.
3. THE Deployment_Script SHALL set HTTP_PORT1 equal to Port_Base.
4. THE Deployment_Script SHALL set HTTPS_PORT1 equal to Port_Base plus 1.
5. THE Deployment_Script SHALL set HTTP_PORT2 equal to Port_Base plus 2 and HTTPS_PORT2 equal to Port_Base plus 3.
6. THE Deployment_Script SHALL set HTTP_PORT3 equal to Port_Base plus 4 and HTTPS_PORT3 equal to Port_Base plus 5.
7. THE Deployment_Script SHALL set HTTP_PORT4 equal to Port_Base plus 6 and HTTPS_PORT4 equal to Port_Base plus 7.
8. THE Deployment_Script SHALL set HTTP_PORT5 equal to Port_Base plus 8 and HTTPS_PORT5 equal to Port_Base plus 9.

### Requirement 2

**User Story:** As a maintainer, I want the previously unsuffixed port variables renamed to their suffix-1 equivalents, so that the naming scheme is uniform across all ten values.

#### Acceptance Criteria

1. THE Deployment_Script SHALL use the name HTTP_PORT1 in place of the former unsuffixed HTTP_PORT variable.
2. THE Deployment_Script SHALL use the name HTTPS_PORT1 in place of the former unsuffixed HTTPS_PORT variable.
3. THE Deployment_Script SHALL reference the port value used for the docker-compose project name as HTTPS_PORT1.
4. THE Deployment_Script SHALL reference the port value used for the health-check curl request as HTTPS_PORT1.

### Requirement 3

**User Story:** As a deployment operator, I want all ten port variables propagated to docker-compose invocations, so that every containerized service receives its assigned ports.

#### Acceptance Criteria

1. WHEN the Deployment_Script invokes docker-compose, THE Deployment_Script SHALL pass the complete Ten_Port_Set as environment variables to the docker-compose command.
2. WHEN the Deployment_Script displays its environment summary, THE Deployment_Script SHALL echo each variable of the Ten_Port_Set with its assigned value.

### Requirement 4

**User Story:** As a deployment operator, I want firewall rules opened for all ten ports, so that all five HTTP and five HTTPS services are reachable.

#### Acceptance Criteria

1. WHERE the ufw firewall command is available, THE Deployment_Script SHALL add an allow rule for each variable of the Ten_Port_Set that has a non-empty value.
2. IF a variable of the Ten_Port_Set is empty, THEN THE Deployment_Script SHALL skip adding a firewall rule for that variable.

### Requirement 5

**User Story:** As a genAI agent consumer, I want every Agent_Doc updated to the ten-value scheme, so that documentation-driven operations match the Deployment_Script behavior.

#### Acceptance Criteria

1. WHERE an Agent_Doc defines the port calculation, THE Agent_Doc SHALL express the Ten_Port_Set using the interleaved consecutive layout defined in Requirement 1.
2. WHERE an Agent_Doc sets RANGE_PORTS_PER_APPLICATION, THE Agent_Doc SHALL set RANGE_PORTS_PER_APPLICATION to 10.
3. WHERE an Agent_Doc passes port environment variables to docker-compose, THE Agent_Doc SHALL pass the complete Ten_Port_Set.
4. WHERE an Agent_Doc references the docker-compose project name port, THE Agent_Doc SHALL reference HTTPS_PORT1.
5. WHERE an Agent_Doc lists port variables for informational or configuration purposes, THE Agent_Doc SHALL list the complete Ten_Port_Set.
6. WHERE an Agent_Doc describes firewall rules for ports, THE Agent_Doc SHALL describe allow rules covering the complete Ten_Port_Set.

### Requirement 6

**User Story:** As a maintainer, I want complete coverage of all port references, so that no stale four-value reference remains after the change.

#### Acceptance Criteria

1. THE Deployment_Script SHALL contain no reference to an unsuffixed HTTP_PORT or unsuffixed HTTPS_PORT variable name.
2. THE Deployment_Script SHALL contain no reference to a RANGE_PORTS_PER_APPLICATION value of 4.
3. WHERE an Agent_Doc references port logic, THE Agent_Doc SHALL contain no reference to an unsuffixed HTTP_PORT or unsuffixed HTTPS_PORT variable name and no reference to a RANGE_PORTS_PER_APPLICATION value of 4.
