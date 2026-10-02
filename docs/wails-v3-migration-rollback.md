# Historical Wails v2-to-v3 Migration and Cutover Record

This file is historical evidence only. It records the bounded Wails v2-to-v3
migration completed by feature 006 and does not provide current setup,
acceptance, rollback, or release instructions. The accepted production runtime
is the exactly pinned Wails v3.0.0-beta.15 runtime documented in the project
constitution and active dependency graphs; Wails v2 is not a production
fallback.

During feature 006, the pre-migration Wails v2 source served as the temporary
fallback until every required personal-use parity, package, and rollback gate
passed. The 60-minute local and 30-minute authenticated-ngrok soak workloads
were removed from acceptance by USER-SOAK-DECISION on 2026-08-15; completed
soak results below remain historical and non-gating. The Electron-to-Wails
record in `docs/wails-migration-rollback.md`, the completed migration artifacts
in `specs/006-wails-v3-migration/`, and the earlier Wails v2 artifacts in
`specs/001-wails-v2-migration/` are all historical evidence rather than active
operating instructions or rollback authority.

## Historical Ownership and Expiry

| Field | Value |
| --- | --- |
| Temporary coexistence owner | Feature-006 implementer; repository owner approves cutover |
| Coexistence scope | `006-wails-v3-migration` branch only |
| Expiry | Final feature-006 cutover, immediately after all required gates pass |
| Permanent v2/v3 switch | Prohibited |

## Historical Wails v2 Source Rollback Reference

| Field | Value |
| --- | --- |
| Source commit | `f1084b3df8b5630862bdf7a0f347b599156653ef` |
| Source verification | `PASS` — `git cat-file -t f1084b3df8b5630862bdf7a0f347b599156653ef` returned `commit`; `git show -s` identified `feat: Migrate player protocol to ConnectRPC and Protobuf (#8)` on 2026-08-13 |
| Application runtime | Wails v2.13.0 accepted pre-migration baseline |
| Session compatibility | session-v1; no conversion permitted or required |
| Player configuration compatibility | player-config-v1; no conversion permitted or required |

During the bounded feature-006 migration, this source commit was the canonical
rollback reference unless a Wails v2 application was genuinely built, manually
accepted, and recorded below. The record intentionally contains only artifact
digests that were actually collected.

## Historical Optional Accepted Wails v2 Artifact

| Field | Value |
| --- | --- |
| Artifact status | `BUILT FOR DRILL — ACCEPTED FOR THIS DRILL` |
| Artifact path | `/private/tmp/fallout-t066-v2-20260814/build/bin/Fallout Terminal.app` (temporary, non-canonical) |
| Executable SHA-256 | `c1faf7fe4f2ed0abc5c4814b8e71805f5b57a65b817fd3a45bbcc90bdaf29530` |
| Build provenance | Clean isolated clone at `f1084b3df8b5630862bdf7a0f347b599156653ef`; exact `go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build -clean -platform darwin/arm64` |
| Acceptance result | `PASS` |
| Acceptance evidence | Built, packaged, and ad-hoc signed successfully. A synchronized corrected rerun served four distinct players, observed the typed `Guess` RPC, accepted the master hack-success override, completed terminal navigation/back and reconnect with retained identity, reopened the saved version-1 safety copy, and released its process/listener cleanly. The temporary artifact passed this historical drill; during feature 006, the immutable source commit remained the rollback reference |

## Historical Candidate and Cutover Identity

These fields were recorded only after the v2-free source, active operating
documentation, pins, and locks were committed and the working tree was clean.
Evidence-only commits record the frozen candidate but do not redefine it.

| Field | Value |
| --- | --- |
| Build candidate commit | `02582af74b1ea63faa040ffe7e79a96f16d154d6` |
| Wails Go/runtime/CLI pin | `github.com/wailsapp/wails/v3 v3.0.0-beta.8`; isolated `wails3` tool at the same parent-module version |
| Frontend runtime/plugin pin | `@wailsio/runtime` `3.0.0-beta.8`, including `@wailsio/runtime/plugins/vite` |
| Personal-use app path | `build/bin/Fallout Terminal.app` |
| Canonical bundle-manifest SHA-256 | `7672ac4679ddb9dbfe0d1e93118f3f15250117caa59040ef6cb8fc6965365a57` |
| Cutover result | `PASS` |
| Cutover timestamp/environment | Candidate frozen 2026-08-15T00:16:45+04:00 on macOS arm64; final matrix passed, with both long-duration soak workloads removed from acceptance by USER-SOAK-DECISION |

The pre-removal qualification worktree was branch `006-wails-v3-migration`,
based on commit `bcb207704657a92f9902f4ac04ef11765b18f031`. Its migration changes and
evidence were intentionally uncommitted during the rollback drill and soak, so
that base commit remains provenance only—not the build candidate and not an
accepted artifact identity. T069 initially froze a v2-free candidate; after
BUG-001 and the package-order test correction,
`02582af74b1ea63faa040ffe7e79a96f16d154d6` became the immutable runtime build
candidate. The later USER-SOAK-DECISION changes acceptance artifacts only and
does not redefine that runtime identity.

## Historical Rollback Triggers

The following table records the stop conditions and actions that governed the
temporary migration window. It is not a current rollback policy.

| Trigger | Required action | Decision owner |
| --- | --- | --- |
| Session corruption, loss, incompatible round trip, or an older save replacing the newest accepted revision | Stop immediately; preserve originals and safety copies; return to canonical v2 source | Repository owner |
| Private bridge capability, inventory, payload, validation, cancellation, or redaction drift | Halt cutover; retain evidence; restore canonical v2 | Feature implementer, then repository owner |
| Master/player visual, persistence, gameplay, authority, replay, reconnect, convergence, sound, or privacy regression | Halt the affected journey and cutover; restore v2 unless a migration-only correction passes the full matrix | Feature implementer |
| Startup/shutdown listener, stream, session-worker, tunnel, temporary-policy, or child-process leak | Stop candidate and clean only its exact owned resources; restore v2 | Feature implementer |
| Local/public access regression, credential exposure, or loss of local fallback after tunnel failure | Disable public mode, preserve redacted evidence, restore v2 | Repository owner |
| Missing master/player/binding/font/sound/icon/plist/entitlement/demo resource | Reject package and restore v2 | Feature implementer |
| Unhandled Wails v3 beta-runtime crash | Reject v3 candidate and restore v2 | Repository owner |
| Architecture, minimum-OS, integrity, signature, notarization, staple, DMG, or Gatekeeper failure for the selected profile | Reject only that profile; restore v2 for any required personal-use gate | Repository owner |

## Historical Data-Safe Rollback Drill Procedure

The accepted feature-006 drill performed the following sequence; it is retained
to explain the evidence below and must not be used as current operating
guidance:

1. The Wails v3 candidate was stopped, and port 3690, owned ngrok processes,
   and temporary policy material were verified gone within five seconds.
2. The candidate identity was recorded, and safety copies of the selected
   session-v1 and player-config-v1 files were made outside the repository.
3. SHA-256 values were recorded for the originals and safety copies without
   including credentials or user file contents in repository evidence.
4. A separate maintenance worktree or clone was created at
   `f1084b3df8b5630862bdf7a0f347b599156653ef` without overwriting the migration
   worktree or reverting unrelated work.
5. That source was built and run with its recorded Wails v2 toolchain, or the
   separately accepted artifact recorded above was used.
6. Only the safety-copy version-1 data was opened, without migration or
   conversion.
7. Representative master create/open/edit/save/reopen behavior and a
   four-player local selection/navigation/hacking/reconnect journey were
   exercised.
8. The application was quit, and port 3690 and every owned resource were
   verified released.
9. The actual result was recorded below; a missing prerequisite or unperformed
   journey was `NOT RUN`, never `PASS`.

## Historical Rollback Drill Evidence

| Field | Value |
| --- | --- |
| Candidate/source tested | Pre-removal feature-006 worktree based on `bcb207704657a92f9902f4ac04ef11765b18f031`; rollback source `f1084b3df8b5630862bdf7a0f347b599156653ef` |
| Safety-copy paths or redacted identifiers | `/private/tmp/fallout-t066-rerun-20260814/session-v1.json`; `/private/tmp/fallout-t066-rerun-20260814/player-config-v1.json` |
| Original/safety-copy SHA-256 values | Session original/copy before drill: `c15baf6195a2a07cb7ed7985693c21bc910ae83092656483c94861ba39692e9c`; player-config original/copy: `07d5d888c58d65c6a3b8769988081215e299bbdf5a206e64d5639e5a5eff06b7`; post-save session safety copy: `0c448064f81665c4d99f0ccf721270227027fc28cc163693a664ebcaa129a36f` |
| Wails v2 source or artifact used | Clean isolated clone at the then-canonical source commit; temporary executable digest `c1faf7fe4f2ed0abc5c4814b8e71805f5b57a65b817fd3a45bbcc90bdaf29530` (accepted for this historical drill; during feature 006, the immutable source commit remained the rollback reference) |
| Session-v1 open/save/reopen result | `PASS` — opened and saved as version 1 with the relative player-config association, quit cleanly, then reopened the exact safety-copy path with both expected terminals and the associated seven-character configuration visible; post-reopen SHA-256 remained `0c448064f81665c4d99f0ccf721270227027fc28cc163693a664ebcaa129a36f` |
| Player-config-v1 result | `PASS` — opened unchanged as version 1 with seven roster characters; post-drill SHA-256 remained identical |
| Representative local master/player result | `PASS` — one v2 app/listener served four distinct assigned players; the synchronized harness observed one typed `Guess` RPC, accepted the master's hack-success override, navigated into a terminal row and back, reconnected with the same retained player identity, and observed 40 typed sound-manifest requests |
| Post-quit cleanup result | `PASS` — no `Fallout Terminal` process or port-3690 listener remained on the immediate post-quit check |
| Overall drill result | `PASS` |
| Timestamp/environment/evidence | 2026-08-14, macOS arm64, Go 1.26.5, Node 26.7.0, Wails v2.13.0; harness `/private/tmp/fallout-t066-v2-drill.mjs` |

## Historical Qualification and Final Evidence

BUG-001 replacement build candidate: `02582af74b1ea63faa040ffe7e79a96f16d154d6`, committed 2026-08-15T00:16:45+04:00 with Wails Go/CLI/frontend runtime `3.0.0-beta.8`, Buf `1.72.0`, protobuf Go `1.36.11`, and Connect `1.20.0`. Candidate `896d208c8b99ece1db31d5daf52c5d8fbd6762bc` was abandoned after the full Go gate exposed test-only package-order drift, and the final sequence restarted at T070. Rows below that still name an earlier candidate are superseded pending their reopened final-matrix reruns.

| Gate | Candidate/artifact identity | Result | Evidence/reason |
| --- | --- | --- | --- |
| Historical pre-removal 60-minute local soak (non-gating) | Pre-removal feature-006 worktree based on `bcb207704657a92f9902f4ac04ef11765b18f031`; packaged app PID 63116 | `HISTORICAL PASS` | 60.67 minutes; seven players; 57 accepted operations; 28 rejected observer attempts; three reconnects; two durable version-1 save/reopen cycles; 90 convergence checks; RSS medians 117936/128432/121312 KiB; one listener; 0.014s cleanup; retained for history only |
| Historical pre-removal authenticated-ngrok soak (non-gating) | Same pre-removal worktree; app PID 2695; owned ngrok PID 2700; configured protected endpoint | `HISTORICAL PASS` | 32.01 minutes; seven players; unauthenticated HTTP 401; 20 accepted operations; 10 rejected observer attempts; two reconnects; 32 convergence checks; controlled tunnel loss preserved local HTTP 200; 0.020s final cleanup; temporary credential deleted |
| Final-candidate native journeys | Candidate `02582af74b1ea63faa040ffe7e79a96f16d154d6` | `PASS` | One native dev app/window/listener with project-owned identity/icon; unchanged version-1 session/config reopen; 4–7 players, 32 actions, three reconnects, 80 sound requests; handled interrupt cleanup |
| Final personal-use package/offline smoke | Canonical bundle `7672ac4679ddb9dbfe0d1e93118f3f15250117caa59040ef6cb8fc6965365a57` | `PASS` | Package verification and cutover scan passed; offline generated-Connect smoke made nine RPCs and zero external requests; hash unchanged after quit |
| Historical final-candidate 60-minute local soak (non-gating) | Earlier canonical bundle; app PID 46826 | `HISTORICAL PASS` | 60.67 minutes; seven players; 53 accepted/26 rejected actions; three reconnects; two durable save/reopens; 84 convergence checks; RSS medians 118640/131968/122128 KiB; clean quit; hash unchanged; no rerun required |
| Historical final authenticated-ngrok soak (non-gating) | Earlier canonical bundle; app PID 83847; owned ngrok PID 83853 | `HISTORICAL PASS` | 30.61 minutes; seven players; unauthenticated HTTP 401; 20 accepted/10 rejected actions; two reconnects; 32 convergence checks; controlled tunnel loss preserved local HTTP 200; credential deleted; hash unchanged; no rerun required |
| Developer ID/notary/staple/DMG/Gatekeeper | No release artifact produced | `NOT RUN` | `security find-identity` found 0 valid identities; release preflight stopped before build/sign/upload because `DEVELOPER_ID_APPLICATION` was unavailable |
| Final active-v2/cutover scan | Candidate `02582af74b1ea63faa040ffe7e79a96f16d154d6` plus the USER-SOAK-DECISION acceptance-artifact patch | `PASS` | No active v2, dual-runtime, floating-tool, generated-global, dependency, bundle, script, CI, or operating-document surface remained |

Feature 006 accepted Wails v3 only after every required non-conditional gate
was `PASS` against the frozen v2-free build candidate and its recorded
personal-use bundle identity. Conditional public gates could remain `NOT RUN`
only with an honest reason and could not serve as passing public-mode evidence.
