export type Phase = "lobby" | "draft" | "betting" | "combat" | "verdict";

export interface Card {
  id: string;
  text: string;
  kind: "adj" | "noun" | "verb";
}

export interface Champion {
  adj: string;
  noun: string;
  imageUrl: string;
}

export interface PlayerView {
  id: string;
  name: string;
  avatar: string;
  avatarUrl: string;
  coins: number;
  connected: boolean;
  host: boolean;
  champion: Champion | null;
  drafted: boolean;
  handSize: number;
}

export interface AvatarInfo {
  key: string;
  name: string;
  url: string;
}
export interface BetView {
  bettorId: string;
  on: string;
  amount: number;
  pass: boolean;
}

export interface FighterView {
  playerId: string;
  name: string;
  avatarUrl: string;
  champion: Champion;
  verbs: string[];
}

export interface EventView {
  text: string;
  imagePrompt: string;
  imageUrl: string;
}

export interface VerdictView {
  winnerId: string;
  reason: string;
  events: EventView[];
}

export interface FightView {
  a: FighterView;
  b: FighterView;
  verdict: VerdictView | null;
}

export interface State {
  type: "state";
  room: string;
  phase: Phase;
  round: number;
  you: string;
  hostId: string;
  players: PlayerView[];
  bets: BetView[];
  fight: FightView | null;
  hand: Card[];
}

export interface ServerMsg extends State {
  message?: string;
}
