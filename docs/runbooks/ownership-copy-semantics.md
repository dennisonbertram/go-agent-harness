# Ownership And Copy-Semantics Runbook

Use this checklist for any exported type, persisted state, event payload, or helper that stores data beyond the current stack frame.

## Required Questions

For each field on the type or payload:

1. Is it a slice, map, pointer, function, channel, or interface that may hold one of those?
2. Who owns it after this boundary: caller, callee, or both?
3. If the value crosses a store/export boundary, what must be cloned?
4. Does `nil` carry different meaning from empty, and must that distinction survive cloning?
5. Is there one shared helper or `Clone()` method enforcing the rule, or are callers re-implementing copies ad hoc?

## Boundary Checklist

- Inbound storage:
  - Clone caller-owned slices/maps/pointers before storing them on long-lived state.
  - Do not keep raw references from request structs, registry inputs, or provider responses unless the ownership contract explicitly allows sharing.
- Internal snapshots:
  - If a snapshot helper reads state that may later be exported, use the same clone helper as the public export path.
  - Avoid `append([]T(nil), src...)` for structs that contain reference-typed fields.
- Outbound export:
  - Any returned slice/map must be safe for the caller to mutate without changing internal state.
  - Prefer `Type.Clone()` plus `copy<TypePlural>()` helpers over scattered field-by-field copies.
- Persistence and queues:
  - Treat conversation stores, rollout writers, and async fanout as untrusted boundaries.
  - Clone before enqueueing or persisting so later mutations cannot rewrite history.
- Event payloads:
  - Deep-clone payload maps before enrichment, redaction, or fanout.
  - Convert structs with pointer fields into scalar maps when that is the safest way to break aliases.

## Current Harness Rules

- `Message.Clone()` owns `ToolCalls` copying.
- `ToolDefinition.Clone()` owns schema-map copying.
- `copyMessages(...)` is the single path for runner message snapshots and exports.
- `deepClonePayload(...)` is the single path for event payload isolation.

## Review Smells

- `append([]T(nil), values...)` on a struct type that contains slices, maps, or pointers.
- Returning a stored struct that still points at caller-provided schema or config maps.
- Clone helpers that silently turn non-nil empty slices/maps into `nil` without a deliberate contract.
- Repeated custom copy logic instead of a single type-level clone method.
