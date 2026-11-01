# Validation: EntryContent block command dialog

Recorded on 2026-11-01 by feature 042 (`specs/042-facility-recheck-fixes`, task T026). The dialog shipped with this feature in #54, but tasks T026 to T030 were never checked off. This file records the check that closed them.

## Code checked

| Task | Where it lives |
|---|---|
| T026 | Five block dialog journeys in `tests/browser/state-changing-command-authoring.spec.mjs` |
| T027 | The `entryBlockCommandMode`, `#entryBlockExistingCommand`, `#entryBlockCompletedText`, `#entryBlockNew*` and `data-entry-block-command-action` controls in `frontend/overseer/src/index.html` |
| T028 | The dialog, conditional field, disabled owner and focus styles in `frontend/overseer/src/overseer.css` |
| T029 | The configure-command action, isolated drafts, catalogs, locks and single-save mutations in `frontend/overseer/src/overseer.js` |

## Requirements against journeys

| Requirement | Journey |
|---|---|
| FR-023: assign an eligible same-terminal command, or create one, and capture the completed text | `block dialog assigns an eligible existing command with empty text and cancels without saving`, `block dialog creates one complete command in the selected same-terminal folder` |
| FR-024: a new command goes only into the chosen same-terminal folder | `block dialog creates one complete command in the selected same-terminal folder` |
| FR-025: reassigning an uncompleted owner is atomic, and a completed owner is locked until reset | `block dialog atomically reassigns and removes an uncompleted owner`, `block dialog keeps a completed owner read-only until reset` |
| FR-026: removing an uncompleted assignment removes only that change in one save, and a completed one is locked | `block dialog atomically reassigns and removes an uncompleted owner`, `block dialog keeps a completed owner read-only until reset` |
| SC-009: one save per assign, create, reassign and remove; no save on cancel or a completed-owner attempt | The same four journeys count `SaveAuthoring` calls before and after each action. `block dialog rejects a block reference that a rejected save removed, without saving` covers a stale reference |

## Runs

| Check | Command | Result |
|---|---|---|
| Overseer production build | `task frontend:build` | Built |
| Authoring journeys, including the original state-changing command suite | `npx playwright test state-changing-command-authoring.spec.mjs` from `tests/browser` | 24 passed |

Both runs were made on branch `042-facility-recheck-fixes`, from `develop` at `d50efcb`. They do not change the player or persistence contracts: the block dialog only writes the existing `stateChange.entryContentChange` field through the existing autosave.
