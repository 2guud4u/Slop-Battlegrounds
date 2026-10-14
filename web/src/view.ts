import type { FighterView, PlayerView, State } from "./components/types";

// Derived views of State — every projection the components share lives here
// once, so a screen renders the view, not re-computed snapshots.

export interface View {
  me: PlayerView | undefined;
  isHost: boolean;
  fight: State["fight"];
  isFighter: boolean;          // I'm in the current bout
  fighter: FighterView | null; // my fighter seat (a or b), null for spectators
  connected: PlayerView[];
  readyCount: number;          // connected players who passed (verdict/shop)
  meReady: boolean;
  canForge: boolean;           // my hand can cover any owned template
  phaseKey: string;            // changes on phase/round transitions — the reset
                               // token for local UI state (the stuck-forge class)
}

// Card kinds — mirrors internal/game's drawable kinds; every other string in
// a template's slots is a literal word.
const CARD_KIND: Record<string, true> = {
  adj: true, noun: true, verb: true, adv: true, pron: true, prep: true, conj: true, inter: true,
};
export const isLiteral = (slot: string) => !CARD_KIND[slot];

// canForge mirrors the server check: can the remaining hand satisfy any owned
// template? Each kind slot needs a distinct card — counts matter, not just
// membership. (Slots needing a literal are free.)
export function canForge(state: State): boolean {
  const counts: Record<string, number> = {};
  for (const c of state.hand) counts[c.kind] = (counts[c.kind] ?? 0) + 1;
  return state.templates.some((t) =>
    t.slots.every((s) => isLiteral(s) || (counts[s] ?? 0) >= t.slots.filter((x) => x === s).length),
  );
}

export function derive(state: State): View {
  const me = state.players.find((p) => p.id === state.you);
  const f = state.fight;
  const isFighter = !!f && (f.a.playerId === state.you || f.b.playerId === state.you);
  const fighter = !f ? null : f.a.playerId === state.you ? f.a : f.b.playerId === state.you ? f.b : null;
  const connected = state.players.filter((p) => p.connected);
  return {
    me,
    isHost: state.hostId === state.you,
    fight: f,
    isFighter,
    fighter,
    connected,
    readyCount: connected.filter((p) => p.ready).length,
    meReady: !!me?.ready,
    canForge: canForge(state),
    phaseKey: `${state.phase}:${state.round}`,
  };
}
