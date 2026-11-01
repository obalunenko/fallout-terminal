# Contract: Player Action Reason

## Proto (`proto/fallout/terminal/player/v1/player.proto`)

```proto
enum ActionReason {
  // ...existing values 0 to 10 unchanged...
  // An active fault's capability block refused the request.
  ACTION_REASON_FAULT_BLOCKED = 11;
}
```

Additive. Older clients read an unknown value; `client.js` already falls back to the generic message for it.

## When the server returns it

The capability-blocked branch of `Service.DispatchPlayerAction` returns `ACTION_REASON_FAULT_BLOCKED` for these requests while a fault that blocks the capability is active on the terminal:

| Player request | Capability |
|---|---|
| Open an entry (`navigate`, action `entry`) | `view-entry` |
| Run a command | `execute-command` |
| Run a terminal-transition command | `terminal-transition` |
| Run a recovery-program command | `run-recovery-program` |
| Guess or activate a pattern while hacking | `hack` |

Every other refusal keeps its current reason. A duplicate request ID replays the cached result, as today.

## Player text (`frontend/client/client.js`)

| Reason name | `#playerNotice` text |
|---|---|
| `fault-blocked` | `ДЕЙСТВИЕ ОТКЛОНЕНО: МЕШАЕТ НЕИСПРАВНОСТЬ СИСТЕМЫ.` |

The text names no fault. `actionReasonName` maps `ActionReason.FAULT_BLOCKED` to `fault-blocked`.
