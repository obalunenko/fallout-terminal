# Refactoring Invariants

## Public live-state projection

For every projected content tree:

- Mutating a projected child slice, raw extension byte slice, state-change configuration, or terminal-transition configuration does not change the authoritative tree.
- Nil source pointers and slices remain nil in the projection.
- Existing effective command-state presentation remains unchanged.

## Command resolution failures

- A stale decision carries the stable stale identity through wrapping and maps to the existing safe stale message.
- A missing durable store, store failure, missing durable terminal, or missing durable completion evidence carries the stable persistence identity and maps to the existing safe persistence message.
- Other failures map to the existing generic resolution message.
- Application classification does not inspect `Error()` text.

## Tunnel lifecycle

- Waiting for another stop, cleanup, or reconfiguration releases the manager lock.
- Cancellation while waiting returns the existing safe timeout category and a detached snapshot.
- Matching concurrent reconfigurations join the existing result; a later revision retries ownership checks.
- Replacement secret buffers remain owned until the outer call completes and are cleared once.
- Retry count does not increase call-stack depth.

## Player-action transaction

- Exactly one coordinator commit owns validation and mutation for each dispatched action.
- Rejected preconditions do not invoke gameplay mutation.
- Cached request replay, duplicate detection, revision increments, effect ordering, and persistence behavior remain unchanged.
- Common terminal activation updates remain identical across direct and navigation-assisted routes; return-route metadata remains route-specific.

## Character and session boundaries

- Character value rules return consistent outcomes at application and control boundaries.
- Application-only presence checks remain before conversion into domain values.
- Create, open, and demo-copy session commands keep distinct operation telemetry and command callbacks while sharing orchestration.

## Extracted update validation

- Every existing malformed identity, package shape, path, mode, digest, duplicate, ordering, collision, and unsupported-file condition remains rejected.
- Validation remains cancellation-aware during filesystem inspection and hashing.
- No manifest or extracted path is trusted before its preceding validation stage succeeds.
