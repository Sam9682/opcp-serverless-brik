#!/bin/bash
set -u
D=/home/slepetre/workspace/forgejo/ai-swautomorph--shared/deployApp.sh
export USER_NAME=admin USER_EMAIL=t@e.com NAME_OF_APPLICATION=myapp
PASS=0; FAIL=0
ok(){ PASS=$((PASS+1)); }
bad(){ echo "  FAIL: $1"; FAIL=$((FAIL+1)); }
RANGE_START=6000; RANGE_RESERVED=100; RANGE_PORTS_PER_APPLICATION=12
# NEW ordering: HTTPS_PORT(main), HTTP_PORT, then HTTPS/HTTP pairs 1..5
PORT_NAMES=(HTTPS_PORT HTTP_PORT HTTPS_PORT1 HTTP_PORT1 HTTPS_PORT2 HTTP_PORT2 HTTPS_PORT3 HTTP_PORT3 HTTPS_PORT4 HTTP_PORT4 HTTPS_PORT5 HTTP_PORT5)
awk "/^calculate_ports\(\) \{/,/^\}/" "$D" > /tmp/c.sh
awk "/^port_env_prefix\(\) \{/,/^\}/" "$D" > /tmp/pe.sh
awk "/^show_environment\(\) \{/,/^\}/" "$D" > /tmp/sh.sh
awk "/^setup_firewall\(\) \{/,/^\}/" "$D" > /tmp/fw.sh
awk "/^check_status\(\) \{/,/^\}/" "$D" > /tmp/cs.sh
source /tmp/c.sh; source /tmp/pe.sh; source /tmp/sh.sh; source /tmp/fw.sh

# ordering across 150 random iters
echo "== ordering (150 iters) =="
for i in $(seq 1 150); do
  USER_ID=$((RANDOM%500)); APPLICATION_IDENTITY_NUMBER=$((RANDOM%50))
  calculate_ports
  base=$((6000+USER_ID*100+APPLICATION_IDENTITY_NUMBER*12))
  g=1
  [[ "$HTTPS_PORT" -eq $((base+0)) ]]||{ g=0; bad "HTTPS_PORT main"; }
  [[ "$HTTP_PORT"  -eq $((base+1)) ]]||{ g=0; bad "HTTP_PORT"; }
  [[ "$HTTPS_PORT1" -eq $((base+2)) ]]||{ g=0; bad "HTTPS_PORT1"; }
  [[ "$HTTP_PORT1"  -eq $((base+3)) ]]||{ g=0; bad "HTTP_PORT1"; }
  [[ "$HTTPS_PORT5" -eq $((base+10)) ]]||{ g=0; bad "HTTPS_PORT5"; }
  [[ "$HTTP_PORT5"  -eq $((base+11)) ]]||{ g=0; bad "HTTP_PORT5"; }
  vv=$(for n in "${PORT_NAMES[@]}"; do echo "${!n}"; done|sort -n|uniq|tr "\n" " ")
  ww=$(seq $base $((base+11))|tr "\n" " ")
  [[ "$vv" == "$ww" ]]||{ g=0; bad "set"; }
  [[ $g -eq 1 ]] && ok
done

USER_ID=2; APPLICATION_IDENTITY_NUMBER=4; calculate_ports
# main port = base (offset 0)
[[ "$HTTPS_PORT" -eq $((6000+200+48)) ]] && ok || bad "main==base"
# show_environment: first port line is HTTPS_PORT
o=$(show_environment ps)
firstport=$(echo "$o"|grep -E "HTTP" | head -1)
echo "$firstport" | grep -q "HTTPS_PORT=" && ok || bad "show_env first not HTTPS_PORT: $firstport"
# env prefix first token HTTPS_PORT
p=$(port_env_prefix)
[[ "$p" == HTTPS_PORT=* ]] && ok || bad "prefix first not HTTPS_PORT"
# firewall order: first allowed is HTTPS_PORT
ALLOWED=""
ufw(){ [[ "$1" == "allow" ]] && ALLOWED="$ALLOWED ${2%%/*}"; }
sudo(){ [[ "$1" == "ufw" ]] && { shift; ufw "$@"; }; }
command(){ [[ "${2:-}" == "ufw" ]] && return 0; builtin command "$@"; }
log_info(){ :; }; log_warn(){ :; }
setup_firewall
firstfw=$(echo "$ALLOWED"|awk "{print \$1}")
[[ "$firstfw" -eq "$HTTPS_PORT" ]] && ok || bad "fw first=$firstfw want=$HTTPS_PORT"
# project name & health-check use HTTPS_PORT (not HTTPS_PORT1)
grep -q 'NAME_OF_APPLICATION}-${USER_ID}-${HTTPS_PORT}"' "$D" && ok || bad "projname not HTTPS_PORT"
grep -q 'https://${DOMAIN}:${HTTPS_PORT}/' "$D" && ok || bad "health not HTTPS_PORT"
# check_status JSON: first env port key = HTTPS_PORT, 15 keys
USER_ID=1; APPLICATION_IDENTITY_NUMBER=0; calculate_ports
docker(){ return 1; }; git(){ echo ""; }
source /tmp/cs.sh
j=$(check_status)
fk=$(echo "$j"|jq -r '.environment_vars|keys_unsorted[3]')
[[ "$fk" == "HTTPS_PORT" ]] && ok || bad "json first port key=$fk"
kc=$(echo "$j"|jq '.environment_vars|keys|length'); [[ "$kc" -eq 15 ]] && ok || bad "keys=$kc"
bash -n "$D" && ok || bad "syntax"
echo ""; echo "PASSED=$PASS FAILED=$FAIL"
[[ $FAIL -eq 0 ]]