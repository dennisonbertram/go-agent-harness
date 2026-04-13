# Documentation Maintenance Runbook

## Policy

Every documentation folder must contain an `INDEX.md` listing files with short descriptions.

Documentation status must stay aligned with implementation status:

- plan docs define intent and scope
- spec docs define the exact contract and current feature status
- implementation notes are written only after code lands
- public docs describe implemented behavior only

## When to Update Indexes

- Immediately after adding a new file.
- When file meaning changes materially.
- When files are moved or removed.

## Update Process

1. Update local folder `INDEX.md`.
2. Update `docs/INDEX.md` if top-level structure changed.
3. Ensure descriptions are concise and accurate.
4. Record major doc architecture changes in `docs/logs/engineering-log.md`.
5. If a feature moved between `planned`, `in implementation`, `implemented`, `deferred`, or `rejected`, update the owning spec and any umbrella plan ledger in the same change.
6. If the code behavior diverges from the current spec, stop and update the spec before continuing.
7. Do not add routes, config keys, or feature descriptions to public docs until they exist and are test-covered.
