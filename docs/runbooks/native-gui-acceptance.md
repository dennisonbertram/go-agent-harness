# Native GUI Acceptance

Issue #1089 validates actual rendered macOS evidence; `ToolWalk`, reducer
tests, an HTTP acknowledgement, and assistant prose are insufficient.

Run only through the owner, which creates a private `0700` root, builds its
fixed `harnessd` probe there, reserves a fresh loopback endpoint, and starts
its fake-provider daemon and GoCode app children. It accepts no caller URL,
driver, manifest, app bundle, artifact root, or cleanup selector. Do not
reuse, kill, or attach to an existing GoCode or harnessd process.

The app can take foreground only after explicit operator opt-in. Invoke:

```bash
./scripts/run-native-gui-acceptance.sh
```

The command now attempts exactly one #1220 core two-message rendered scenario.
It first calls macOS's non-prompting Accessibility and Screen Recording
preflight APIs. If either grant is not already available, it stops before the
owner creates a root, reserves a port, or launches a child. It never requests a
grant, opens System Settings, clicks a consent dialog, or treats an environment
variable as permission evidence. Granting permission is a separate manual
operator action; rerun the command afterwards.

When admission succeeds, the owner creates separate private runtime and
retained-artifact roots. It builds and starts only its own `harnessd` and GoCode
children, submits a nonce-bearing initial prompt, then targets the attested app
PID through Accessibility to enter and send a second prompt. It captures:

- the owner-created app window as PNG;
- the app PID's accessibility tree;
- raw SSE for both completed runs;
- conversation messages and run-store API responses;
- owner-created daemon and app logs; and
- `proof.json`, which binds nonce, child PIDs, one conversation, two run IDs,
  relative artifact paths, byte lengths, SHA-256 digests, and cleanup evidence.

PASS requires all six artifacts to be distinct, nonempty, regular files under
the retained root. AX, SSE, and API/store evidence must all contain the exact
two prompts, two replies, nonce, conversation ID, and both run IDs. The PNG
must have a PNG signature. Child shutdown and disposable-root removal must be
verified before `proof.json` can be finalized. A scenario or cleanup failure
writes `failure.json` and reports the exact retained diagnostic path instead of
claiming rendered success.

This is one core rendered foundation scenario, not the #1089 matrix and not
#1010 convergence. Cron/callback rendered cases remain unproven here.

## Issue #1208 deterministic scenario preflight

The owner now creates a fresh nonce and validates a fixed fake-provider fixture
before it can reserve a port or spawn either child. The fixture contains three
future rendered cases: a core `ls` tool followed by a second message in the
same Chat conversation; a `cron_create` scheduled continuation; and a
`set_delayed_callback` one-shot continuation. Each case predeclares distinct
nonce-scoped screenshot, AX snapshot, raw SSE, and API/store probe paths plus
run/conversation ID markers. This is preflight input only, not evidence and
not a GUI PASS.

The core driver uses only the first scenario's three fake turns. The cron and
callback declarations remain preflight input for later issues. Foreground opt-in
does not permit discovery, attachment, reuse, or termination of any pre-existing
app, daemon, or tmux session, and it does not authorize the driver to accept a
TCC prompt. #1089's complete applicability matrix and validator remain separate
acceptance gates.
