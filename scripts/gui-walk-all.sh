#!/bin/zsh
# Walk every tool through the rendered GUI.
#
# Two constraints shape this. Synthetic clicks do not register on buttons while
# the session is locked (the app cannot become active), but the text field takes
# focus and the keyboard works — so every tool is driven by typing and Return.
# And there is no reachable new-conversation control, so the app is restarted
# periodically: that keeps each transcript short and clears any blocking prompt.
set -u
S=/private/tmp/claude-501/-Users-dennison-develop-go-code/aefc480f-b989-4ee1-acd8-6b982c19c050/scratchpad
REPO=/Users/dennison/develop/go-code
OUT=/tmp/guiwalk-results.txt
BATCH="${BATCH:-10}"
: > $OUT

restart_app() {
  pkill -f "macapp/.build/debug/GoCode" 2>/dev/null
  sleep 3
  cd $REPO
  nohup env HARNESS_BINARY=$REPO/.harnessd-bin/harnessd HARNESS_WORKSPACE=/tmp/uiwalk-ws \
    ./macapp/.build/debug/GoCode > $S/gocode.log 2>&1 &
  sleep 16
  local p w
  p=$(pgrep -f "macapp/.build/debug/GoCode" | head -1)
  w=$(/tmp/winlist "$p" 2>/dev/null | grep "^window:" | python3 -c "
import sys,re
best,area=None,0
for l in sys.stdin:
    i=re.search(r'\bid=(\d+)',l); wd=re.search(r'\"Width\": (\d+)',l); h=re.search(r'\"Height\": (\d+)',l)
    if i and wd and h:
        a=int(wd.group(1))*int(h.group(1))
        if a>area: area,best=a,i.group(1)
print(best or '')")
  echo "$p" > /tmp/guipid.txt
  echo "$w" > /tmp/guiwin.txt
  echo "  [restarted pid=$p win=$w]"
}

restart_app
n=0
while IFS='|' read -r -u 3 tool prompt; do
  [ -z "$tool" ] && continue
  n=$((n+1))
  # A fresh app per tool makes the baseline an empty transcript, so whatever
  # appears is unambiguously this tool's answer. The transcript scrolls, which
  # made every delta-based attribution noisy.
  if [ $n -gt 1 ] && [ $(( (n-1) % BATCH )) -eq 0 ]; then restart_app; fi
  echo "### $tool" >> $OUT
  PID=$(cat /tmp/guipid.txt) WIN=$(cat /tmp/guiwin.txt) \
    $S/guiwalk.sh "$prompt" 22 </dev/null >> $OUT 2>&1
  echo "" >> $OUT
  echo "[$n] $tool"
done 3< $S/uiwalk-tools.txt
echo "GUIWALK COMPLETE ($n tools)"
