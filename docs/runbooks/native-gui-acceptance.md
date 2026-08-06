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

This establishes lifecycle ownership only. It does not drive AX/OCR, submit a
prompt, collect screenshots, write a manifest, or claim a rendered scenario
passed. Separate native scenario evidence must be planned and run with its own
operator/TCC authorization; lifecycle startup is never GUI acceptance proof.

## Issue #1208 deterministic scenario preflight

The owner now creates a fresh nonce and validates a fixed fake-provider fixture
before it can reserve a port or spawn either child. The fixture contains three
future rendered cases: a core `ls` tool followed by a second message in the
same Chat conversation; a `cron_create` scheduled continuation; and a
`set_delayed_callback` one-shot continuation. Each case predeclares distinct
nonce-scoped screenshot, AX snapshot, raw SSE, and API/store probe paths plus
run/conversation ID markers. This is preflight input only, not evidence and
not a GUI PASS.

An operator may execute the later driver only after explicitly authorizing all
of the following: foregrounding this owner-created isolated GoCode app, typing
and navigating its controls, and accepting Accessibility and Screen Recording
prompts if macOS requires them. That authorization does not permit discovery,
attachment, reuse, or termination of any pre-existing app, daemon, or tmux
session. The driver must preserve a real failed proof pack on any mismatch and
may report PASS only after its four artifacts carry matching nonce, run, and
conversation identities and the #1089 validator accepts them.
