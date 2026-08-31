# T080 terminal action/editor/settings evidence

Date: 2026-08-31

Per-row terminal action menus and the selected-terminal header/settings workflow are now Vue-owned. Legacy code retains only detached domain mutation/privileged command dispatch for the later T083 authoring join; migrated DOM queries, renderers, listeners, dynamic rename/reset nodes, and publish timeout ownership are absent.

Validation:

- `task frontend:typecheck:overseer` — passed.
- `frontend/node_modules/.bin/tsc -p frontend/overseer/tsconfig.legacy.json --noEmit` — passed.
- Candidate build — passed with 90 modules transformed.
- Focused assertion `terminal action menu editor and settings preserve validation and atomic save` — passed, 1 test.
- Existing selected-terminal/broadcast-context assertion — passed.
- Existing live-content publish assertion — passed.
- Existing terminal/group menu accessibility assertion — passed at both governed desktop viewports.
- Existing terminal move/reorder zero-mutation assertion — passed.
- Existing individual/terminal reset assertion — passed.

The focused assertion proves outside-click menu closure, blank rename rejection, Escape cancellation and trigger focus, one accepted rename save, settings drafts with zero pre-Apply saves, one atomic accepted settings save, stale editor/row request suppression, stable canonical presentation, repeated-unmount cleanup, late-projection rejection, and empty Teleport targets after teardown.
