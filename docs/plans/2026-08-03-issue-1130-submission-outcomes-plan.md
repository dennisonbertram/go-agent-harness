# Issue #1130: Submission-local Outcomes Plan

## Intent and scope

An A submission must retain its own start, terminal, and failure outcome when
a callback/cron continuation B becomes the visible conversation run. This is a
native-client and ToolWalk ownership repair stacked on #1128 (`654b7da`): no
server, scheduler, TUI, or callback implementation changes are in scope.

## Design

`RunSubmission` will hold two independent facts: an A-local lifecycle
(`starting`, `started`, `terminal`, or `failed`) and a displacement bit.
Displacement prevents controls from being sent to selected B; it must not erase
an A terminal/failure that ToolWalk needs to judge truthfully. `RunSession`
will retain/clear `activeSubmission` by object identity, bind a late A
`startRun` acknowledgement to its handle before checking displacement, and
apply visible failure/accounting/activation only while that exact submission is
still the selected A owner. Reset/load detach the handle; stream EOF produces
an A-local failure and only marks the visible transcript failed when A owns it.

`ToolWalk.Runner` will use a typed wait outcome. Terminal and failure are
consumed before displacement; only an actual timeout invokes A's guarded cancel
endpoint. A displaced or failed submission produces a result without any B
action.

## Test-first plan

1. Add deterministic barrier tests that currently fail: terminal A then
   select B before ToolWalk observes it; delayed A acknowledgement after B;
   late A start/stream failure after B; reset/load detachment; and EOF failure
   ownership.
2. Add ToolWalk outcome tests proving only `.timedOut` requests cancel and that
   A terminal is judged rather than reported as a timeout.
3. Implement the smallest lifecycle/identity changes, then run focused Swift,
   all Swift tests, format, and the full repository regression gate.

## Rollout and rollback

This is an in-process macOS/ToolWalk reducer change with no persisted or wire
format change. Rollback is a normal PR revert. The safety trigger is any
evidence that an A-local completion changes B's visible state or that a timeout
action targets B; the barrier regressions prevent both.
