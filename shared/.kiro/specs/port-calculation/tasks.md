# Implementation Plan: port-calculation

## Overview

Widen the per-application port scheme from four ports to ten (five HTTP + five HTTPS) laid out as a single contiguous interleaved block. Work is mechanical: define one canonical port-calc block and one canonical env-passing prefix, then apply them uniformly across `deployApp.sh` and every Agent_Doc, renaming the former unsuffixed `HTTP_PORT`/`HTTPS_PORT` to `HTTP_PORT1`/`HTTPS_PORT1` and removing every stale four-value reference. Implementation language is Bash (`deployApp.sh`) and Markdown (docs); property tests use a small Bash/Bats-style arithmetic harness.

## Tasks

- [x] 1. Update the port-calculation core in `deployApp.sh`
  - [x] 1.1 Change the reservation constant and rewrite the port-calc block
    - In the globals block, change `RANGE_PORTS_PER_APPLICATION=4` to `RANGE_PORTS_PER_APPLICATION=10`
    - Replace the four-line port computation with the ten-value interleaved layout: `HTTP_PORT1=$((PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION))`, `HTTPS_PORT1=$((HTTP_PORT1 + 1))`, then chained `+ 1` for `HTTP_PORT2` through `HTTPS_PORT5`
    - Preserve the existing chained `+ 1` style so consecutiveness holds by construction
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8_

  - [x]* 1.2 Write property test for the port base formula
    - **Feature: port-calculation, Property 1: Port base formula**
    - Drive randomized non-negative `USER_ID` / `APPLICATION_IDENTITY_NUMBER` (min 100 iterations) and assert `Port_Base == RANGE_START + USER_ID*RANGE_RESERVED + APPLICATION_IDENTITY_NUMBER*10`
    - **Validates: Requirements 1.1, 1.2**

  - [x]* 1.3 Write property test for the ten-port interleaved consecutive layout
    - **Feature: port-calculation, Property 2: Ten-port interleaved consecutive layout**
    - Assert `HTTP_PORTn = Port_Base + 2*(n-1)` and `HTTPS_PORTn = HTTP_PORTn + 1` for n in 1..5, and that the ten values are the distinct consecutive integers `Port_Base..Port_Base+9`
    - **Validates: Requirements 1.3, 1.4, 1.5, 1.6, 1.7, 1.8**

- [x] 2. Update environment summary and docker-compose propagation in `deployApp.sh`
  - [x] 2.1 Rewrite `show_environment` to echo the full Ten_Port_Set
    - Replace the four `echo` lines with ten, each printing the variable name paired with its value (`HTTP_PORT1`..`HTTPS_PORT5`)
    - _Requirements: 3.2_

  - [x] 2.2 Update all docker-compose env-passing prefixes and project name
    - Rewrite the env prefix in `deploy_services` (down / build / up) and the generated `backup.sh` in `create_backup_script` to carry all ten variables plus `USER_ID`
    - Change the `-p` project-name segment from `${HTTPS_PORT}` to `${HTTPS_PORT1}`
    - Repoint the failure-path `docker-compose ... logs` invocation (around line 280) whose `-p` project name still references unsuffixed `HTTPS_PORT` — change it to `HTTPS_PORT1` to match the down/build/up invocations
    - In the generated `backup.sh` heredoc, repoint the SECOND `-p` reference (around line 352): `docker cp $(docker-compose -p "-$USER_ID-$HTTPS_PORT" ...)` — both `HTTPS_PORT` refs in the heredoc must become `HTTPS_PORT1`
    - _Requirements: 2.1, 2.2, 2.3, 3.1_

  - [x]* 2.3 Write property test for environment summary listing
    - **Feature: port-calculation, Property 3: Environment summary lists the full Ten_Port_Set**
    - Assert the summary output contains each of the ten variable names paired with its assigned value
    - **Validates: Requirements 3.2**

  - [x]* 2.4 Write property test for docker-compose propagation
    - **Feature: port-calculation, Property 4: docker-compose invocations propagate the full Ten_Port_Set**
    - Assert every docker-compose invocation's env prefix includes all ten variables bound to their computed values
    - **Validates: Requirements 3.1**

- [x] 3. Update firewall, status, and health-check in `deployApp.sh`
  - [x] 3.1 Rewrite `setup_firewall` to cover all ten ports
    - Under the existing `command -v ufw` guard, iterate the Ten_Port_Set and issue `sudo ufw allow "${port}/tcp"` only for non-empty values
    - _Requirements: 4.1, 4.2_

  - [x] 3.2 Update `check_status` JSON and health-check curl
    - Emit the full Ten_Port_Set under `environment_vars`, using `*1` names in place of the unsuffixed names
    - Repoint the intermediate local reads (around lines 452-455), e.g. `actual_http_port="$HTTP_PORT"` / `actual_https_port="$HTTPS_PORT"`, and their `jq --arg` bindings: the RHS reads `$HTTP_PORT`/`$HTTPS_PORT` must become `$HTTP_PORT1`/`$HTTPS_PORT1`, and new locals and `--arg` bindings must be added for ports 3-5 as well — not just the JSON keys
    - Change the health-check URL from `https://${DOMAIN}:${HTTPS_PORT}/` to `https://${DOMAIN}:${HTTPS_PORT1}/`
    - _Requirements: 2.4, 3.2, 5.5_

  - [x]* 3.3 Write property test for firewall rules matching non-empty ports
    - **Feature: port-calculation, Property 5: Firewall rules match exactly the non-empty ports**
    - With some Ten_Port_Set variables empty and `ufw` available, assert allow rules correspond exactly to non-empty variables and no rule is issued for an empty one
    - **Validates: Requirements 4.1, 4.2**

- [x] 4. Checkpoint - Ensure all `deployApp.sh` tests pass
  - Ensure all property and smoke tests for the script pass, ask the user if questions arise.

- [x] 5. Apply canonical edits to database context Agent_Docs
  - [x] 5.1 Update `STOP_context.md`
    - Apply the ten-value port-calc block, ten-var docker-compose env prefix, `HTTPS_PORT1` project name, and firewall description where each pattern is present
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.6_

  - [x] 5.2 Update `START_context.md`
    - Apply port-calc block, env prefix, project name, health-check URL, and firewall edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.6_

  - [x] 5.3 Update `RESTART_context.md`
    - Apply port-calc block, env prefix, project name, and health-check edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 5.4 Update `CLEAN_DATABASE_context.md`
    - Apply port-calc block, env prefix, and project name edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 5.5 Update `RESTORE_DATABASE_context.md`
    - Apply port-calc block, env prefix, and project name edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 5.6 Update `BACKUP_DATABASE_context.md`
    - Apply port-calc block, env prefix, and project name edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [x] 6. Apply canonical edits to remaining context Agent_Docs
  - [x] 6.1 Update `PS_context.md`
    - Apply port-calc block, env prefix, project name, and the informational `environment_vars` listing (full Ten_Port_Set with `*1` names) in the `jq` block and example JSON
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

  - [x] 6.2 Update `LOGS_context.md`
    - Apply port-calc block, env prefix, and project name edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 6.3 Update `MODIFY_CODE_context.md`
    - Apply port-calc block, env prefix, project name, and health-check edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 6.4 Update `VERIFY_APP_COMPLIANCE_context.md`
    - Apply port-calc block, env prefix, project name, and health-check edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 6.5 Update `MAKE_APP_COMPLIANT_context.md`
    - Apply port-calc block, env prefix, project name, and health-check edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

  - [x] 6.6 Update `SPECIFY_context.md`
    - Apply port-calc block, env prefix, and project name edits where present
    - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [x] 7. Apply canonical edits to README Agent_Docs
  - [x] 7.1 Update `README.md`
    - Apply port-calc block, env prefix, project name, informational listing, and firewall description edits where each pattern is present; set `RANGE_PORTS_PER_APPLICATION` to 10 where assigned
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

  - [x] 7.2 Update `README_SPECIFY.md`
    - Apply port-calc block, env prefix, project name, informational listing, and firewall edits where present; set `RANGE_PORTS_PER_APPLICATION` to 10 where assigned
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

- [x] 8. Verify no stale four-value references remain
  - [x] 8.1 Repo-wide search for stale references and fix any remaining
    - Grep the whole repository for word-boundary unsuffixed `HTTP_PORT` / `HTTPS_PORT` (not followed by a digit) and for `RANGE_PORTS_PER_APPLICATION=4`
    - Repoint or fix any remaining occurrences in `deployApp.sh` and Agent_Docs
    - _Requirements: 6.1, 6.2, 6.3_

  - [x]* 8.2 Write smoke test asserting absence of stale references
    - Assert `RANGE_PORTS_PER_APPLICATION=10` present and `=4` absent; assert `HTTP_PORT1`/`HTTPS_PORT1` present and unsuffixed names absent across script and docs; assert `HTTPS_PORT1` in project-name and health-check
    - _Requirements: 6.1, 6.2, 6.3_

- [x] 9. Final checkpoint - Ensure all tests pass
  - Ensure all property and smoke tests pass and the repo-wide search is clean, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional test sub-tasks and can be skipped for a faster MVP.
- Each task references specific granular requirements for traceability.
- Property tests validate the universal input-varying port arithmetic; smoke tests cover the rename and documentation-consistency requirements that do not vary with input.
- A doc receives only the edits for the patterns it actually contains; patterns absent from a doc are left unchanged.
- Checkpoints ensure incremental validation after the script and again after all docs.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3", "2.1", "2.2", "3.1", "3.2"] },
    { "id": 2, "tasks": ["2.3", "2.4", "3.3", "5.1", "5.2", "5.3", "5.4", "5.5", "5.6", "6.1", "6.2", "6.3", "6.4", "6.5", "6.6", "7.1", "7.2"] },
    { "id": 3, "tasks": ["8.1"] },
    { "id": 4, "tasks": ["8.2"] }
  ]
}
```
