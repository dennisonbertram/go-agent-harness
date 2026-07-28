#!/bin/zsh
# Wait for the screen to be usable, then walk every tool through the GUI.
#
# A locked session composites no windows, so a newly launched app never gets one
# and every accessibility read returns the menu bar. That is indistinguishable
# from a broken app unless you check the session state first — so this checks,
# waits, and only then drives anything.
set -u
S=/private/tmp/claude-501/-Users-dennison-develop-go-code/aefc480f-b989-4ee1-acd8-6b982c19c050/scratchpad
REPO=/Users/dennison/develop/go-code
LOG=/tmp/autowalk.log
: > $LOG

locked() {
  python3 - <<'PY'
import subprocess, sys
o = subprocess.run(['ioreg','-n','Root','-d1','-r'], capture_output=True, text=True).stdout
sys.exit(0 if '"CGSSessionScreenIsLocked"=Yes' in o else 1)
PY
}

echo "$(date '+%H:%M:%S') waiting for an unlocked session" >> $LOG
for i in $(seq 1 2880); do          # up to 24h at 30s
  if ! locked; then
    echo "$(date '+%H:%M:%S') session unlocked — starting" >> $LOG
    break
  fi
  sleep 30
done

# Fresh app against a fresh workspace so results are not polluted by earlier runs.
pkill -f "macapp/.build/debug/GoCode" 2>/dev/null
pkill -f "harnessd-bin/harnessd" 2>/dev/null
sleep 3
WS=/tmp/uiwalk-ws
rm -rf $WS && mkdir -p $WS/.harness
cp $REPO/go.mod $REPO/README.md $WS/ 2>/dev/null
(cd $WS && git init -q && git add -A && git -c user.email=a@b -c user.name=c commit -qm seed) 2>/dev/null

cd $REPO
nohup env HARNESS_BINARY=$REPO/.harnessd-bin/harnessd HARNESS_WORKSPACE=$WS \
  ./macapp/.build/debug/GoCode > $S/gocode.log 2>&1 &
sleep 18
PID=$(pgrep -f "macapp/.build/debug/GoCode" | head -1)
echo "$PID" > /tmp/walkpid.txt
echo "$(date '+%H:%M:%S') app pid $PID" >> $LOG

# The window can take a moment after launch; a menu-bar-only tree means it is not
# there yet, not that the app is broken.
cd $S
for i in $(seq 1 30); do
  timeout 25 ./nativeui snapshot --pid $PID > /tmp/snap.json 2>&1
  ok=$(python3 -c "
import json
try:
    d=json.load(open('/tmp/snap.json'))
    print('yes' if any(e.get('role')=='AXTextField' for e in d.get('elements',[])) else 'no')
except Exception:
    print('no')" 2>/dev/null)
  [ "$ok" = "yes" ] && { echo "$(date '+%H:%M:%S') composer ready" >> $LOG; break; }
  sleep 4
done
if [ "$ok" != "yes" ]; then
  echo "$(date '+%H:%M:%S') ABORT: no composer after 2 minutes" >> $LOG
  exit 2
fi

export TARGET=$PID
echo "$(date '+%H:%M:%S') walking $(wc -l < $S/uiwalk-tools.txt) tools" >> $LOG
$S/uiwalk-all.sh >> $LOG 2>&1
echo "$(date '+%H:%M:%S') WALK COMPLETE" >> $LOG
