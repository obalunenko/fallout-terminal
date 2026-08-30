import type { PlayerViewState } from '../models/player-view-state.js';

export interface PlayerTransport {
  readonly current: Readonly<PlayerViewState>;
}
