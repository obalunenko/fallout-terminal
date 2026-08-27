# Contract: Public Player Protobuf Additions

The governed source is `proto/fallout/terminal/player/v1/player.proto`. Field numbers below are
additive and must not renumber or reinterpret existing fields.

## Proposed schema shape

```proto
message SubscribeRequest {
  optional string recognition_handle = 1;
  optional string client_instance_id = 2;
}

message PresentationUplinkOpen {
  string client_instance_id = 1;
  uint64 uplink_generation = 2;
  string recognition_handle = 3;
}

message PresentationIntent {
  string recognition_handle = 1;
  string request_id = 2;
  string broadcast_id = 3;
  string terminal_id = 4;
  string context_key = 5;
  ControllerTerminalPresentation presentation = 6;
}

message PresentationUplinkRequest {
  oneof payload {
    PresentationUplinkOpen open = 1;
    PresentationIntent intent = 2;
  }
}

message PresentationUplinkReady {}

message PresentationUplinkResult {
  string client_instance_id = 1;
  uint64 uplink_generation = 2;
  oneof payload {
    PresentationUplinkReady ready = 3;
    ActionResult action = 4;
  }
}

message PresentationUplinkResponse {}

message SubscriptionMessage {
  oneof payload {
    PersonalizedSnapshot snapshot = 1;
    CompoundUpdate update = 2;
    PresentationUplinkResult presentation_uplink_result = 3;
  }
}

service PlayerService {
  rpc Subscribe(SubscribeRequest) returns (stream SubscriptionMessage);
  rpc SelectCharacter(SelectCharacterRequest) returns (ActionResult);
  rpc Navigate(NavigateRequest) returns (ActionResult);
  rpc Guess(GuessRequest) returns (ActionResult);
  rpc ActivatePattern(ActivatePatternRequest) returns (ActionResult);
  rpc SetPresentation(SetPresentationRequest) returns (ActionResult);
  rpc PresentationUplink(stream PresentationUplinkRequest) returns (PresentationUplinkResponse);
  rpc SoundManifest(SoundManifestRequest) returns (SoundManifestResponse);
}
```

The implementation may place the new declarations near related presentation messages, but names,
cardinality, field numbers, and directions in this contract are stable once implementation begins.

## Direction and ownership

| Contract | Producer | Consumer | Authority |
|---|---|---|---|
| `SubscribeRequest.client_instance_id` | Browser tab | Player handler | Routing identity only; never authorization. |
| `PresentationUplinkOpen` | Browser tab | Player handler/hub | Requests one current physical binding and probe. |
| `PresentationIntent` | Active controller browser | Player adapter/coordinator | Input only; canonical validation remains server-owned. |
| `PresentationUplinkResult.ready` | Player handler | One physical `Subscribe` | Proves an open request frame crossed the deployment path. |
| `PresentationUplinkResult.action` | Player processor | One physical `Subscribe` | Existing canonical acceptance/rejection vocabulary, targeted to one tab. |
| `PresentationUplinkResponse` | Player handler | Closing client stream | Empty normal-close acknowledgement; not used for open-stream decisions. |

## Validation and rejection

- `SubscribeRequest.client_instance_id`, when present, must satisfy existing bounded public-field
  validation. Absence retains old-client behavior but makes uplink binding unavailable.
- The first `PresentationUplinkRequest` must contain `open`; a missing, repeated, or later open frame
  is a stream-level invalid-argument error.
- Open identity and generation must be non-empty/non-zero, match one current `Subscribe`, resolve to
  the same logical session, and supersede rather than equal/regress the last generation.
- Every intent is validated independently for protobuf size, presence, public fields, exclusive
  presentation variant, repeated context agreement, recognition/session, authority, broadcast,
  terminal, context, and current generation.
- Structurally malformed/oversized stream messages fail the stream. Valid but stale/unauthorized
  intents return a targeted `ActionResult` rejection and do not mutate canonical state.

## Compatibility

- Existing field numbers 1–2 of `SubscriptionMessage`, all current request/response messages, and
  all existing procedures remain unchanged.
- `SetPresentation` remains generated and callable for all browser deployments.
- Older clients may omit `client_instance_id`, ignore unknown downlink field 3, and continue unary
  and server-streaming behavior.
- No private, config, persistence, provider, credential, secret-word, or candidate schema is imported
  into the public player descriptor.
- Run Buf format/lint/build, compatibility breaking checks against
  `proto/compatibility-baseline.binpb`, deterministic Go/ECMAScript generation, descriptor privacy,
  and generated client/server compilation. The baseline remains the compatibility reference rather
  than being advanced merely to hide an additive change.

## Generated artifacts

Generated outputs include `internal/gen/fallout/terminal/player/v1/player.pb.go`, the generated
`playerv1connect` service, and `frontend/client/gen/fallout/terminal/player/v1/player_pb.js` with its
`PlayerService` descriptor. Generated files are never edited manually.
