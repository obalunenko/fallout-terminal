# Validation: Facility Authoring UX

**Date**: 2026-09-05

**Platform**: macOS arm64
**Node.js**: v26.8.1

## Automated evidence

| Check | Result |
|---|---|
| `node --check frontend/overseer/src/overseer.js` | PASS |
| `git diff --check` | PASS |
| `task frontend:build` | PASS — Overseer and client production bundles |
| `task browser:test -- facility-authoring-ux.spec.mjs facility-authoring.spec.mjs --reporter=line` | PASS — 22/22 |
| `task browser:test -- facility-authoring-ux.spec.mjs facility-authoring.spec.mjs facility-diagnostics.spec.mjs facility-lifecycle.spec.mjs facility-player-state.spec.mjs demo-session.spec.mjs --reporter=line` | PASS — 49/49 |
| Actual-demo visual capture at 1440×900 and 800×700 | PASS — 1/1 using `sessions/demo.json`; screenshots preserved under ignored `tests/browser/test-results/` |
| `task check` | PASS — formatting, vet, lint (0 issues), race tests, protobuf checks/breaking fixtures, Wails bindings, SpecKit update tests |
| `task test` | PASS — repository Go test suite with the required macOS deployment/CGO flags |
| `task build` | PASS — produced `build/bin/Fallout Terminal`; Go emitted a non-fatal sandbox stat-cache warning after compilation |
| `task package` | PASS — rebuilt and signed the macOS application bundle |

No Go source file changed in this feature, so the pre-commit `go fix ./...` requirement is not applicable.

## Simplification review

- Removed obsolete immediate-persistence and async deletion paths; facility edits now share one draft boundary.
- Kept one binding catalog and one transition-summary path for workspace and command context.
- Preserved unknown and unedited entity attributes by overlaying represented form fields on cloned originals.
- Made preview generation explicit and ignored responses superseded by close or later selection changes.
- Hardened action validation for devices with no transitions and routed recovery-backed command actions to their program editor.

## Visual and requirement evidence

Visual comparison used the eight feature-035 screenshots in `/Users/olegbalunenko/Desktop/states_ux` as the baseline. The final responsive-layout pass loaded the actual `sessions/demo.json` through the authoring fixture: 10 devices, 7 conditions, 2 recovery programs, and 6 terminals.

| Surface | Result |
|---|---|
| Desktop workspace, 1440×900 | PASS — searchable device list and independently scrollable selected-device detail remain adjacent; current/initial state, affected terminals, scoped rules, and session-wide draft actions are visible without the old distant focus jump. |
| Narrow workspace, 800×700 | PASS — list and detail stack in source order, primary actions wrap, text remains readable, and `document.documentElement.scrollWidth <= window.innerWidth`. |
| Device graph dialog | PASS — source/destination transition summary is primary, prerequisite/effect counts are collapsed by default, technical IDs remain editable but secondary, and apply/cancel remain in a sticky footer. |
| Comparative preview dialog | PASS — entity and Online → Offline context, saved/no-player basis, changed count, unavailable-command count, changed node highlight, and refresh/close controls are all visible together. |
| Relationship navigation | PASS — readable block, command, and composite-transition rows open the owning editor and focus the referenced target; command properties expose transition and prerequisite context. |

The final inspected screenshots are `facility-authoring-ux-demo-desktop.png` and `facility-authoring-ux-demo-narrow.png` in ignored `tests/browser/test-results/`. Graph and preview dialogs were inspected during the earlier focused visual pass and remain covered by the browser suite.

## Final requirement coverage

| Requirement | Evidence | Result |
|---|---|---|
| FR-001–002 | Multi-transition save/reopen preservation plus parameterized focused validation for incomplete device, state, and condition references | PASS |
| FR-003–004 | Shared form staging/discard/save, failed-save retry, and newer-session stale-draft rejection/discard regressions | PASS |
| FR-005–007 | Exact desktop/narrow geometry, search, readable device status, and explicit guarded reset/recovery UI | PASS |
| FR-008 | Block, command/node, condition, recovery-program, and composite-transition navigation paths, including missing-target feedback | PASS |
| FR-009 | Binding and recovery-program edit/remove coverage with zero durable writes before session save | PASS |
| FR-010 | Facility-backed command summary and contextual action editor coverage | PASS |
| FR-011 | Readable transition summary, keyboard-expandable rules, explicit fault choices, and computed sticky footer | PASS |
| FR-012 | Automatic comparative preview, changed/unavailable markers, saved/no-player explanation, mutation check, and dirty-draft guard | PASS |
| FR-013 | Authored names in primary summaries with IDs retained in secondary disclosures and form controls | PASS |
| FR-014 | 22 focused and 49 affected browser journeys plus actual-demo exact-width visual inspection | PASS |

All six success criteria are satisfied. The final constitution recheck remains PASS for all nine principles: the change stays inside the private Overseer, uses existing server-authoritative contracts and revision checks, adds no schema/player surface/dependency, preserves portable session fields, completes the immediate-save cutover, and updates the demo workflow. No exception is required.
