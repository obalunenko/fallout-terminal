# Data Model: Facility Authoring UX

No persistence schema changes.

- **Facility draft**: Existing complete cloned session; base session revision captured when initialized; dirty flag; selected device/condition IDs. Form application modifies only the clone. Save uses the captured revision and existing facility revision. External updates replace the canonical session but preserve a dirty draft, exposing a conflict instead of silently rebasing it. Discard rebuilds from canonical state.
- **Relationship locator**: Local property/type plus terminal/node/block identity and variant index, resolved against the current draft at action time. Never serialized. Missing targets produce visible feedback. Editing replaces the selected relation; removal removes exactly that relation.
- **Device draft**: Clone existing device, states, and transitions and overlay represented form fields, preserving recovery metadata and unrelated attributes. Existing current state must belong to the edited states; removing a current state fails validation until repaired.
- **Preview request generation**: Local monotonically increasing token invalidates older responses on selection change/close. Baseline and override trees come from the existing preview boundary. UI compares stable IDs and resolved text/availability; absent baseline nodes are listed as hidden.

Validation remains authoritative at save. Local checks provide prompt errors for incomplete references and duplicate/unknown states. None of these transient UI fields enter the saved session or player payload.
