# Implementation Plan: Facility Authoring UX

## Summary

Replace the long facility dashboard with a searchable device list and adjacent details, make every authoring form stage one explicit draft, and bring readable relationship editing into facility and command views. Fix reference selector corruption before the layout changes, then validate complete editing, discard/save, navigation, and detached preview journeys against the existing private contracts. Preserve unedited data by cloning existing entities and changing only represented fields.

## Project Structure

```text
frontend/overseer/src/
  overseer.js       # facility forms, draft lifecycle, summaries, navigation, preview
  index.html        # list/detail workspace and accessible controls
  overseer.css      # responsive panels, readable forms, sticky actions
tests/browser/
  facility-authoring.spec.mjs # existing journeys adapted to explicit draft copy
  facility-authoring-ux.spec.mjs # regressions and screenshot checks
  fixtures/facility-session.mjs # richer relationships if needed
sessions/demo-capability-paths.md # revised authoring paths
specs/036-facility-authoring-ux/
  spec.md, plan.md, research.md, data-model.md, contracts/overseer-ui.md
  tasks.md, validation.md
```

**Structure Decision**: Extend the existing Overseer and fixture seams without adding a framework, parallel store, or transport.

## Constitution Check

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern desktop runtime | PASS | Existing private desktop API only |
| II. Protobuf contracts | PASS | No new serialized fields or DTOs; local UI state only |
| III. Server authority | PASS | Preview remains server-resolved; save/reset/recovery use existing revision checks |
| IV. Private capabilities | PASS | All work in Overseer; no player endpoint changes |
| V. Schema evolution | PASS | No schema or generated-file edits |
| VI. Portable session | PASS | Clone and preserve untouched fields; regression save/reopen checks |
| VII. Complete cutovers | PASS | Remove old immediate device-save path and obsolete layout styles |
| VIII. Activity observability | PASS | No new player requests; existing mutation/audit path retained |
| IX. Demo coherence | PASS | Preserve demo narrative; update authoring route inventory and exercise demo |

The design has no architectural exception. Work branches from the required 035 implementation tip (three commits ahead of local develop); this dependent follow-up retains those prerequisite commits until integration.

## Verification

Use Node 26.8.1 from `.nvmrc` and Taskfile dependency/build/browser workflows. Add regressions before fixing reference preservation and expand browser checks as each story lands. Run the affected authoring, facility lifecycle/player/diagnostic, and demo journeys, both frontend production builds, repository vet/test/lint, native build/package where supported, and visual checks at 1440×900 and 800×700. No schema regeneration or new dependency research is needed. Preserve screenshots under an ignored test-output directory and record actual commands/results in validation.md. Re-review simplification and constitution compliance before completion.

## Implementation Order

Safe reference editing and explicit draft save → workspace layout/readable status → editable relationships and contextual command actions → detached comparative preview → regression/visual verification and docs.
