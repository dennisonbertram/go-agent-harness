#!/bin/zsh
S=/private/tmp/claude-501/-Users-dennison-develop-go-code/aefc480f-b989-4ee1-acd8-6b982c19c050/scratchpad
OUT=/tmp/uiwalk-results.txt
export TARGET="${TARGET:-$(cat /tmp/walkpid.txt)}"
: > $OUT
while IFS='|' read -r -u 3 tool prompt; do
  [ -z "$tool" ] && continue
  echo "### $tool" >> $OUT
  res=$($S/uiwalk.sh "$prompt" 22 </dev/null 2>&1 | sed -n '/=== TRANSCRIPT ===/,$p' | tail -n +2)
  echo "$res" >> $OUT
  echo "" >> $OUT
  echo "walked: $tool"
done 3< $S/uiwalk-tools.txt
echo "UIWALK COMPLETE"
