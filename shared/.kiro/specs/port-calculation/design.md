# Design Document

## Overview

This feature expands the per-application port scheme in the deployment tooling from four ports (`HTTP_PORT`, `HTTPS_PORT`, `HTTP_PORT2`, `HTTPS_PORT2`) to ten ports, exposed as five HTTP ports (`HTTP_PORT1`..`HTTP_PORT5`) and five HTTPS ports (`HTTPS_PORT1`..`HTTPS_PORT5`). The ten ports form a single contiguous block laid out in an HTTP/HTTPS-interleaved consecutive order starting from a per-application base.

The change touches two kinds of artifacts:

1. **`deployApp.sh`** (Bash) — the authoritative Deployment_Script that computes ports, echoes an environment summary, passes ports to `docker-compose`, opens `ufw` firewall rules, builds the docker-compose project name, and issues a health-check `curl`.
2. **Agent_Docs** (Markdown) — the `*_context.md` files and the README files that duplicate the port-calc block and docker-compose invocation patterns for consumption by the genAI agent. These must be kept consistent with the script so documentation-driven operations match runtime behavior.

The design is deliberately mechanical: it defines one canonical port-computation block and one canonical env-passing prefix, then applies them uniformly across the script and every doc. No new abstractions are introduced; the work is a widening and renaming of an existing scheme plus removal of every stale four-value reference.

## Architecture

### Port layout

`RANGE_PORTS_PER_APPLICATION` changes from `4` to `10`. The base for an application instance is unchanged in formula:

```
PORT_RANGE_BEGIN = RANGE_START + USER_ID * RANGE_RESERVED
Port_Base        = PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION
```

The ten ports are assigned as consecutive interleaved offsets from `Port_Base`:

| Variable     | Value        | Offset |
|--------------|--------------|--------|
| `HTTP_PORT1`  | `Port_Base`      | +0 |
| `HTTPS_PORT1` | `Port_Base + 1`  | +1 |
| `HTTP_PORT2`  | `Port_Base + 2`  | +2 |
| `HTTPS_PORT2` | `Port_Base + 3`  | +3 |
| `HTTP_PORT3`  | `Port_Base + 4`  | +4 |
| `HTTPS_PORT3` | `Port_Base + 5`  | +5 |
| `HTTP_PORT4`  | `Port_Base + 6`  | +6 |
| `HTTPS_PORT4` | `Port_Base + 7`  | +7 |
| `HTTP_PORT5`  | `Port_Base + 8`  | +8 |
| `HTTPS_PORT5` | `Port_Base + 9`  | +9 |

General form for `n` in `1..5`:

```
HTTP_PORTn  = Port_Base + 2*(n-1)
HTTPS_PORTn = Port_Base + 2*(n-1) + 1
```

Because `RANGE_PORTS_PER_APPLICATION` is now `10` and the ten values occupy exactly offsets `0..9`, the block for one application does not overlap the block for the adjacent `APPLICATION_IDENTITY_NUMBER`. The former scheme reserved `4` but the new scheme requires and reserves `10`; updating the reservation constant is what keeps neighboring application blocks non-overlapping.

### Rename of the unsuffixed variables

The former unsuffixed `HTTP_PORT` / `HTTPS_PORT` become `HTTP_PORT1` / `HTTPS_PORT1`. Every downstream reference to the former unsuffixed names is repointed to the `*1` names, specifically:

- The docker-compose project name (`-p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}"`) becomes `...-${HTTPS_PORT1}`.
- The health-check URL (`https://${DOMAIN}:${HTTPS_PORT}/`) becomes `https://${DOMAIN}:${HTTPS_PORT1}/`.
- The `ufw` allow rule that used `HTTPS_PORT` becomes `HTTPS_PORT1`.

`HTTP_PORT2` / `HTTPS_PORT2` already exist and keep their names; their computed values shift into the interleaved layout (offsets +2 / +3), which is unchanged from the old `+2` / `+3` values, so their arithmetic is preserved. Only `RANGE_PORTS_PER_APPLICATION` and the addition of ports 3–5 change their downstream reservation footprint.

## Components and Interfaces

### `deployApp.sh` — `calculate_ports()`

The single source of truth for the port block. Replaces the four-line computation with the ten-value interleaved layout.

Current:

```bash
PORT_RANGE_BEGIN=$((RANGE_START + USER_ID * RANGE_RESERVED))
HTTP_PORT=$((PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION))
HTTPS_PORT=$((HTTP_PORT + 1))
HTTP_PORT2=$((HTTPS_PORT + 1))
HTTPS_PORT2=$((HTTP_PORT2 + 1))
```

Target:

```bash
PORT_RANGE_BEGIN=$((RANGE_START + USER_ID * RANGE_RESERVED))
HTTP_PORT1=$((PORT_RANGE_BEGIN + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION))
HTTPS_PORT1=$((HTTP_PORT1 + 1))
HTTP_PORT2=$((HTTPS_PORT1 + 1))
HTTPS_PORT2=$((HTTP_PORT2 + 1))
HTTP_PORT3=$((HTTPS_PORT2 + 1))
HTTPS_PORT3=$((HTTP_PORT3 + 1))
HTTP_PORT4=$((HTTPS_PORT3 + 1))
HTTPS_PORT4=$((HTTP_PORT4 + 1))
HTTP_PORT5=$((HTTPS_PORT4 + 1))
HTTPS_PORT5=$((HTTP_PORT5 + 1))
```

The chained `+ 1` form preserves the existing style and guarantees consecutiveness by construction. `RANGE_PORTS_PER_APPLICATION=4` is changed to `RANGE_PORTS_PER_APPLICATION=10` in the globals block.

### `deployApp.sh` — `show_environment()`

Echoes the full Ten_Port_Set, replacing the four `echo` lines with ten, each printing the variable name and value:

```bash
echo "  HTTP_PORT1=${HTTP_PORT1}"
echo "  HTTPS_PORT1=${HTTPS_PORT1}"
echo "  HTTP_PORT2=${HTTP_PORT2}"
echo "  HTTPS_PORT2=${HTTPS_PORT2}"
echo "  HTTP_PORT3=${HTTP_PORT3}"
echo "  HTTPS_PORT3=${HTTPS_PORT3}"
echo "  HTTP_PORT4=${HTTP_PORT4}"
echo "  HTTPS_PORT4=${HTTPS_PORT4}"
echo "  HTTP_PORT5=${HTTP_PORT5}"
echo "  HTTPS_PORT5=${HTTPS_PORT5}"
```

### `deployApp.sh` — docker-compose env-passing prefix

A canonical prefix passes all ten variables plus `USER_ID` to every `docker-compose` invocation. It appears in `deploy_services()` (down / build / up), in `create_backup_script()` (the generated `backup.sh`), and anywhere else a `docker-compose` command is prefixed with port variables.

```bash
HTTP_PORT1=$HTTP_PORT1 HTTPS_PORT1=$HTTPS_PORT1 \
HTTP_PORT2=$HTTP_PORT2 HTTPS_PORT2=$HTTPS_PORT2 \
HTTP_PORT3=$HTTP_PORT3 HTTPS_PORT3=$HTTPS_PORT3 \
HTTP_PORT4=$HTTP_PORT4 HTTPS_PORT4=$HTTPS_PORT4 \
HTTP_PORT5=$HTTP_PORT5 HTTPS_PORT5=$HTTPS_PORT5 \
USER_ID=$USER_ID docker-compose -p "${NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT1}" -f docker-compose.yml ...
```

The project-name segment uses `HTTPS_PORT1` (formerly `HTTPS_PORT`). In-line for readability the prefix may be kept on one line as in the current script; the requirement is that all ten variables are present in every invocation.

### `deployApp.sh` — `setup_firewall()`

Opens a `ufw` allow rule for each Ten_Port_Set variable that has a non-empty value, guarded by the existing `command -v ufw` check. Each rule is wrapped in a non-empty guard so an unset/empty port is skipped:

```bash
if command -v ufw &> /dev/null; then
    for port in "$HTTP_PORT1" "$HTTPS_PORT1" \
                "$HTTP_PORT2" "$HTTPS_PORT2" \
                "$HTTP_PORT3" "$HTTPS_PORT3" \
                "$HTTP_PORT4" "$HTTPS_PORT4" \
                "$HTTP_PORT5" "$HTTPS_PORT5"; do
        if [[ -n "$port" ]]; then
            sudo ufw allow "${port}/tcp"
        fi
    done
    log_info "Firewall configured ✅"
else
    log_warn "UFW not found, skipping firewall configuration"
fi
```

A loop is used in place of ten repeated `if [[ -n ... ]]` blocks to keep the non-empty guard uniform across all ten ports and to avoid drift. Retaining the current explicit per-variable `if` blocks (expanded to ten) is an equally valid implementation; either satisfies Requirement 4. The behavior — allow rule for exactly the non-empty ports — is what matters.

### `deployApp.sh` — `check_status()`

The status JSON currently reports `HTTP_PORT`, `HTTPS_PORT`, `HTTP_PORT2`, `HTTPS_PORT2`. It is updated to read the renamed/added variables and emit the full Ten_Port_Set under `environment_vars`, using the `*1` names in place of the unsuffixed names. This is where the requirement to leave no stale unsuffixed reference (Requirement 6.1) intersects with the informational listing (Requirement 3.2 / 5.5).

### Agent_Docs — shared edits

Each Agent_Doc that references port logic is edited by applying the same canonical blocks. The recurring patterns and their target forms:

1. **Port-calc block** (present in `STOP_context.md`, `PS_context.md`, `CLEAN_DATABASE_context.md`, `RESTORE_DATABASE_context.md`, and other `*_context.md` files, plus README files where duplicated):

   ```bash
   export PORT_RANGE_BEGIN=$((RANGE_START+USER_ID*RANGE_RESERVED))
   export HTTP_PORT1=$((PORT_RANGE_BEGIN+APPLICATION_IDENTITY_NUMBER*RANGE_PORTS_PER_APPLICATION))
   export HTTPS_PORT1=$((HTTP_PORT1+1))
   export HTTP_PORT2=$(($HTTPS_PORT1+1))
   export HTTPS_PORT2=$(($HTTP_PORT2+1))
   export HTTP_PORT3=$(($HTTPS_PORT2+1))
   export HTTPS_PORT3=$(($HTTP_PORT3+1))
   export HTTP_PORT4=$(($HTTPS_PORT3+1))
   export HTTPS_PORT4=$(($HTTP_PORT4+1))
   export HTTP_PORT5=$(($HTTPS_PORT4+1))
   export HTTPS_PORT5=$(($HTTP_PORT5+1))
   ```

2. **docker-compose env-passing lines** (e.g. the `... stop` / `... up -d` lines in the database context docs): prefix rewritten to carry all ten variables and use `HTTPS_PORT1` in the `-p` project name.

3. **Project-name references** (`-p "$NAME_OF_APPLICATION-$USER_ID-$HTTPS_PORT"`): `HTTPS_PORT` → `HTTPS_PORT1`.

4. **Health-check / site-URL references** (e.g. `https://www.${DOMAIN}:$HTTPS_PORT`): `HTTPS_PORT` → `HTTPS_PORT1`.

5. **Informational listings** (e.g. the `jq` `environment_vars` block and its example JSON in `PS_context.md`): list the complete Ten_Port_Set with `*1` names replacing unsuffixed names.

6. **`RANGE_PORTS_PER_APPLICATION` assignments** in docs: any literal `4` → `10`.

7. **Firewall descriptions** in docs (where present): describe allow rules covering the complete Ten_Port_Set.

The full list of Agent_Docs to update: `STOP_context.md`, `CLEAN_DATABASE_context.md`, `RESTORE_DATABASE_context.md`, `PS_context.md`, `MODIFY_CODE_context.md`, `VERIFY_APP_COMPLIANCE_context.md`, `START_context.md`, `LOGS_context.md`, `RESTART_context.md`, `MAKE_APP_COMPLIANT_context.md`, `BACKUP_DATABASE_context.md`, `SPECIFY_context.md`, `README.md`, `README_SPECIFY.md`. A doc receives only the edits for the patterns it actually contains; docs without a given pattern (e.g. a doc with no firewall text) are left unchanged for that pattern.

## Data Models

There is no persistent data model. The domain is a fixed-size record of ten integer port values derived from three inputs:

```
Inputs:
  USER_ID                     : non-negative integer (non-numeric coerced to 0)
  APPLICATION_IDENTITY_NUMBER : non-negative integer (from deploy.ini)
  RANGE_START, RANGE_RESERVED : configured constants
  RANGE_PORTS_PER_APPLICATION : constant = 10

Derived:
  PORT_RANGE_BEGIN : integer
  Port_Base        : integer
  Ten_Port_Set     : ordered tuple
                     (HTTP_PORT1, HTTPS_PORT1, HTTP_PORT2, HTTPS_PORT2,
                      HTTP_PORT3, HTTPS_PORT3, HTTP_PORT4, HTTPS_PORT4,
                      HTTP_PORT5, HTTPS_PORT5)
                   = (Port_Base+0 .. Port_Base+9)
```

Invariants on the derived tuple:
- The ten values are exactly the consecutive integers `Port_Base` through `Port_Base + 9`.
- All ten values are distinct.
- `HTTP_PORTn = Port_Base + 2*(n-1)` and `HTTPS_PORTn = HTTP_PORTn + 1` for `n` in `1..5`.

## Error Handling

- **Non-numeric `USER_ID`**: existing guard coerces it to `0` before computing ports. Unchanged.
- **Empty port variable at firewall step**: each `ufw allow` is guarded by a non-empty check; empty values are skipped rather than producing an invalid `ufw allow /tcp` rule (Requirement 4.2).
- **Missing `ufw`**: the existing `command -v ufw` guard short-circuits firewall setup with a warning. Unchanged.
- **Arithmetic safety**: all port expressions use Bash `$(( ))` integer arithmetic; inputs are integers, so no overflow or type coercion issues arise within the expected port range.
- **Consistency risk between script and docs**: the primary error class introduced by this change is a doc drifting from the script (a stale unsuffixed name or `RANGE_PORTS_PER_APPLICATION=4` left behind). Requirement 6 makes absence of stale references an explicit acceptance criterion; verification includes a repository-wide search for unsuffixed `HTTP_PORT`/`HTTPS_PORT` word-boundary matches and for `RANGE_PORTS_PER_APPLICATION=4`.

## Testing Strategy

Because the artifacts are Bash and Markdown, testing combines:

- **Property tests** (Bats or a small harness invoking the sourced functions, or a language-agnostic runner that evaluates the arithmetic): drive `calculate_ports`-equivalent logic with randomized `USER_ID` / `APPLICATION_IDENTITY_NUMBER` and assert the layout invariants. Minimum 100 iterations per property. Each property test is tagged **Feature: port-calculation, Property {number}: {property_text}**.
- **Example/smoke tests**: static assertions over the script and docs — presence of `RANGE_PORTS_PER_APPLICATION=10`, presence of `HTTP_PORT1`/`HTTPS_PORT1`, `HTTPS_PORT1` in project-name and health-check, absence of unsuffixed names and of `RANGE_PORTS_PER_APPLICATION=4`.
- **Integration-style checks**: for each docker-compose invocation line, assert all ten variables are present in the env prefix.

Property tests cover universal input-varying logic; example/smoke tests cover the rename, static content, and documentation-consistency requirements that do not vary with input.

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Port base formula

*For any* non-negative `USER_ID` and `APPLICATION_IDENTITY_NUMBER`, the computed `Port_Base` equals `RANGE_START + USER_ID * RANGE_RESERVED + APPLICATION_IDENTITY_NUMBER * RANGE_PORTS_PER_APPLICATION`, with `RANGE_PORTS_PER_APPLICATION` equal to 10.

**Validates: Requirements 1.1, 1.2**

### Property 2: Ten-port interleaved consecutive layout

*For any* computed `Port_Base`, the ten port variables satisfy `HTTP_PORTn = Port_Base + 2*(n-1)` and `HTTPS_PORTn = Port_Base + 2*(n-1) + 1` for every `n` in `1..5`; equivalently, the ten values are exactly the distinct consecutive integers `Port_Base` through `Port_Base + 9`.

**Validates: Requirements 1.3, 1.4, 1.5, 1.6, 1.7, 1.8**

### Property 3: Environment summary lists the full Ten_Port_Set

*For any* assigned Ten_Port_Set, the environment summary output contains each of the ten variable names paired with its assigned value.

**Validates: Requirements 3.2**

### Property 4: docker-compose invocations propagate the full Ten_Port_Set

*For any* docker-compose invocation issued by the Deployment_Script, the invocation's environment prefix includes all ten variables of the Ten_Port_Set, each bound to its corresponding computed value.

**Validates: Requirements 3.1**

### Property 5: Firewall rules match exactly the non-empty ports

*For any* assignment of the Ten_Port_Set in which some variables may be empty, when `ufw` is available the set of allow rules issued corresponds exactly to the variables with non-empty values, and no rule is issued for an empty variable.

**Validates: Requirements 4.1, 4.2**
