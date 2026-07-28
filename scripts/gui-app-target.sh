#!/bin/zsh
# Resolve, and PROVE, which GoCode instance you are about to drive.
#
# Several GoCode processes routinely coexist — one per worktree build, plus
# whatever a subagent launched — and they all answer to the same app name. Data
# gathered from the wrong one looks entirely plausible, which is how a stale
# build got reported as a new measurement twice.
#
# Usage: gui-app-target.sh <expected-workspace-name>
#   prints "<pid> <window-id>" and exits non-zero if identity cannot be proven.
set -u
EXPECT="${1:?usage: gui-app-target.sh <expected-workspace-name>}"

pids=$(pgrep -f "macapp/.build/debug/GoCode" | tr '\n' ' ')
[ -z "$pids" ] && { echo "no GoCode process running" >&2; exit 2; }

count=$(echo "$pids" | wc -w | tr -d ' ')
[ "$count" -gt 1 ] && echo "note: $count GoCode instances running ($pids) — matching on workspace" >&2

for pid in ${=pids}; do
  win=$(/tmp/winlist "$pid" 2>/dev/null | grep "^window:" | python3 -c "
import sys,re
best,area=None,0
for l in sys.stdin:
    i=re.search(r'\bid=(\d+)',l); w=re.search(r'\"Width\": (\d+)',l); h=re.search(r'\"Height\": (\d+)',l)
    if i and w and h:
        a=int(w.group(1))*int(h.group(1))
        if a>area: area,best=a,i.group(1)
print(best or '')")
  [ -z "$win" ] && continue
  # The window title carries the workspace name. That is the only identity
  # check that survives a stale process still holding an old window.
  shot=$(mktemp -t gocodeid).png
  screencapture -x -o -l"$win" "$shot" 2>/dev/null
  title=$(/tmp/guidrive "$pid" ocr "$shot" 2>/dev/null | head -1)
  rm -f "$shot"
  if echo "$title" | grep -q "$EXPECT"; then
    # Matching the workspace name proves WHICH app, not WHICH BUILD. A critic
    # once certified a binary started six minutes before the fix it was sent to
    # verify — right window, stale code. So also require the running process to
    # be no older than the binary it was launched from.
    if [ -n "${BINARY:-}" ] && [ -f "$BINARY" ]; then
      started=$(ps -o lstart= -p "$pid" 2>/dev/null)
      started_epoch=$(date -j -f "%a %b %d %T %Y" "$started" +%s 2>/dev/null || echo 0)
      binary_epoch=$(stat -f %m "$BINARY" 2>/dev/null || echo 0)
      if [ "$started_epoch" -gt 0 ] && [ "$binary_epoch" -gt "$started_epoch" ]; then
        echo "pid $pid is running a build older than $BINARY — relaunch before measuring" >&2
        exit 4
      fi
    fi
    echo "$pid $win"
    exit 0
  fi
done

echo "no GoCode window titled '$EXPECT' — refusing to guess" >&2
exit 3
