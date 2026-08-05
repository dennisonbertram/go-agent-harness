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
