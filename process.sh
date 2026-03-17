#!/usr/bin/env bash
set -euo pipefail
echo -e "alice 10\nbob 5\ncarol 20\ndave 15" | awk '{print $2, $1}' | sort -k1 -n | awk '{print $2 " -> " $1}' > /tmp/ranked.txt
cat /tmp/ranked.txt
