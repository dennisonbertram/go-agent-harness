#!/bin/zsh
# Walk one tool through the real GUI and read the answer off the screen.
#
# Accessibility is gated while the session is locked and the session event tap
# swallows synthetic input — but CGEventPostToPid reaches the process directly
# and the window still composites. So this clicks, types and sends for real,
# then reads the rendered pixels with Vision OCR rather than trusting a reply.
set -u
PID="${PID:?set PID}"
WIN="${WIN:?set WIN}"
FIELD_X="${FIELD_X:-460}"; FIELD_Y="${FIELD_Y:-967}"
NEW_X="${NEW_X:-1304}";    NEW_Y="${NEW_Y:-995}"
PROMPT="$1"
MAXWAIT="${2:-25}"
SHOT="/tmp/gauntlet/walk-$$.png"

screen_text() {
  screencapture -x -o -l"$WIN" "$SHOT" 2>/dev/null
  /tmp/guidrive "$PID" ocr "$SHOT" 2>/dev/null
}

# No new-conversation click: that control does not accept a synthetic click and
# the app exposes no keyboard shortcut for it. Attribution comes from diffing
# against the pre-send screen instead, which is strictly more reliable anyway.

# Focus the composer and clear it. cmd+A does not survive the direct-post route,
# so this deletes character by character — crude, but it actually works.
/tmp/guidrive "$PID" click $FIELD_X $FIELD_Y >/dev/null 2>&1
sleep 1
for i in $(seq 1 220); do /tmp/guidrive "$PID" delete >/dev/null 2>&1; done

/tmp/guidrive "$PID" type "$PROMPT" >/dev/null 2>&1
sleep 1

# Record what is on screen BEFORE sending. Waiting only for the text to settle
# accepts the previous exchange when a reply is slow, which silently offsets
# every result by one tool — the answers look plausible and belong to the wrong
# tool. Requiring the screen to differ from this baseline makes that impossible.
baseline="$(screen_text)"
/tmp/guidrive "$PID" return >/dev/null 2>&1

# Wait for a real answer, not merely for the screen to change. The echoed prompt
# renders first, so settling on "something changed" accepts the prompt and
# attributes the reply — which arrives moments later — to the NEXT tool. Every
# result then looks plausible and belongs to the wrong tool.
#
# So: require new content that is not just the prompt echo, and require the app
# to be idle again ("Ready") before accepting it.
prev=""; stable=0; answered=0
for i in $(seq 1 "$MAXWAIT"); do
  sleep 3
  cur="$(screen_text)"
  extra="$(python3 - "$baseline" "$cur" "$PROMPT" <<'PYEOF'
import sys
before = set(l.strip() for l in sys.argv[1].split(chr(10)) if l.strip())
prompt_words = set(w.lower() for w in sys.argv[3].split())
out = []
for line in sys.argv[2].split(chr(10)):
    t = line.strip()
    if not t or t in before:
        continue
    words = set(w.lower() for w in t.split())
    # A line that is entirely prompt vocabulary is the echo, not an answer.
    if words and words <= prompt_words:
        continue
    out.append(t)
print(chr(10).join(out))
PYEOF
)"
  # The assistant's reply renders with a leading "+", which is a far more
  # reliable signal than trying to filter the echoed prompt by word overlap —
  # OCR mangles enough characters that the echo slips through the filter and
  # gets captured as though it were the answer.
  if echo "$cur" | grep -q "^+ "; then
    answered=1
    if [ "$cur" = "$prev" ]; then
      stable=$((stable+1)); [ $stable -ge 3 ] && break
    else
      stable=0
    fi
  fi
  prev="$cur"
done
rm -f "$SHOT"
if [ "$answered" -eq 0 ]; then
  echo "GUIWALK_NO_RESPONSE"
else
  # Only what appeared after this prompt was sent. Printing the whole transcript
  # would attribute an earlier tool's answer to this one.
  python3 - "$baseline" "$prev" <<'PYEOF'
import sys
before = set(l.strip() for l in sys.argv[1].split(chr(10)) if l.strip())
for line in sys.argv[2].split(chr(10)):
    t = line.strip()
    if t and t not in before:
        print(t)
PYEOF
fi
