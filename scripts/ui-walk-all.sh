#!/bin/zsh
S=/private/tmp/claude-501/-Users-dennison-develop-go-code/aefc480f-b989-4ee1-acd8-6b982c19c050/scratchpad
OUT=/tmp/uiwalk-results.txt
export TARGET="${TARGET:-$(cat /tmp/walkpid.txt)}"
: > $OUT
while IFS='|' read -r -u 3 tool prompt; do
  [ -z "$tool" ] && continue
  echo "### $tool" >> $OUT
  # One retry: a single empty read is usually the AX tree rebuilding after a
  # view swap, not a tool that failed. Reporting that as a failure is how an
  # earlier run produced 23 false negatives.
  res=$($S/uiwalk.sh "$prompt" 22 </dev/null 2>&1 | sed -n '/=== TRANSCRIPT ===/,$p' | tail -n +2)
  if [ -z "$(echo "$res" | tr -d '[:space:]')" ]; then
    sleep 3
    res=$($S/uiwalk.sh "$prompt" 26 </dev/null 2>&1 | sed -n '/=== TRANSCRIPT ===/,$p' | tail -n +2)
    echo "RETRY" >> /tmp/uiwalk-retries.txt
  fi
  echo "$res" >> $OUT
  echo "" >> $OUT
  echo "walked: $tool"
done 3< $S/uiwalk-tools.txt
echo "UIWALK COMPLETE"
