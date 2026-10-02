# Research: Facility Authoring UX

## Reference preservation

**Decision**: Refresh only transition source/destination selectors; update prerequisite state selectors from their selected device; validate incomplete rows. Clone original states/transitions/devices before overlaying edited fields.

**Rationale**: `refreshFacilityStateChoices` currently replaces all nested select options. Adding the next transition destroys the previous transition's condition selections, and form serialization filters blank references. Rebuilding entities also loses attributes such as `recovery`.

**Alternatives considered**: Patching placeholders alone cannot preserve references; weakening backend validation would conceal malformed drafts.

## Draft ownership

**Decision**: One local session draft, one base session revision, explicit stage/save/discard. Retain the entire dirty draft on external updates and reject a stale publication with guidance to discard/reapply, rather than merging unrelated whole-session changes implicitly. Keep reset/recovery unavailable while authoring changes need save/discard.

**Rationale**: Bindings live on terminal nodes while device graphs live in the facility. Preserving only the facility on session events loses staged bindings. Device-only-looking save currently commits everything.

**Alternatives considered**: Autosave would publish partially prepared graphs. Automatic three-way merging is unnecessary complexity for this follow-up.

## Navigation and workspace

**Decision**: A left device list and right detail panel, followed by categorized relationship/fault/program sections within the detail workspace. Search devices, filter relationships for selection, expose all-session rules when no device is selected. Preserve existing IDs where practical for fixture compatibility.

**Rationale**: Long global binding lists currently control page height and push selection controls out of view. Human names and compact purpose summaries make live operation possible.

**Alternatives considered**: A visual node-graph editor adds interaction complexity and dependencies without solving save semantics or reference editing.

## Preview

**Decision**: Resolve both a saved baseline and a hypothetical override through the existing private preview API; compare detached trees by stable node/block identity and render changed/hidden/unavailable indicators. Guard unsaved configuration and ignore stale responses.

**Rationale**: Local evaluation would duplicate authoritative rules. The current API accepts an override but no draft, so preview must clearly describe its saved basis.

**Alternatives considered**: Adding a draft-preview contract expands backend/schema scope; showing only unmarked text makes consequences hard to assess.

## Verification seams

**Decision**: Use existing Playwright facility fixture routes, adding browser-level preservation assertions and end-to-end UX scenarios. Use actual demo data for layout evidence.

**Rationale**: Existing tests assert controls exist but do not verify precondition/effect option preservation across multi-transition editing.

**Alternatives considered**: DOM-only unit tests cannot demonstrate scrolling, focus, navigation, or saved-state boundaries.
