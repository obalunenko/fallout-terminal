# Feature Specification: Phase 1 Server-Authoritative Hacking Patterns

> **SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE.**
> Any WebSocket or handwritten JSON player-transport description in this retained
> completed feature document has been replaced by the generated ConnectRPC contract in
> [`specs/005-connectrpc-protobuf-migration/contracts/public-player.md`](../005-connectrpc-protobuf-migration/contracts/public-player.md).

**Bugfix**: 2026-08-11 — BUG-001 separated opening-symbol-only pattern activation from ordinary individual delimiter-symbol selection.

**Bugfix**: 2026-08-28 — BUG-002 made authoritative forced-success outcomes take precedence over stale hacking action and presentation rejections.

**Bugfix**: 2026-08-28 — BUG-003 clarified that BUG-002 reconciliation follows the ended hacking generation across presentation-context changes and requires verification through the real private Overseer control.

## Clarifications

### Session 2026-08-11

- Q: How are valid special-pattern outcomes selected and verified? → A: Each accepted activation independently uses server-side weighted randomness: 80% dud removal and 20% attempt restoration, with deterministic boundary tests rather than an exact production sample.
- Q: How is the initial 3–6 pattern requirement measured? → A: Run the final rendered board through the gameplay discovery algorithm before publication and regenerate until it contains 3–6 distinct selectable patterns.
- Q: What gives a dynamic pattern stable identity? → A: Puzzle generation, row, inclusive opening index, and inclusive closing index; used state belongs to that complete coordinate pair.
- Q: What must a player pattern request prove before it can mutate the puzzle? → A: It targets the active generation and a currently valid, unused coordinate pair while the puzzle is actionable, and validation precedes used-state marking, randomness, mutation, projection, and broadcast under the live-service lock.
- Q: Where do pattern state and trusted overrides live? → A: Pattern progress is runtime-only and reconnectable within one server process; `ForceHackSuccess` remains private to the desktop/Wails boundary and is never player-accessible.

## User Scenarios & Testing

### User Story 1 - Solve Without Player Cheats (Priority: P1)

As a player, I face the hacking puzzle using ordinary password guesses and server-authoritative special patterns only, so success follows the established game rules without player-accessible cheats.

**Why this priority**: Replacing the existing player shortcuts is the Phase 1 goal and establishes the trust boundary for every other story.

**Independent Test**: Start a hacking puzzle, try every previously available player shortcut and public interface, and confirm that none can expose or force the answer while ordinary password guessing and ~~non-delimiter filler~~ individual filler-symbol attempt rules remain unchanged. The struck wording was superseded by BUG-001 because delimiter glyphs also retain ordinary individual selection when they are not a current pattern's opening symbol.

**Acceptance Scenarios**:

1. **Given** an active puzzle, **When** a player enters the former administrator command, **Then** the puzzle state does not change and no incorrect passwords are removed.
2. **Given** a newly generated board, **When** the player reviews its selectable content, **Then** no player-selectable entry directly forces success or bulk-removes incorrect passwords.
3. **Given** an active puzzle, **When** the player selects a candidate password, **Then** the existing password-match, likeness, logging, success, failure, and attempt rules determine the result unchanged.
4. **Given** access only to the player browser and player protocol, **When** a player tries browser globals, DOM controls, keyboard shortcuts, query parameters, or protocol messages, **Then** `ForceHackSuccess` cannot be invoked.

### User Story 2 - Discover and Use Special Patterns (Priority: P1)

As a player, I can discover and activate Fallout-style delimiter patterns on the rendered board for a weighted chance to remove one incorrect password or restore my attempts.

**Why this priority**: Special patterns are the server-authoritative replacement for the removed player cheat.

**Independent Test**: Generate controlled boards containing each allowed delimiter type and exercise discovery, highlighting, request validation, deterministic weighted outcomes, fallback behavior, and one-use state.

**Acceptance Scenarios**:

1. **Given** a matching opening delimiter, **When** the first compatible closing delimiter to its right is on the same row and the intervening rendered characters contain no alphabetic character, **Then** the inclusive opening-through-closing coordinate range is a valid selectable pattern.
2. **Given** an unused valid pattern and at least one currently available incorrect password, **When** the deterministic random source selects dud removal, **Then** exactly one available incorrect password is replaced by periods and the correct password remains unchanged.
3. **Given** an unused valid pattern, **When** the deterministic random source selects attempt restoration, **Then** remaining attempts return to the configured maximum without exceeding it.
4. **Given** no removable incorrect password remains, **When** the deterministic random source selects dud removal for an accepted activation, **Then** attempts are restored instead.
5. **Given** a valid request for an unused pattern, **When** activation succeeds, **Then** that exact generation-and-coordinate identity becomes used and cannot produce another effect.
6. **Given** an unused valid pattern, **When** the player hovers, focuses, or selects its opening symbol at `start`, **Then** the whole inclusive `start`-through-`end` span is the pattern interaction, and selection sends one `HACK_PATTERN` request for that pattern.
7. **Given** an interior or closing symbol inside a valid pattern, **When** the player hovers, focuses, or selects that non-opening symbol, **Then** only that symbol receives ordinary filler interaction and it does not highlight or activate the enclosing pattern.

### User Story 3 - Discover Stacked and Dynamic Patterns (Priority: P1)

As a player, I can use overlapping patterns and patterns created by board mutation, so the selectable set always reflects the current canonical rendered board.

**Why this priority**: Shared closers and dynamically changed pairings are part of the defined discovery rule and identity model.

**Independent Test**: Exercise a board with multiple compatible openings sharing one closer, mutate it through dud removal, and verify identity, availability, rediscovery, and permanent used-pair history.

**Acceptance Scenarios**:

1. **Given** two compatible opening delimiters have the same first compatible closing delimiter on one row, **When** patterns are discovered, **Then** their different opening coordinates produce two distinct selectable identities.
2. **Given** one of two patterns sharing a closing delimiter has been used, **When** the other is selected, **Then** the other remains independently available.
3. **Given** board mutation causes an opening delimiter to pair with a different first compatible closing delimiter, **When** the board is rediscovered, **Then** the new coordinate pair is a new pattern that may be used once.
4. **Given** a coordinate pair was used earlier in the active puzzle generation, **When** later mutation makes that same pair valid again, **Then** it remains used and unavailable.
5. **Given** dud removal creates additional valid patterns after the first player action, **When** the current board is rediscovered, **Then** all valid new patterns are published even if the current count exceeds six.

### User Story 4 - Share One Atomic Puzzle State (Priority: P1)

As a group of players, we receive one canonical puzzle state, so concurrent, stale, invalid, and reconnecting requests cannot create divergent effects.

**Why this priority**: Special-pattern effects change shared secret-bearing state and therefore must be validated and applied atomically by the live service.

**Independent Test**: Connect multiple clients, submit duplicate and stale `HACK_PATTERN` requests concurrently, mutate returned projections, and reconnect a client while the same server process remains running.

**Acceptance Scenarios**:

1. **Given** concurrent requests for the same currently available pattern, **When** the server handles them, **Then** exactly one request is accepted, exactly one outcome is applied, and rejected duplicates consume no random value.
2. **Given** a delayed request from an older puzzle generation, **When** its coordinates happen to match a pattern in the active puzzle, **Then** the request is rejected without mutation or random-source advancement.
3. **Given** a missing, unknown, malformed, invalid, unavailable, used, stale, or non-actionable pattern request, **When** validation fails, **Then** canonical state and the random source remain unchanged.
4. **Given** a client mutates any returned public pattern projection, **When** canonical puzzle state is read again, **Then** canonical slices, maps, objects, identities, board contents, and used state are unchanged.
5. **Given** a client reconnects while the server process and puzzle remain active, **When** synchronization completes, **Then** it receives the current board, attempts, pattern availability, used identities, removed duds, and outcome.

### User Story 5 - Let the Game Master Resolve the Puzzle Privately (Priority: P1)

As a game master, I retain the existing trusted `ForceHackSuccess` control through the private desktop application so I can keep the table moving without exposing a player cheat.

**Why this priority**: Removing player shortcuts must preserve the established trusted recovery control and its privileged boundary.

**Independent Test**: Invoke `ForceHackSuccess` through the existing private desktop/Wails boundary, verify the shared success flow, and verify that every player-accessible surface lacks an equivalent invocation path. Repeat after an unsuccessful guess while a correlated hacking action or presentation result is delayed, and verify that the solved outcome remains unambiguous on the hacking surface and subsequent terminal menu. BUG-003 requires at least one replay through the real private Overseer control; a fixture-only force-success endpoint does not by itself verify this boundary or its production event ordering.

**Acceptance Scenarios**:

1. **Given** an active unsolved puzzle, **When** the game master invokes `ForceHackSuccess` through the existing private desktop/Wails boundary, **Then** the puzzle is solved without consuming an attempt.
2. **Given** the trusted action succeeds, **When** state is broadcast, **Then** all connected players transition through the existing success flow.
3. **Given** there is no eligible active puzzle, **When** the game master views the hacking controls, **Then** the trusted solve control is unavailable and cannot alter state.
4. **Given** a player WebSocket connection or browser context, **When** any player-accessible input is attempted, **Then** no `ForceHackSuccess` operation is exposed or accepted.
5. **Given** an unsuccessful guess has left a player action or hacking-presentation result in flight, **When** the game master successfully forces the active puzzle to completion before or after that result is delivered, **Then** every player retains the authoritative solved outcome and sees no stale or raw rejection notice on either the hacking surface or subsequent terminal menu.

### User Story 6 - Search a Camouflaged Board (Priority: P1)

As a player, I search for special patterns in the same rows as candidate words, ordinary punctuation, and misleading delimiter characters, so valid patterns feel discovered rather than pre-labeled.

**Why this priority**: Camouflage is required for the special-pattern mechanic to preserve the intended deduction challenge without changing which spans are valid.

**Independent Test**: Generate 1,000 publishable initial boards, inspect their final rendered rows and public projections, interact with valid and invalid delimiter cases in the browser, remove a word from an alphabetic-interrupted span, and verify every camouflage and rediscovery rule plus individual delimiter selection and opening-symbol-only pattern activation independently of pattern outcomes. ~~The earlier test treated delimiter decoys as inert.~~ That interaction contract was superseded by BUG-001.

**Acceptance Scenarios**:

1. **Given** a newly published board, **When** its final rendered rows are inspected, **Then** candidate words, valid-pattern endpoints, and standalone delimiter decoys each occupy at least two rows, their occupied-row intervals overlap pairwise, and ordinary punctuation or filler remains present in at least two rows.
2. **Given** a newly published board, **When** its valid patterns are inspected, **Then** at least one has one or more non-alphabetic filler characters between its endpoints, while adjacent empty pairs remain allowed for other patterns.
3. **Given** a newly published board, **When** final production discovery and camouflage classification complete, **Then** the board has 3–6 valid patterns, at least as many standalone delimiter-decoy characters as valid patterns, and at least one matching-delimiter span invalidated by alphabetic content.
4. **Given** matching delimiters on opposite sides of a candidate word, **When** the player selects that word, **Then** it follows the existing candidate-word guess rules and the enclosing delimiter span has no pattern identity or pattern interaction.
5. **Given** dud removal replaces the incorrect candidate inside a word-interrupted delimiter span with periods, **When** production discovery analyzes the mutated canonical board, **Then** the span is published and selectable if and only if it now satisfies every existing special-pattern rule.
6. ~~**Given** a standalone, mismatched, word-interrupted, later-closer, or otherwise invalid delimiter, **When** the player hovers, focuses, or selects it, **Then** it does not highlight, send `HACK_PATTERN`, consume an attempt, or change puzzle state.~~ **Superseded by BUG-001**: such a delimiter remains outside the public pattern projection but receives ordinary individual filler-symbol preview, highlight, and selection behavior; it never sends `HACK_PATTERN` merely because it is a delimiter.
7. **Given** valid pattern endpoints and delimiter decoys before interaction, **When** their static presentation is compared, **Then** color, brightness, font, CRT effect, and other static styling reveal no validity difference; ~~only normal valid-pattern hover, focus, or selection behavior reveals validity~~ under BUG-001, only targeting an unused valid pattern's opening symbol reveals validity through whole-span interaction.
8. **Given** words and all camouflage characters have been placed, **When** the complete final rendered board fails any initial count or camouflage condition, **Then** the entire board is regenerated before any player receives it.
9. **Given** the complete final rendered board contains valid and invalid delimiter characters, **When** its public pattern projection is produced, **Then** only spans returned by production discovery receive public pattern identities.

## Edge Cases

- A selected dud-removal outcome falls back to attempt restoration when no currently available incorrect password can be removed.
- Attempt restoration sets remaining attempts to the configured maximum and never exceeds it, including when attempts are already at maximum.
- `<!%>`, `[.=+]`, `{#}`, and `()` are representative valid patterns because their interiors are empty or contain only non-alphabetic filler.
- `<PASSWORD>`, `[RAIDER+]`, and `{!ROBOT}` are representative invalid spans while their alphabetic content remains rendered.
- `!%]>`, `{+#)`, `[!%}`, and the trailing `]+>` in `()+]>` are representative standalone or mismatched delimiter-decoy arrangements.
- A closing delimiter before its opening delimiter, a mismatched delimiter type, a cross-row span, or a span containing any alphabetic character is invalid.
- Discovery pairs each opening delimiter only with the first compatible closing delimiter to its right on the same rendered row; later compatible closers do not form patterns for that opening while the first remains its pair.
- ~~An opening without a compatible closer, a closing without a compatible opener, and mismatched delimiter types remain inert rendered characters unless another valid opening-to-first-compatible-closing relationship uses them.~~ **Superseded by BUG-001**: these glyphs remain invalid as patterns but are individually selectable with ordinary filler-symbol behavior.
- A generated standalone delimiter decoy that falls inside any valid pattern's inclusive ~~selectable range cannot satisfy the inert-decoy requirement~~ span cannot satisfy the standalone-decoy classification and is not counted toward the initial decoy minimum. BUG-001 changes interaction anchoring, not camouflage classification.
- A candidate word between matching delimiters remains selectable as a normal word while the surrounding span is invalid; replacing an incorrect word with periods may make that exact span valid on rediscovery.
- Multiple opening delimiters may share one closing delimiter, and each complete coordinate pair has independent used state.
- Board mutation may create, remove, or change pattern pairings. A newly formed coordinate pair is available if it has not been used in the current generation; a previously used pair remains unavailable if rediscovered.
- Delimiter decoys may accidentally participate in a valid span with other board characters; final production discovery treats the resulting span as valid, removes affected characters from the standalone-decoy count, and includes the span in the initial 3–6 count.
- The published initial board is regenerated when final-board discovery returns fewer than three or more than six distinct selectable patterns, including patterns formed accidentally by filler or camouflage characters.
- The published initial board is also regenerated when it lacks the minimum standalone delimiter-decoy count, a non-empty valid pattern interior, an alphabetic-interrupted potential span, or the required occupied-row distribution.
- The 3–6 count applies only before the first player action. Later board mutation may cause the valid pattern count to exceed six.
- A delayed request carrying an old generation identifier cannot activate coincident coordinates in a new generation.
- A request with missing, unknown, or invalid fields is rejected before used-state mutation or random selection.
- Pattern requests received before a puzzle is actionable or after success, failure, or other terminal state have no effect.
- Concurrent requests for the same pattern yield exactly one accepted activation; all duplicates are rejected without consuming attempts or random values.
- A guess or hacking-presentation result may arrive immediately before or after a trusted forced-success snapshot; the solved snapshot remains authoritative in both orderings, and any notice from the superseded hacking context is cleared rather than carried to the terminal menu.
- Process restart may discard the active puzzle and all special-pattern progress; reconnect synchronization is guaranteed only while the same server process retains the canonical puzzle.

## Requirements

### Functional Requirements

- **FR-001**: Phase 1 MUST replace player-accessible hacking cheats with server-authoritative Fallout-style special patterns.
- **FR-002**: The player experience MUST expose no command, board entry, protocol operation, browser global, DOM control, keyboard shortcut, or query parameter that directly forces puzzle success.
- **FR-003**: The player experience MUST remove the former administrator shortcut that bulk-removes incorrect passwords.
- **FR-004**: ~~Ordinary candidate-word guesses and non-delimiter filler clicks MUST retain their existing password-match, likeness, attempt-spending, logging, success, and failure rules.~~ **Superseded by BUG-001 and FR-084 through FR-088** because ordinary filler behavior also applies to delimiter glyphs unless an available pattern is activated from its opening symbol.
- **FR-005**: The existing game-master `ForceHackSuccess` control MUST remain available only through the private desktop/Wails boundary.
- **FR-006**: `ForceHackSuccess` MUST NOT be exposed through the player WebSocket protocol, browser globals, DOM controls, keyboard shortcuts, or query parameters.
- **FR-007**: A successful trusted `ForceHackSuccess` action MUST use the existing shared success flow without consuming an attempt.

- **FR-008**: A valid special pattern MUST use exactly one matching delimiter type from `()`, `[]`, `{}`, or `<>`.
- **FR-009**: A valid special pattern MUST start and end on the same rendered row.
- **FR-010**: A valid special pattern MUST contain no alphabetic character between its endpoints.
- **FR-011**: Pattern discovery MUST pair each opening delimiter with the first compatible closing delimiter to its right on the same rendered row.
- **FR-012**: A discovered pattern MUST be represented by an inclusive opening-through-closing coordinate range.
- **FR-013**: Multiple opening delimiters that share the same first compatible closing delimiter MUST be discovered as distinct patterns.
- **FR-014**: Using one pattern MUST NOT consume another pattern with a different complete coordinate pair.
- **FR-015**: Before publication, a newly generated puzzle board MUST contain between three and six distinct selectable special patterns inclusive.
- **FR-016**: The initial pattern count MUST be computed from the final rendered board using the same discovery algorithm used during gameplay.
- **FR-017**: A final rendered board with fewer than three or more than six discovered patterns MUST be regenerated before publication.
- **FR-018**: The initial three-to-six limit MUST apply only until the first player action.
- **FR-019**: Gameplay discovery after board mutation MUST publish all current valid patterns even when their count exceeds six.
- **FR-020**: The initial three-to-six limit MUST NOT vary by hacking difficulty.

- **FR-021**: A special-pattern identity MUST contain the puzzle generation identifier, rendered row, opening-character index, and closing-character index.
- **FR-022**: Pattern used state MUST belong to the complete generation-and-coordinate identity.
- **FR-023**: A complete pattern identity MUST be accepted at most once during its puzzle generation.
- **FR-024**: When board mutation causes the same opening character to pair with a different first compatible closing character, the resulting coordinate pair MUST be treated as a new pattern identity.
- **FR-025**: A coordinate pair already used in the active puzzle generation MUST remain unavailable if that pair is later rediscovered.
- **FR-026**: Replacing a removed incorrect password with periods MUST trigger discovery against the resulting canonical rendered board.

- **FR-027**: Each accepted pattern activation MUST independently select one outcome using server-side weighted randomness.
- **FR-028**: Eighty percent of the random-source value space MUST map to dud removal.
- **FR-029**: Twenty percent of the random-source value space MUST map to attempt restoration.
- **FR-030**: A dud-removal outcome MUST remove exactly one currently available incorrect password candidate.
- **FR-031**: Dud removal MUST replace the selected incorrect password with periods without altering the correct password.
- **FR-032**: An attempt-restoration outcome MUST set remaining attempts to the configured maximum without exceeding it.
- **FR-033**: A selected dud-removal outcome MUST apply attempt restoration instead when no removable incorrect password remains.
- **FR-034**: Invalid, stale, already-used, or otherwise rejected pattern requests MUST NOT consume a random-source value.
- **FR-035**: Verification MUST use an injected deterministic random source for outcome mapping and activation-order tests.
- **FR-036**: A controlled source spanning 100 equiprobable values MUST map exactly 80 values to dud removal and 20 values to attempt restoration.
- **FR-037**: Production behavior MUST be judged by the configured probability mapping rather than by requiring every sample of 100 accepted activations to contain exactly 80 dud removals and 20 restorations.

- **FR-038**: Every `HACK_PATTERN` request MUST identify both the target pattern and the puzzle generation from which the client received it.
- **FR-039**: A pattern target MUST be represented either by an opaque server-issued `patternId` that resolves to generation and coordinates or by an explicit equivalent containing only `generationId`, `row`, `start`, and `end`.
- **FR-040**: The server MUST reject a `HACK_PATTERN` payload containing missing, unknown, or invalid fields.
- **FR-041**: The server MUST reject a `HACK_PATTERN` request whose generation does not match the active puzzle generation.
- **FR-042**: The server MUST reject coordinates that do not identify a currently valid pattern in canonical board state.
- **FR-043**: The server MUST reject a pattern identity already marked used.
- **FR-044**: The server MUST reject a pattern request when the puzzle is not in an actionable state.
- **FR-045**: A request from an older generation MUST NOT activate a pattern at matching coordinates in a newer generation.
- **FR-046**: Accepted pattern activation MUST execute under the canonical live-service mutex.
- **FR-047**: Under that mutex, activation MUST perform these steps in order: validate the active generation; validate or rediscover the requested coordinate pair against canonical board state; verify unused state; mark the pair used; select the weighted outcome; apply the outcome or fallback; recompute patterns affected by mutation; produce a detached public projection; broadcast resulting canonical state.
- **FR-048**: Concurrent requests for the same pattern MUST result in exactly one accepted activation.
- **FR-049**: Rejected duplicate requests MUST NOT mutate canonical state.
- **FR-050**: Rejected duplicate requests MUST NOT advance the random source.
- **FR-051**: Player authorization MUST remain behind a distinct player/live-service boundary so a future active-session check can be added without changing hacking domain logic.
- **FR-052**: Phase 1 MUST NOT implement an active controlling-player or observer authorization model.

- **FR-053**: The public pattern projection MUST contain only a stable public pattern identity, rendered row, inclusive start and end coordinates, and current available-or-used status.
- **FR-054**: The public pattern projection MUST contain the rendered row and inclusive start and end coordinates.
- **FR-055**: The public pattern projection MUST state whether each pattern is currently available or used.
- **FR-056**: The public pattern projection MUST NOT reveal the password, dud identities, future effects of unused patterns, or private candidate metadata.
- **FR-057**: Every public projection MUST be detached from canonical slices, maps, and objects.
- **FR-058**: Mutating a returned projection MUST NOT alter canonical state.
- **FR-059**: Pattern availability, used identities, removed duds, remaining attempts, and pattern outcomes MUST remain runtime-only state.
- **FR-060**: A reconnecting client MUST receive the current canonical puzzle state while the originating server process retains that puzzle.
- **FR-061**: Phase 1 MUST NOT preserve an active puzzle across server application restarts.
- **FR-062**: Phase 1 MUST NOT modify the version-1 persisted session schema.

- **FR-063**: Every initially published board MUST interleave candidate password words, ordinary punctuation and filler symbols, delimiters belonging to valid special patterns, and standalone delimiter-decoy characters ~~outside every currently valid pattern's inclusive interaction range~~ not contained in any currently valid pattern's inclusive span. The struck interaction wording was superseded by BUG-001; this requirement remains a board-classification rule.
- **FR-064**: On every initially published board, candidate words, valid-pattern endpoints, and standalone delimiter decoys MUST each occupy at least two rendered rows, their occupied-row intervals MUST overlap pairwise, and ordinary punctuation or filler MUST remain present in at least two rendered rows.
- **FR-065**: Production discovery MUST accept a matching same-row pattern with one or more ordinary non-alphabetic filler characters between its opening and first compatible closing delimiter.
- **FR-066**: Every initially published board MUST contain at least one valid special pattern with one or more non-alphabetic filler characters between its endpoints.
- **FR-067**: Production discovery MUST continue to accept an adjacent empty matching delimiter pair that satisfies all normal special-pattern rules.
- **FR-068**: The generator MUST NOT construct every initial valid pattern as an adjacent empty pair.
- **FR-069**: Matching delimiters surrounding a candidate word or other alphabetic content MUST NOT form a valid special pattern while that alphabetic content remains rendered.
- **FR-070**: A candidate word inside an alphabetic-interrupted delimiter span MUST remain selectable under the existing candidate-word guess rules.
- **FR-071**: When dud removal replaces an incorrect word inside a delimiter span with periods, production discovery MUST publish the resulting span if it satisfies every normal special-pattern rule on the mutated board.
- **FR-072**: Every initially published board MUST contain standalone delimiter-decoy characters that are not part of any currently valid pattern ~~interaction range~~ inclusive span. BUG-001 limits the pattern activation target to the opening symbol without changing this classification.
- **FR-073**: The number of standalone delimiter-decoy characters on every initially published board MUST be at least the number of initially valid special patterns.
- **FR-074**: Every initially published board MUST contain at least one potential matching-delimiter span that is invalid because alphabetic content appears between its endpoints.
- **FR-075**: ~~A standalone delimiter decoy MUST NOT highlight, produce a `HACK_PATTERN` request, consume an attempt, or change puzzle state.~~ **Superseded by BUG-001 and FR-084/FR-088**: it remains invalid as a pattern but retains ordinary individual filler-symbol interaction.
- **FR-076**: A delimiter decoy MUST use the same color, brightness, font, CRT effect, and static styling as the same delimiter character when it belongs to a valid pattern.
- **FR-077**: ~~Pattern validity MUST be revealed only through normal hover, focus, or selection behavior for currently valid patterns.~~ **Superseded by BUG-001 and FR-085/FR-086**: only interaction with an available pattern's opening symbol may reveal validity by highlighting or activating the whole span.
- **FR-078**: Candidate words and all camouflage characters MUST be added before initial-board validation begins.
- **FR-079**: Initial-board validation MUST run the production discovery algorithm against the complete final rendered board and count every accidentally formed valid pattern toward the initial 3–6 limit.
- **FR-080**: Initial-board validation MUST regenerate a board that fails the 3–6 valid-pattern limit, standalone delimiter-decoy minimum, non-empty valid-interior requirement, alphabetic-interrupted-span requirement, or mixed-distribution requirement.
- **FR-081**: The public pattern projection MUST include only spans that are currently valid under production discovery.
- **FR-082**: Standalone, mismatched, word-interrupted, later-compatible-but-unselected, and otherwise invalid delimiters MUST receive no public pattern identity.
- **FR-083**: Camouflage construction and final-board validation MUST preserve the discovery rules in FR-008 through FR-013 unchanged.
- **FR-084**: Every rendered filler symbol, including `(`, `)`, `[`, `]`, `{`, `}`, `<`, and `>`, MUST remain individually selectable unless that exact cell is the opening symbol at `start` of a currently valid special pattern; unused openings use pattern interaction and used openings retain their existing unavailable behavior.
- **FR-085**: The cell at a current pattern's `start` coordinate MUST be the only hover, focus, or selection target that resolves to that pattern; the inclusive `start`-through-`end` coordinates remain the resulting highlight and effect span, not the hit area.
- **FR-086**: Hovering or focusing an unused pattern's opening symbol MUST highlight and preview its whole inclusive span, and selecting that opening symbol MUST send exactly one `HACK_PATTERN` request containing the opaque `patternId`.
- **FR-087**: Hovering, focusing, or selecting a non-opening filler symbol inside a valid pattern span, including its closing delimiter, MUST use ordinary individual filler-symbol behavior and MUST NOT highlight or activate the enclosing pattern.
- **FR-088**: Selecting an individually actionable delimiter symbol that is not a current pattern's opening coordinate MUST use the existing `HACK_GUESS` filler-target path, including its established logging and attempt-spending behavior, and MUST NOT send `HACK_PATTERN`.
- **FR-089**: When an authoritative snapshot transitions the active hacking generation to solved, every player client MUST give that terminal outcome precedence over pending or later action and presentation results from the superseded actionable hacking context; those results MUST NOT replace the solved outcome with a rejection notice.
- **FR-090**: A transient rejection notice produced by a hacking action or hacking-presentation result MUST remain scoped to the hacking context that produced it and MUST be cleared when an authoritative solved snapshot ends that context, without suppressing a rejection for an unrelated current action.

**BUG-003 clarification for FR-089–FR-090**: The superseded hacking context is the ended actionable puzzle generation, not merely the controller-presentation value visible when the solved update is applied. The same precedence and notice-lifetime rules apply when success arrives through a full live-terminal snapshot or a hacking-only projection and when the presentation context is absent, changes in the same update, or was already advanced by another authoritative update.

## Key Entities

- **Puzzle Generation**: One runtime hacking puzzle instance with an opaque generation identifier that prevents actions from an older puzzle targeting a newer one.
- **Special Pattern Identity**: The immutable tuple of puzzle generation identifier, rendered row, opening-character index, and closing-character index.
- **Special Pattern**: A currently discovered inclusive coordinate span plus its stable public identity and current available-or-used status.
- **Used Pattern History**: The runtime-only set of complete coordinate identities already accepted during the active puzzle generation, retained even when a pair temporarily disappears.
- **Hacking Puzzle**: The canonical runtime challenge containing the rendered board, private candidate metadata, correct password, configured attempt maximum, remaining attempts, used-pattern history, progress log, and outcome.
- **Candidate Password**: A selectable word that is either the correct password or an incorrect dud; only a currently available incorrect candidate may be removed by a pattern.
- **Delimiter Decoy**: A rendered opening or closing delimiter deliberately left outside every currently valid pattern ~~interaction range so it looks plausible but remains inert~~ inclusive span so it looks plausible, remains individually selectable as ordinary filler under BUG-001, and receives no public pattern identity.
- **Alphabetic-Interrupted Span**: Matching delimiters with candidate-word or other alphabetic content between them; invalid while the letters remain, but eligible for normal rediscovery after board mutation removes the alphabetic content.
- **Pattern Outcome**: The server-selected effect of one accepted activation: dud removal or attempt restoration, with restoration used as the fallback when dud removal has no eligible target.
- **Public Pattern Projection**: A detached rendering and interaction view containing only stable public identity, row, inclusive coordinates, and current available-or-used state.
- **Player Action**: An untrusted request submitted through the player/live-service boundary to guess a word or activate a generation-bound pattern.
- **Game-Master Action**: A trusted `ForceHackSuccess` request available only through the private desktop/Wails boundary.

## Non-Goals

Phase 1 does not implement or modify:

- one active controlling player with observer sessions;
- session names or game-master reassignment;
- a Fallout visual or CRT redesign;
- modern-resolution rendering changes;
- dictionary import or dictionary validation;
- localization or audio controls;
- terminal switching;
- new lockout modes;
- persistent unlocked state;
- existing ordinary-word or filler-click attempt rules;
- deterministic persistence of a complete puzzle seed;
- active-puzzle persistence across server application restarts; or
- the version-1 persisted session schema.

The player/live-service authorization boundary is preserved only as an extension point for a future active-session check. No session-controller role, observer role, or reassignment behavior is introduced here.

## Success Criteria

### Measurable Outcomes

- **SC-001**: With an injected deterministic source spanning 100 equiprobable values, exactly 80 source values select dud removal and exactly 20 select attempt restoration.
- **SC-002**: In all rejection-path tests for invalid, stale, already-used, duplicate, and non-actionable requests, the deterministic random source records zero consumed values and canonical state remains byte-for-byte equivalent in observable fields.
- **SC-003**: Across 1,000 generated and publishable initial boards, production discovery and final-board classification confirm that every complete rendered board has 3–6 distinct selectable patterns, at least as many standalone delimiter-decoy characters as valid patterns, at least one valid pattern with a non-empty non-alphabetic interior, at least one alphabetic-interrupted matching-delimiter span, at least two occupied rows per candidate-word, valid-endpoint, and standalone-decoy category, pairwise-overlapping occupied-row intervals for those categories, ordinary punctuation or filler in at least two rows, and a public projection containing exactly the discovered valid patterns and no invalid delimiters.
- **SC-004**: Every generated board that fails any initial pattern-count or camouflage condition is rejected for publication and regenerated after the complete board is analyzed.
- **SC-005**: Every allowed delimiter type is discovered under the first-compatible-closer rule, while 100% of tested cross-row, mismatched, and alphabetic-content spans are rejected.
- **SC-006**: In all tested shared-closer boards, each distinct complete coordinate pair can be activated once independently.
- **SC-007**: In all tested mutation sequences, a new coordinate pair is available once, while any previously used pair remains unavailable if rediscovered.
- **SC-008**: Concurrent activation tests for the same pattern always produce exactly one accepted request, one applied outcome, one random-source advancement, and one resulting canonical broadcast state.
- **SC-009**: Mutating every field and nested mutable value reachable from a returned public projection never changes canonical board, candidate, attempt, identity, or used-state data.
- **SC-010**: Two connected players and one reconnecting player display identical current board text, attempts, pattern availability, used state, removed-dud effects, and puzzle outcome while the same server process is running.
- **SC-011**: Restart-boundary tests confirm that no active-pattern runtime state is written to or required from version-1 session data.
- **SC-012**: Player-surface checks find zero routes to `ForceHackSuccess` through WebSocket messages, browser globals, DOM controls, keyboard shortcuts, or query parameters, while the existing private desktop/Wails control still completes an eligible puzzle through the normal shared success flow.
- **SC-013**: ~~Regression tests confirm that ordinary password guesses and non-delimiter filler clicks retain their pre-feature attempt behavior.~~ **Superseded by BUG-001 and SC-018** to cover delimiter glyphs as ordinary filler symbols when they are not a current pattern's opening coordinate.
- **SC-014**: Controlled generator tests prove that adjacent empty pairs remain discoverable but no publishable initial board relies exclusively on adjacent empty pairs.
- **SC-015**: ~~Browser interaction tests confirm that 100% of tested delimiter decoys produce no highlight, focus preview, `HACK_PATTERN` request, attempt consumption, log change, board mutation, or outcome change, while valid patterns retain their existing inclusive interaction behavior.~~ **Superseded by BUG-001, SC-018, and SC-019** because delimiters are individually selectable and only a pattern's opening symbol activates the inclusive span.
- **SC-016**: Dynamic-board tests confirm that an alphabetic-interrupted span has no public identity before dud removal and is published immediately after the word becomes periods when the resulting span satisfies all unchanged discovery rules.
- **SC-017**: Static-style checks find no validity-dependent color, brightness, font, CRT effect, class, or other persistent visual treatment on delimiter characters before interaction.
- **SC-018**: Browser and domain interaction tests confirm that 100% of tested standalone, mismatched, word-interrupted, later-closer, and otherwise invalid delimiter targets use ordinary individual filler-symbol highlight, preview, `HACK_GUESS`, logging, and attempt behavior while sending zero `HACK_PATTERN` requests.
- **SC-019**: For every controlled valid-pattern span, interaction with `start` highlights the complete inclusive span and selection sends one `HACK_PATTERN`, while interaction with every tested offset greater than `start` highlights or selects only that symbol and sends no `HACK_PATTERN`.
- **SC-020**: Deterministic browser tests covering a rejected or unsuccessful guess followed by trusted forced success, with correlated shared-action and hacking-presentation results delivered both before and after the solved snapshot through streamed and unary paths, show the authoritative success flow to the active controller and observers and display zero stale or raw rejection notices on the hacking surface and subsequent terminal menu.

**BUG-003 verification clarification for SC-020**: Automated coverage MUST include full live-terminal and hacking-only solved publications with absent, changed, and already-advanced presentation context. The final evidence MUST also include a native replay in which the player selects the first incorrect password and the game master immediately uses the real private Overseer success control; fixture-only success injection is insufficient final evidence.

## Assumptions

- The canonical rendered board is the source used by one shared pattern-discovery algorithm during initial publication checks and gameplay rediscovery.
- Character indices are row-local indices into the rendered board representation, and `start` and `end` are inclusive.
- Alphabetic means the same character classification used consistently by initial and gameplay discovery; selecting a concrete classification mechanism is deferred to planning without changing the no-alphabetic-character rule.
- The configured maximum attempts already exists in the hacking puzzle and is not changed by this feature.
- Weighted randomness is server-side and independently sampled only after a request has passed generation, coordinate, actionable-state, and unused-state validation.
- The deterministic 80/20 test proves the probability mapping; production samples are allowed to vary naturally.
- Board mutation from dud removal replaces only that candidate's rendered characters with periods and may increase, decrease, or otherwise change the discovered pattern set.
- A generated standalone delimiter decoy is placed outside every currently valid pattern's inclusive span. ~~A delimiter character inside a valid range follows the existing inclusive valid-pattern interaction.~~ **Superseded by BUG-001**: only the opening coordinate activates the span; every non-opening filler symbol uses ordinary individual selection. A delimiter inside the span still does not count toward the standalone-decoy minimum.
- An occupied-row interval is the inclusive range from the lowest to highest rendered-row ordinal containing a category; two occupied-row intervals overlap when their inclusive ranges share at least one row ordinal, even when the categories do not occupy the same character cells.
- Distributed content means candidate words, valid-pattern endpoints, and standalone delimiter decoys each occupy at least two rows and their occupied-row intervals overlap pairwise; ordinary punctuation or filler remains present in at least two rows, and no category-specific row block is reserved.
- Initial camouflage validation derives valid spans from production discovery first, then classifies remaining delimiter characters and alphabetic-interrupted spans from the same complete rendered board.
- Runtime-only live puzzle data remains available to reconnecting clients only for the lifetime of the server process that owns it.
- The existing player/live-service boundary is the authorization extension point; Phase 1 adds no controlling-player session model.
- Existing candidate selection, likeness calculation, word-guess attempts, filler-click behavior, lockout, terminal navigation, and normal success transition remain unchanged.

## Verbatim Constraints

- Allowed pattern pairs: `()`, `[]`, `{}`, `<>`.
- Initial valid special-pattern count: `3–6` inclusive on the final rendered board before the first player action.
- Dud-removal probability mapping: `80%`.
- Attempt-restoration probability mapping: `20%`.
- Pattern identity: `generationId + row + inclusive start + inclusive end`.
- Pattern activation request: `HACK_PATTERN`.
- Persistence boundary: runtime-only; no version-1 session schema change.
