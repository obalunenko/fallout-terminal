# Data Model: Public Access Credential Sharing

## Player Credential Summary

Read-only projection used by the main public-access settings dialog.

| Field | Source | Rules |
|---|---|---|
| Login | Public-access preferences | Trimmed, non-empty, no newline; falls back to `players` only through existing normalization. |
| Password presence | Secure-store presence snapshot | `present`, `absent`, or `unknown`; never contains the password. |
| Password display | Derived presentation | Exactly `*****` only when presence is `present`; otherwise an explicit absence/unavailable label. |
| Can share | Derived presentation | True only when the login is valid, password presence is `present`, and no public-access command is pending. |

## Player Credential Draft

Ephemeral form state owned by the child dialog.

| Field | Rules |
|---|---|
| Login | Pre-filled from the current preference; required and normalized by the existing settings mutation. |
| Replacement password | Starts empty, uses password input semantics, and is cleared on every close path. |
| Delete pair | Explicit destructive action; sends the default login and requests password deletion. |

An empty replacement password preserves an existing saved password. When no saved password exists, the dialog requires a valid replacement before saving.

## Share Operation

One-shot native operation with no reusable secret-bearing result.

1. Resolve the current normalized login.
2. Open a scoped secure-store callback for only the player password.
3. Validate the stored password.
4. Write `Логин: <login>\nПароль: <password>` to the injected native clipboard.
5. Clear temporary password bytes when the callback returns.
6. Return only `{ok: true}` or `{ok: false, error: <safe message>}`.

## State Transitions

| Current state | Action | Result |
|---|---|---|
| Password absent/unknown | Open credentials | Login draft opens; replacement password is required to save; Share disabled. |
| Password present | Open credentials | Login draft opens; password field is empty; blank password preserves saved value. |
| Any editable state | Save valid draft | Existing atomic public-access mutation updates login and optional password; summary refreshes. |
| Password present | Delete pair | Password is deleted, login becomes `players`, Share becomes disabled. |
| Password present | Share | Native clipboard receives both values; page receives only outcome. |
| Any state | Cancel/close | Draft password is cleared; no mutation occurs; focus returns to Edit. |
