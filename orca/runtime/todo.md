# Runtime (`orca/runtime`) — architecture fix backlog

Prioritized phased plan from runtime review. Goal: fix correctness and liveness first, then cleanups.

## Phase 1 — Execution lifecycle source-of-truth

- [x] **Define execution lifecycle contract in DB layer**
  - Add explicit API for run registration/completion (or document lazy creation semantics).
  - Ensure `GetRunningExecutionIDs()` reflects real active executions.
- [x] **Support dynamic execution creation**
  - On first inbound event for unknown `run_id`, load/create `Execution`, register it, and start its loop.
  - Keep in-memory execution map synchronized with DB lifecycle.
- [x] **Tests (table-driven)**
  - Unknown `run_id` webhook creates execution and gets non-error response.
  - Restart path reloads active executions from DB and continues processing.

## Phase 2 — Deadlock fix in `DispatchWebhook`

- [x] **Remove blocking self-enqueue while holding `GraphState.mu`**
  - Do not send to `eventInbox` under lock.
  - Ensure response callback is not delayed by queue backpressure.
- [x] **Define queue failure policy**
  - If enqueue fails/times out, persist error state and respond deterministically.
- [x] **Tests (table-driven)**
  - Saturated inbox does not deadlock execution.
  - Callback still returns under backpressure/failure path.

## Phase 3 — End-to-end backpressure and routing policy

- [x] **Make `ag.events` and `execution.eventInbox` routing non-blocking-safe**
  - Avoid main-loop stall when one execution is saturated.
  - Add timeout/cancellation behavior and structured logging for dropped/retried events.
- [x] **Webhook ingress behavior**
  - If runtime is overloaded/shutting down, return explicit error payload (no hanging client).
- [x] **Tests (table-driven)**
  - One saturated run does not block unrelated run events.
  - Shutdown route sends deterministic response.

## Phase 4 — `InMemoryDB` concurrency safety

- [x] **Add synchronization for maps**
  - Protect `events` and `executions` with `sync.RWMutex` (or single-threaded dispatcher).
- [x] **Tests**
  - Concurrent `AddEvent`/`GetEvents` race-safe under `-race`.

## Phase 5 — Replay/durability consistency

- [x] **Persist scheduling intent**
  - Ensure webhook -> `run_node` continuation survives restart.
  - Use outbox-style persisted transition or replay reconciliation for `webhook_dispatched`.
- [x] **Tests (table-driven)**
  - Crash/restart between webhook persist and node run still progresses correctly.

## Follow-up (after critical phases)

- [ ] **Unify transition logic**
  - Replace split replay/live transition rules with shared transition function to avoid drift.
- [ ] **Webhook path collisions**
  - Detect duplicate normalized routes at startup and fail fast.
- [ ] **`orca start` workflow selection**
  - Support explicit workflow selection flag or hard error on ambiguity.
- [ ] **`cmd/start` HTTP listener testability**
  - Accept injected mux/handler instead of global default mux.
- [ ] **`runtime/llmclient` tests**
  - Add tests once API is stable.
- [ ] **Materialization failure policy**
  - Fail startup (or analyzer) when required workflow node cannot be materialized.
- [ ] **Shared test helper consolidation**
  - Move duplicate runtime test helpers to common testutil if duplication grows.
