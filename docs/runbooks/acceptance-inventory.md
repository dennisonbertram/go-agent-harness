# Acceptance Inventory Runbook

Status: v2 implemented and final full-regression verified under Issue #1086;
promotion gates remain. This runbook describes the inventory/evidence contract, not a completed
all-surface acceptance result.

## Purpose

The acceptance inventory is compiled from two production boundaries:

- the condition-resolved `harness.Registry` / running `GET /v1/tools` catalog;
- `tui.NewCommandRegistry()` for built-in TUI commands and aliases.

It has no hand-maintained tool or command list. The canonical item identity is
`tool:<name>` or `tui_command:<name>`. An alias remains metadata on its one
canonical command item, but canonical and alias spellings compile into distinct
required invocation variants such as `tui_command:resume/canonical` and
`tui_command:resume/alias:continue`; one PTY result cannot satisfy both.

## Generate a live report

Start a harnessd instance with an isolated workspace, store, and safe provider
fixture. Then run:

```bash
go run ./cmd/acceptance-inventory -harness-url http://127.0.0.1:8080 > inventory.md
```

The report's schema version and SHA-256 inventory hash bind later evidence to
the exact daemon catalog and command registry. A non-200 `/v1/tools` response
is a failed inventory generation, not an empty catalog. Both resolver arrays
must be present and non-null; explicit `[]` means resolution completed with no
unavailable configured provider, while omission or `null` is an incompatible,
fail-open daemon response and is rejected.

## Conditions and applicability

An available item is copied from the resolved runtime registry. A dynamic item
that cannot be resolved (for example an unconnected MCP server) is returned by
the live `/v1/tools` boundary as paired configured and observed `toolset`
`not-applicable` records with its owner,
capability condition, stable reason, and resolver provenance (source and
provider identity). The compiler rejects a configured unavailable toolset that
lacks this observation, so it cannot be silently omitted. Individual tool rows
are permitted only when the resolver actually observed those names; unknown
provider catalogs must never be fabricated into guessed tool names. Conditions
originate at runtime resolution; a later runner must preserve their
source/provenance rather than inventing a parallel static list.

Tool provenance is attached at the same conditional registration branch that
adds the tool. Core, deferred, script, recipe, provider, and other conditional
groups therefore carry their actual owner and enabling condition into the
Registry snapshot. MCP-discovered and runtime-connected tools also retain the
logical server name. Consumers must not infer ownership from tool names or
maintain a name-to-owner map; missing authoritative Registry provenance is a
compile error.

TUI command owner and condition likewise originate in each `CommandEntry`.
Built-ins are stamped by `builtinCommandEntries`, bundle and legacy plugin
loaders stamp their own provenance, and the compiler copies those fields. A
later registration that omits either field fails inventory compilation instead
of being mislabeled as a built-in command.

When MCP discovery fails but neither a typed resolver report nor configured
provider names identify the missing catalog, Registry marks resolution
incomplete. `/v1/tools` returns `503 toolset_resolution_incomplete`; operators
must repair provider discovery/identity rather than treating the catalog as
empty.

Registry-derived tool rows declare API and TUI applicability; built-in slash
commands declare TUI-only applicability. Native GUI applicability is a separate
reviewed suite overlay because registry presence does not prove a native UX.
Every resolved available item must be mapped to native `available` or
`not-applicable`, with non-empty source references and UX rationale. The suite
hash covers the complete overlay and the full inventory hash. Missing,
duplicate, unknown, or non-native mappings are invalid; a runner cannot choose
N/A on its own. Native-available mappings require a case, while explicit N/A
mappings reject one and render separately from registry-derived API/TUI rows.

Independent runners call `ValidateCasesForSurface` with the unmodified full
compiled inventory and their selected surface. The validator requires exactly
one mapping for every applicable item invocation (including every command
alias), rejects mappings for other surfaces, and never filters or recalculates
the inventory hash.

Runner-owned negative or synthetic behavior is declared separately through a
`SuiteContract`; it is not inserted into the registry inventory. Stable typed
scenario IDs (currently unknown-command and invalid-form contracts) are sorted
and hashed together with the full inventory hash. Every declared scenario for
the selected surface requires exactly one case. Undeclared scenario cases,
missing required negatives, duplicate IDs, and evidence carrying a different
suite hash fail validation. `RenderSuiteResultMarkdown` shows these in a
separate synthetic-scenario table.

## Evidence contract

Each v2 record carries the schema version, full inventory hash, optional suite
hash, item or scenario identity, invocation/surface identity, evidence class,
ordered actions, typed artifact references, cleanup, timing, and failure class
when failed. Conversation-class passes require run, conversation, and event
IDs. Local TUI-command/scenario passes require none and reject supplied runtime
IDs, so `/help`, `/cost`, `/config`, `/clear`, `/quit`, and `/doctor` need no
fabricated run. A passing record also carries:

- every typed expected postcondition contract from its case;
- one separately observed, verified probe for every assertion ID, matching its
  kind and probe (rendered screen, durable state, conversation state, or
  external state);
- exactly the case's ordered actions and verified cleanup.

Artifact references contain a supported kind, path, `sha256:<64 hex>` digest,
and an explicit `redacted: true|false` declaration. A bare path, missing digest,
or omitted redaction declaration is not valid evidence.

A completed tool event, HTTP 200, screenshot, or assistant narration alone is
not pass evidence. The observed probe value must be non-empty and externally
verified. `RenderResultMarkdown` validates each supplied record against its
case and compiled item/surface before representing pass, fail, not-applicable,
or pending in the operator report.

A native GUI pass has a stricter minimum proof bundle: typed screenshot,
accessibility snapshot, raw SSE/event capture, and API/store probe artifacts.
Its environment metadata must identify a valid build SHA, absolute `.app`
bundle path, daemon PID and port, and an absolute isolated workspace path with
isolation explicitly asserted. Missing any one of these fields or artifact
kinds makes the claimed pass invalid.

## Scope boundaries

This slice does not execute tool cases or claim API, PTY, or rendered-native
success. #1087 owns API/SSE execution, #1088 PTY command execution, #1089
rendered macOS applicability, #1090 bounded orchestration, and #1010 remains
the independent cron/callback convergence gate.

## API/SSE executor status (Issue #1201 foundation, in implementation)

`internal/acceptance/apisserunner` preflights an API plan with
`ValidateCasesForSurface`, so a registry-derived available API item missing an
intent case fails before a request is sent. A plan starts a normal run through
`POST /v1/runs`, captures its raw per-run SSE stream and event IDs, reads the
terminal run state independently, then invokes a fixture-owned state probe and
cleanup. The resulting record is accepted only through `ValidateEvidence`.

The executor deliberately does not manufacture safe arguments or conditions
from tool names. Every default-registry tool still requires a reviewed,
fixture-safe intent plan (or a runtime resolver-backed N/A row); a generic
tool-call event is not coverage. The current real-daemon fixture proves the
create-profile durable-state/cleanup path only, not complete default-registry
coverage. Do not report an all-tool API result until the full live plan has
passed against the daemon's current inventory hash. This foundation PR closes
#1201 only; parent #1087 still requires complete positive intent coverage.

The suite also has a universal negative lane. It loads the same live catalog,
creates one request per available API item with that item in `denied_tools`,
and requires `tool_denied_for_run` in the raw SSE plus an unchanged isolated
workspace probe. This is rejection/no-mutation evidence only: it must be
rendered separately from the positive intent case, and never upgrades a tool
to an intent pass. On the initial fixture catalog it covered 63 API items at
`8cafb764383f2ef97a0b863cf2da0e395e4e880d1cb38494c64e3cbe190bc011`; the
count/hash are runtime evidence, not a stable manifest constant.

## Rollback

The inventory is additive and owns no product state. Revert Issue #1086 if it
breaks report generation or existing consumers; do not weaken a completeness or
postcondition assertion to make a failing execution lane appear green.
