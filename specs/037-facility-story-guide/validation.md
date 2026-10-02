# Validation: Facility Story Guide

**Date**: 2026-09-05
**Host**: macOS arm64, Node.js v26.8.1 from `.nvmrc`

## Verified behavior

- An empty facility can be configured through the guide as a door with closed/open states, an opening transition, a terminal command, and a block reading «Путь свободен» when open. Form applications and guide navigation perform no durable write; one explicit save persists the entire scene. Preview shows the result without publication.
- Missing devices, transitions, terminal commands, or display destinations have explanatory routes. Selected-device changes, newly applied devices, and discard update the guide context. Counts describe configured data; optional faults/programs do not create false readiness errors.
- A recovery program can be created, selected as a command action, reopened for editing, and saved without changing other programs. The existing generated private authoring contract accepts the existing recovery-program reference shape; no new serialized field is introduced.
- Relationship groups explain actions versus player information. Contextual form instructions explain states, transitions, prerequisites, faults, recovery, and binding types. Recovery instructions explicitly explain that all prerequisites must hold before the atomic action.
- Keyboard step selection/collapse works. Apply and Cancel from guide-launched forms restore focus to the current guide heading. The guide lives in the scrollable detail panel and its step navigation stays visible while scrolling its text.

## Commands and results

| Check | Result |
|---|---|
| `node --check frontend/overseer/src/overseer.js` | PASS |
| `node --check tests/browser/facility-authoring-ux.spec.mjs` | PASS |
| `git diff --check` | PASS |
| `task browser:test -- facility-authoring-ux.spec.mjs facility-authoring.spec.mjs --reporter=line` | PASS — 26/26, including the final guide focus-return correction |
| `task browser:test -- facility-authoring-ux.spec.mjs facility-authoring.spec.mjs facility-diagnostics.spec.mjs facility-lifecycle.spec.mjs facility-player-state.spec.mjs demo-session.spec.mjs --reporter=line` | PASS — 53/53; the subsequent focus-only correction was covered by the 26-test authoring rerun |
| `task package` | PASS — includes locked dependencies, protobuf checks, generated bindings, both production frontend builds, macOS compilation and app signing |

An initial recovery test assumed a free command in a fixture where all commands already had actions. It was corrected to exercise converting an existing command to a program and reopening the selection. This was a test setup error, not a remaining product failure.

The package pipeline emitted Node's experimental localStorage warning while checking generated contracts; those checks and production builds succeeded. The built native app was not launched for an interactive smoke test in this pass. Repository-wide Go checks were already passing in feature 036; no Go source changed here.

## Visual inspection

Inspected actual `sessions/demo.json` at 1440×900 and 800×700. Desktop retains the device list alongside the open guide and selected-device controls. Narrow layout stacks the list and guide, wraps actions, scrolls to the guide actions, and has no horizontal page overflow. The guide can be collapsed for operation during play.

Screenshots are regenerated under ignored `tests/browser/test-results/`:

- `facility-story-guide-desktop.png` — the action step and its contextual actions.
- `facility-story-guide-narrow.png` — player-information instructions and reachable previous/next controls, with sticky step navigation.
- `facility-authoring-ux-demo-desktop.png` and `facility-authoring-ux-demo-narrow.png` — full-demo entry views.

## Review and coverage

FR-001–004 and FR-007: empty door journey, contextual/discard journey, and missing-destination journey. FR-005–006: grouped relationship assertions and program-to-command journey. FR-008: keyboard journey and actual-demo responsive capture. SC-001–004 are covered by these journeys, visual inspection, and the package build.

Integration review retained one facility draft, existing dialog editors, one explicit persistence boundary, and no stored guide-completion flags. The guide's new dialog return-focus behavior is local to guide-launched forms. Constitution I–IX remain satisfied through existing private desktop contracts, server authority, unchanged schemas and persistence, preserved player behavior/audit boundary, and updated private demo instructions.

## Convergence follow-up — T005

- Missing-command and missing-display-destination routes now explain terminal autosave and offer an explicit **save draft and open Terminals** checkpoint. Failed saves retain the current step, selected device, and draft; only successful saves navigate. The same boundary protects the final step's terminal shortcut.
- Both new browser journeys stage a door and transition, reject a checkpoint, retry once, create a command or entry through the terminal UI, return to the same device, bind the new content, and save without losing the device graph. The command includes its required success text; the entry includes an authored text block.
- The desktop browser fixture now exercises terminal autosave through the existing authoritative facility save endpoint and publishes its canonical revision/session, instead of returning a no-op success. This is test-only; production transport and persistence are unchanged.
- `task browser:test -- facility-authoring-ux.spec.mjs facility-authoring.spec.mjs --reporter=line`: **PASS, 28/28**. Initial new-test failures were corrected missing form inputs (device type and command success text), not waived assertions. Existing stale-draft, failure/retry, preview, and responsive journeys also passed.
- JavaScript syntax checks for the Overseer, UX tests, and desktop fixture, plus `git diff --check`: **PASS**. Browser tooling emitted the benign `NO_COLOR`/`FORCE_COLOR` precedence warning.
- Simplification review of this follow-up kept one shared terminal-route helper, the existing save/conflict guard, and the existing draft store; no additional refactor was needed.
- `task frontend:build`: **PASS**, both production frontends under Node.js v26.8.1.

## Convergence follow-up — T006

- Step 4 now explains that a recovery program needs existing transitions, offers **ДОБАВИТЬ ПЕРЕХОД ДЛЯ ПРОГРАММЫ** for the selected device, and explicitly permits skipping optional recovery. Applying a transition replaces the prerequisite guidance with program creation.
- The browser journey jumps straight to step 4 with a transitionless door, uses the prerequisite action by keyboard, applies a transition, returns focus to the guide, and creates a program for that same door. The canonical fixture remains byte-for-byte equivalent in its reported state: no durable writes occurred.
- `task browser:test -- facility-authoring-ux.spec.mjs --grep 'optional recovery prerequisite|saves a staged story' --reporter=line`: **PASS, 3/3**. JavaScript syntax checks and `git diff --check`: **PASS**.
- Inspected new captures: `facility-story-checkpoint-command.png` at 1440×900, `facility-story-checkpoint-entry.png` at 800×700, and `facility-story-recovery-prerequisite-narrow.png` at 800×700. Explanations and long action labels wrap; navigation and prerequisite actions remain reachable with no horizontal overflow. Images are regenerated under ignored `tests/browser/test-results/`.

## Final convergence validation

- `task browser:test -- facility-authoring-ux.spec.mjs facility-authoring.spec.mjs facility-diagnostics.spec.mjs facility-lifecycle.spec.mjs facility-player-state.spec.mjs demo-session.spec.mjs --reporter=line`: **PASS, 56/56** after both fixes.
- `task package`: **PASS** after both fixes, including generated-contract checks, Wails bindings, both production frontends, macOS arm64 compilation, and signing. Rebuilt `build/bin/Fallout Terminal.app`; no interactive native smoke was performed. The generated-contract checks emitted the same benign Node experimental localStorage warning recorded above.
- Both convergence tasks are complete. No production Go, schema, transport, dependency, or persistence-model changes were needed; unrelated existing working-tree changes remain intact.
