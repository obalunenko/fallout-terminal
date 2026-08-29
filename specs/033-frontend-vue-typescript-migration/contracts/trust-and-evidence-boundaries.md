# Trust, Runtime, and Evidence Boundaries

## Application dependency boundaries

```text
Overseer Vue components/composables
        ↓ typed DesktopPort only
frontend/overseer/src/adapters/desktop-api.ts
        ↓ generated-service alias + @wailsio/runtime
generator-owned Wails bindings / named events
        ↓ private Wails transport
existing Go adapters, validation, authorization, persistence, services

Player Vue components/composables
        ↓ typed Player transport/projection ports only
frontend/client/src/adapters/player-rpc.ts
        ↓ generated public TypeScript + ConnectRPC
existing public ConnectRPC handlers
        ↓ existing Go validation/authorization/state owners
canonical server-authoritative state
```

The Player graph must have no direct or indirect path to `frontend/overseer`, `@wailsio/runtime`, Wails bindings, native capabilities, filesystem APIs, privileged types, Overseer state, or a shared cross-boundary store. Capability-neutral compiler configuration is not application state and may be shared at the workspace root.

## Preserved Player RPC contract

| RPC | Cardinality | Preserved client responsibility |
|---|---|---|
| `Subscribe` | unary request → server stream | Include opaque recognition handle/client instance, require complete snapshot first, cancel replaced streams, reconnect after current fixed delay, reject stale generations/revisions. |
| `SelectCharacter` | unary | Send typed request, correlate `ActionResult`, and wait for applicable authoritative revision before considering canonical state complete. |
| `Navigate` | unary | Preserve command/navigation request validation, authority, pending-state exclusion, typed rejection, and authoritative convergence. |
| `Guess` | unary | Preserve target identity, controller authority, attempts/outcomes, typed rejection, and no optimistic canonical hack mutation. |
| `ActivatePattern` | unary | Preserve pattern identity, controller authority, typed rejection, and authoritative convergence. |
| `SetPresentation` | unary | Preserve functionally equivalent fallback for unsupported/failed client streaming and context-keyed transient presentation. |
| `PresentationUplink` | client stream → unary response | Preserve capability probe, open handshake, targeted ready/results through `Subscribe`, latest-value backpressure, cancellation, generation/request correlation, timeout/retry, and fallback. |
| `SoundManifest` | unary | Preserve exact categories, same-origin asset validation, allowed filenames/formats, and optional failure isolation. |

All paths remain `/fallout.terminal.player.v1.PlayerService/<RPC>`, binary Connect encoding remains enabled, and no public health, reflection, status, administration, native, or private method is added. Existing Go request-size/origin/authentication/authorization rules and Connect error mapping remain authoritative.

## Preserved state and lifecycle rules

- Every new physical `Subscribe` stream begins with one complete personalized snapshot; deltas before a snapshot are invalid.
- Applicable revisions strictly increase. Older snapshots, updates, action results, events, update/public-access snapshots, and stale async completions cannot replace newer state.
- Go remains the canonical owner of player role, controller authority, navigation, command state, hacking outcomes, terminal transitions, sessions, persistence, public access, and updates.
- One logical Player session is coordinated across qualifying tabs; each connected tab retains its own physical stream. Storage values are untrusted and recognition handles remain opaque.
- Controller-only local presentation is transient, context-keyed, non-authoritative, bounded, and reconciled with server updates; observers remain read-only.
- Client-streaming presentation is optional. Backpressure is latest-value, cancellation is explicit, and any unsupported or failed path returns to `SetPresentation` without losing basic operation.
- Overseer command/result and named-event adapters preserve listener-before-getter ordering, exact-once release, correlation/idempotency, tuple/revision comparisons, confirmation atomicity, and stale completion suppression.
- Session and player-configuration JSON retain their exact version-1 fields, defaults, compatible unknown-field behavior, and business meaning.

## External runtime validation

Generated decoding or Wails construction proves transport structure only. Trusted application state is created only after boundary validation.

| Boundary | Required validation |
|---|---|
| `localStorage` and storage events | Key, record shape, version/expiry, string IDs, safe integers, ownership token, malformed/denied storage fallback |
| Wails method results and named events | Non-null records, exact strings/booleans/safe integers, enum allowlists, optional fields, revision/generation applicability, error defaults, deep detachment |
| ConnectRPC decoded values | Semantic identifiers, unique roster entries, revision applicability, active broadcast/terminal/context match, typed action correlation, public-only meaning |
| DOM/form/pointer/keyboard input | Actual target kind, trimmed strings, ranges, IDs, modes, filler coordinate syntax, focusability, current authoritative context |
| Clipboard | Non-empty value, supported native function, promise outcome/failure; secrets remain transient and are cleared immediately |
| Sound manifests/assets | Exact category prefix, safe single filename, approved extension, same-origin URL, optional fetch/decode/play failure isolation |
| Presentation streaming | Secure-context capability, stream generation, open/ready/result correlation, response envelope/end-stream validity, cancellation, timeout, context applicability |

## Imperative Vue-owned seams and cleanup

Imperative code is limited to these narrow owners:

| Seam | Vue owner | Acquisition | Cleanup requirement |
|---|---|---|---|
| Web Audio | `useTerminalSound` | Gesture listeners, `AudioContext`, decoded buffers, `Audio`, BufferSource/Gain nodes, fetch controllers | Remove listeners; abort fetches; stop/disconnect nodes; pause/clear ambient audio; close context; suppress late promises |
| Focus | `useDialogFocus`, `useFocusRestoration`, or focused directives | Element refs, opener ref, focus key | Restore only to connected valid opener; cancel queued focus on unmount; no global descendant queries |
| Element measurement | `usePaginationMeasurement` | Element refs, `ResizeObserver`, font readiness, animation frame | Disconnect observers; cancel frames; ignore late font completion; restore any temporary content |
| CRT/typewriter timing | `useTypewriterReveal` | Timers, performance clock, reveal generation, key listeners | Clear timers; remove listeners; cancel/complete active generation once; prevent replay after unmount/context change |
| Hacking-board fitting | `useHackingBoardFit` | Owned hidden probe, measurements, observers/frames | Strip IDs/inert probe; always remove probe; disconnect/cancel; never mutate outside owned root |
| Pointer geometry | `useHackingPointer` | Owned cell refs, pointer/focus delegates, hover timer | Remove listeners; clear timers/local targets; preserve latest authoritative context |
| Presentation streaming | `usePresentationUplink` | AbortControllers, request stream/iterator, mailbox, ready/result/retry timers | Abort/cancel/return iterator; clear mailbox/timers; invalidate generation; ignore late completion; preserve unary fallback |

Subscriptions, document/window listeners, timers, animation frames, observers, audio resources, AbortControllers, streams, and temporary nodes are registered immediately with the owning Vue scope and released in `onScopeDispose`/`onUnmounted` as appropriate.

## Browser versus native evidence

| Evidence class | Producer | What it proves | What it must not claim |
|---|---|---|---|
| Browser functional/visual | Existing `tests/browser/*.mjs`, unchanged selectors/snapshots, Player production bundle, Overseer production SFCs with typed fake `DesktopPort` | DOM semantics, accessibility, copy, focus, keyboard/pointer, timing, geometry, audio observations, transport fixtures, revisions, application behavior | Real Wails binding generation, native dialogs/clipboard, embedded startup, packaged resources, secure store, matching-host runtime |
| Wails binding integrity | Pinned clean generator plus `scripts/wails-bindings-check.sh` | Exact generated JS/JSDoc/declarations, 39 methods, seven events, deterministic output, capability separation | Vue DOM behavior or packaged startup |
| Native/runtime | Task/buildtool, Wails startup/resource tests, native smoke scripts | Separate embeds, binding availability, window startup, native adapter behavior, resources, native accessibility where run | Complete browser visual parity unless the browser suite also ran |
| Package content | Matching-host `task package` and governed inspection scripts | Supported archive/executable/resource inventory and identity for that target | Signing/notarization, credentials, or unexecuted UI journeys |
| Conditional external/signing | Explicit opt-in/manual workflows | Only the real provider/signing/notarization behavior actually executed | A PASS when prerequisites were absent; absent checks are `NOT RUN` |

## Final forbidden-state boundary

Scan handwritten production application source under `frontend/client` and `frontend/overseer/src` for:

- `.js` application modules;
- legacy bootstrap names and script tags;
- candidate/staging entrypoints or temporary mount switches;
- mixed/cross-owned DOM queries and mutations;
- `allowJs` and `checkJs`;
- broad `any`;
- `@ts-nocheck`;
- blanket assertions and assertion chains used to bypass checking;
- unexplained `@ts-ignore`, `@ts-expect-error`, or lint/compiler suppressions.

Applicable scans exclude `frontend/overseer/bindings/**`, dependency directories, Vite/build/package output, and `tests/browser/*.mjs`. Exclusions must be path-exact and tested with positive and negative self-test fixtures so a forbidden authored file cannot hide behind a generated filename.
