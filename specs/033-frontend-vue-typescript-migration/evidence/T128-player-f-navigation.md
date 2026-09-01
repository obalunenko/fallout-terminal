# T128 — Wave-f Player navigation/identity/action integration

Date: 2026-09-01
Host: macOS, local Playwright candidate server
Evidence class: browser candidate integration

## Command

```text
npm test --prefix tests/browser -- player-candidate-identity.spec.mjs player-candidate-navigation.spec.mjs player-candidate-pagination.spec.mjs player-candidate-session.spec.mjs
```

## Result

PASS — 9 tests in 3.3 seconds.

- Identity rejected malformed/duplicate roster data, mismatched and stale assignments, and malformed recognition handles without changing trusted state.
- Controller/observer authority stayed terminal-, broadcast-, revision-, and context-keyed; observer state remained read-only and could not publish controller feedback.
- Character, terminal-menu, record, footer, and pagination leaves preserved governed selectors, keyboard focus, typed events, and stable keys.
- SelectCharacter/Navigate results stayed correlated and pending until the authoritative revision converged; late work was cancelled without optimistic canonical mutation.
- Navigation rejected malformed modes and stale revisions, excluded duplicate pending work, and restored focus only to connected owners.
- Pagination preserved temporary DOM content/listeners and released observer, RAF, and late-font work.
- The integrated candidate owned one logical session and one physical stream and cascaded abort, iterator return, storage-listener release, and Vue unmount cleanup.

No production Player route or legacy owner was selected by this evidence.
