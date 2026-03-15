# Issue #7 Grooming: Persistence layer: move run state from in-memory to database

## Summary
Runs are lost on restart, there's no history, and the system can't scale. Move run/conversation/event state to a database (SQLite/Postgres).

## Evaluation
- **Clarity**: Clear — the problem is well-articulated (runs lost on restart, no history, no scaling)
- **Acceptance Criteria**: Present — explicit schema design, interface definition, and new API endpoints specified
- **Scope**: Too broad — combines run persistence, conversation management, event storage, and migration of observational memory
- **Blockers**: None, but pairs well with #9 (auth) for multi-tenant isolation
- **Effort**: large — requires schema design, store interface implementation, SQLite/Postgres backends, refactoring server/runner to use store, migration testing

## Recommended Labels
needs-clarification, large

## Missing Clarifications
1. Should observational memory migrate to the same store, or remain separate?
2. How should event batching/buffering work in the hot path (runner loop)?
3. What's the retention policy for old conversations (auto-cleanup)?
4. Should split into stages: (1) core persistence, (2) conversation endpoints, (3) observational memory migration

## Notes
- Issue should be scoped/split before starting work
- Should await #9 (auth) for tenant design if multi-tenancy is needed
- The proposed interface is sound but needs detail on message write batching
