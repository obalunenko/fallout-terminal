# Contract: Public Player Navigation

## Wire compatibility

No new player RPC, request, response, stream, or public group message is added. The player continues to use the existing generated unary navigation request and server-streamed terminal presentation.

The existing public fields remain authoritative:

- active terminal identity and complete terminal presentation;
- `TerminalNavigationPresentation.route_depth`;
- optional `return_target`;
- optional pending direction and target; and
- broadcast and stream revision used for stale rejection and reconnect convergence.

Group CRUD, membership, confirmation impact, session revision, and private group names remain absent from public descriptors and payloads.

## Eligibility behavior

- A forward terminal-transition command is eligible only when the latest catalog snapshot proves that source and target exist in the same non-empty group.
- A root backward request is eligible only when the route has a current top whose target still shares the active terminal's group.
- A fresh broadcast first activated at a middle group member exposes the immediately preceding ordered member through the existing `return_target`; no client-generated group position is accepted.
- Cross-group, missing, self, stale-link, and stale-order attempts produce no terminal or route mutation.
- Legacy sessions normalized to singleton groups do not make old cross-singleton links executable until the Overseer intentionally couples their endpoints.

## Approval behavior

Every eligible forward or backward selection creates exactly one server-owned pending decision. Before approval, active terminal and route remain unchanged. Approval repeats link, route, terminal, group, and seeded-order validation, then applies exactly one terminal/route effect. Reject and close apply none.

A concurrent private group proposal must validate against the same pending and route state. If group commit wins first, the later player approval revalidates the new group state. If a player action advances coordination first, the stale private proposal is rejected.

## Authorization and reconnection

Only the assigned controlling player may submit navigation actions. Observers and unassigned players remain read-only. Controller, observers, and reconnecting clients receive the same revisioned active terminal, route depth, return target, and pending projection; clients do not calculate or repair group navigation locally.

