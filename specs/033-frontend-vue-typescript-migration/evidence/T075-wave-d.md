# T075 Wave-d Exit Evidence

**Date**: 2026-08-31
**Result**: PASS

## Exclusive ownership scan

The governed migrated-ID absence scan passed across the remaining legacy/bootstrap owners:

```text
scripts/frontend-assert-no-match.sh \
  'applicationUpdate(StatusPanel|Dialog|RestartDialog)|commandExecutionDialog|terminalNavigationDialog|terminalSwitchDialog|resetConfirmationDialog|logicalSessionDialog|playerManagementDialog|playerDeleteDialog|terminalGroup(Draft|Impact)Dialog|publicAccess(Section|SettingsDialog|ProviderTokenDialog|PlayerCredentialsDialog)|generatedPasswordDialog' \
  frontend/overseer/src/overseer.js \
  frontend/overseer/src/desktop-api.js \
  frontend/overseer/src/main.ts \
  frontend/overseer/src/mount.ts
```

Result: the pattern is absent from all four readable files. The production document retains exactly two non-owning semantic references to migrated dialogs:

```text
#btnCreateTerminalGroup aria-controls="terminalGroupDraftDialog"
#btnManageLogicalSessions aria-controls="logicalSessionDialog"
```

Those legacy-owned trigger buttons send detached typed requests through the coexistence bridge; they do not query, render, focus, mutate, or bind inside either Vue dialog. All migrated markup and handlers live in their reviewed components/composables under the single `#overseerVueLeaves` application.

## Cleanup verification

The exact T075 command passed:

```text
npm test --prefix tests/browser -- overseer-dialog-focus.spec.mjs desktop-api.spec.mjs
```

Results: 15 passed. The suite proved connected-only focus restoration, strict desktop result validation, listener-before-getter ordering, monotonic event authority, exact-once release, hot-disposal cleanup, the 39-method/seven-event facade inventory, terminal-group detached authorities, and public-access/application-update ordering and disposal.

T074 additionally passed the full browser gate with 213 passes, two existing credential-gated real-ngrok skips, and no immutable CRT snapshot diff. The Vue leaf joins provide explicit first-unmount/repeated-unmount evidence for application-update, approval/reset, session/player/group, and public-access subscriptions, bridge listeners, focus work, secret drafts, and clipboard timers.

## Remaining legacy inventory

- `overseer.js`: start/runtime and server-link presentation; adjacent server/client/hack/session/coordination projections; typed coexistence dispatch; terminal/group model and action menus; terminal list, tree, authoring, and settings controls; broadcast, end-broadcast, take-off-air, and create-terminal dialogs; hacking controls.
- `desktop-api.js`: temporary validated 39-method/seven-event desktop facade, generated-binding adaptation, subscription registry, normalization/order guards, secret-mutation redaction, and hot disposal.
- Temporary mechanisms still open: `#legacyOverseerRoot`, `#overseerVueLeaves`, the typed callback bridge, both legacy scripts/tags, candidate document/entry, candidate Vite/Playwright selection, and the bounded two-file legacy compiler program.

No remaining handler owns a migrated leaf. All listed temporary mechanisms retain their registered Wave-e expiry and unconditional T090 removal task; Wave d introduces no additional mechanism.
