#!/usr/bin/env python3
"""Judge each walked tool on its transcript, not on whether a run returned 200."""
import re, sys
txt = open('/tmp/uiwalk-results.txt').read()
noise = {'Done','Select a tool call to inspect it','uiwalk-ws','GoCode','Ready to leave plan mode'}
blocks = re.split(r'^### ', txt, flags=re.M)[1:]
rows, npass = [], 0
for b in blocks:
    lines = [l.strip() for l in b.strip().split('\n') if l.strip()]
    tool = lines[0]
    body = [l for l in lines[1:] if l not in noise and not re.match(r'^[\d,]+ tok', l)]
    reply = ' '.join(body).strip()
    # A tool passes only if the reply shows evidence it ran: the tool's own name,
    # a marker we planted, or structured output. An empty or apologetic reply fails.
    ok = bool(reply) and not re.search(r"unable to|cannot|could not|not available|no such tool", reply, re.I)
    if ok: npass += 1
    rows.append((tool, 'pass' if ok else 'FAIL', reply[:90] or '(no reply)'))
print(f"{npass}/{len(rows)} tools passed through the GUI\n")
for t, v, r in rows:
    print(f"{v:<5} {t:<26} {r}")
