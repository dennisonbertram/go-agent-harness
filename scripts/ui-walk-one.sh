#!/bin/zsh
# Drive one conversation through the app's real UI and read the reply back.
#
# The point is to exercise a tool the way a user does — typed into the chat box,
# sent with the button, answer read off the transcript — rather than posting to
# the API. Anything that only breaks in the UI (a reply that never renders, a
# run that fires invisibly) is invisible to an HTTP sweep.
#
# Usage: uiwalk.sh "<prompt>" [seconds-to-wait]
set -u
DIR=/private/tmp/claude-501/-Users-dennison-develop-go-code/aefc480f-b989-4ee1-acd8-6b982c19c050/scratchpad
cd "$DIR"
PROMPT="$1"
TARGET="${TARGET:-$(cat /tmp/walkpid.txt)}"
WAIT="${2:-40}"

snap() {
  for i in $(seq 1 12); do
    out=$(timeout 20 ./nativeui snapshot --pid $TARGET 2>&1)
    if echo "$out" | grep -q '"count"'; then echo "$out"; return 0; fi
    perl -e 'select undef,undef,undef,1.5'
  done
  echo "$out"; return 1
}

# The AX tree goes briefly unavailable while SwiftUI rebuilds, so every step
# retries rather than treating one empty read as failure.
click() {
  for i in $(seq 1 10); do
    r=$(timeout 20 ./nativeui click "$1" --pid $TARGET 2>&1)
    echo "$r" | grep -q '"ok" : true' && return 0
    perl -e 'select undef,undef,undef,1'
  done
  return 1
}

# Start a fresh conversation so each tool is judged on its own transcript.
S=$(snap)
NEWREF=$(echo "$S" | python3 -c "
import sys,json
d=json.load(sys.stdin,strict=False)
for e in d.get('elements',[]):
    if (e.get('label') or '')=='New': print(e['ref']); break
")
[ -n "$NEWREF" ] && { click "$NEWREF" >/dev/null; perl -e 'select undef,undef,undef,2'; }

S=$(snap)
FIELD=$(echo "$S" | python3 -c "
import sys,json
d=json.load(sys.stdin,strict=False)
for e in d.get('elements',[]):
    if e.get('role')=='AXTextField': print(e['ref']); break
")
SEND=$(echo "$S" | python3 -c "
import sys,json
d=json.load(sys.stdin,strict=False)
for e in d.get('elements',[]):
    if e.get('role')=='AXButton' and (e.get('help') or '')=='Send': print(e['ref']); break
")
if [ -z "$FIELD" ] || [ -z "$SEND" ]; then echo "UIWALK_ERROR: no composer (field=$FIELD send=$SEND)"; exit 2; fi

printf '%s' "$PROMPT" | timeout 20 ./nativeui type "$FIELD" - --pid $TARGET >/dev/null 2>&1
perl -e 'select undef,undef,undef,1'
click "$SEND" >/dev/null || { echo "UIWALK_ERROR: send failed"; exit 2; }

# Poll the transcript until it stops growing, so a slow reply is not cut off.
prev=""; stable=0
for i in $(seq 1 "$WAIT"); do
  perl -e 'select undef,undef,undef,2'
  cur=$(snap | python3 -c "
import sys,json
d=json.load(sys.stdin,strict=False)
out=[]
for e in d.get('elements',[]):
    if e.get('role')=='AXStaticText':
        v=(e.get('value') or e.get('label') or '').strip()
        if v: out.append(v)
print(chr(10).join(out))
" 2>/dev/null)
  if [ "$cur" = "$prev" ] && [ -n "$cur" ]; then
    stable=$((stable+1)); [ $stable -ge 3 ] && break
  else
    stable=0
  fi
  prev="$cur"
done
echo "=== TRANSCRIPT ==="
# Only the tail after the echoed prompt is this tool's answer; everything above
# it belongs to an earlier exchange that the New click had not yet cleared.
echo "$prev" | python3 -c "
import sys
lines=[l.rstrip() for l in sys.stdin if l.strip()]
noise={'Done','Select a tool call to inspect it','uiwalk-ws','GoCode','Ready to leave plan mode'}
lines=[l for l in lines if l not in noise and not l.endswith('tok') and ' tok \u00b7 ' not in l]
# the prompt is echoed verbatim as a user bubble; take what follows the LAST copy
idx=max((i for i,l in enumerate(lines) if l.startswith(sys.argv[1][:40])), default=-1)
print(chr(10).join(lines[idx+1:]) if idx>=0 else chr(10).join(lines))
" "$PROMPT"
