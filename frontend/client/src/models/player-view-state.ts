export type PlayerViewPhase = 'idle' | 'loading' | 'ready' | 'unavailable';

export interface PlayerViewState {
  readonly phase: PlayerViewPhase;
  readonly message: string;
}
