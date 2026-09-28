export type Phase = "lobby" | "draft" | "betting" | "combat" | "verdict" | "shop" | "over";

export type CardKind = "adj" | "noun" | "verb" | "adv" | "pron" | "prep" | "conj" | "inter";

export interface Card {
  id: string;
  text: string;
  kind: CardKind;
}

export interface Champion {
  name: string;
  cards: Card[]; // played cards, in order — rendered on the table
}

export interface ForgeTemplate {
  id: string;
  name: string;    // pattern, e.g. "__noun__ of __noun__"
  cost: number;    // 0 = starter
  example: string;
  slots: string[]; // card kinds or literal words ("of", "with"…)
}

export interface PlayerView {
  id: string;
  name: string;
  avatar: string;
  avatarUrl: string;
	coins: number;
	wins: number;
  connected: boolean;
  host: boolean;
  champion: Champion | null;
  drafted: boolean;
  handSize: number;
  ready: boolean; // agreed to continue (verdict/shop)
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

export interface MoveView {
  round: number;
  verb: string;
  fate: boolean;
}

export interface FighterView {
  playerId: string;
  name: string;
  avatarUrl: string;
  champion: Champion | null; // null while the fighter is still drafting
  moves: MoveView[];
}

export interface EventView {
  text: string;
  imageUrl: string;
}

export interface RoundEventView {
  round: number;
  text: string;
  imageUrl: string;
  decided: boolean;
  winnerId: string;
}

export interface VerdictView {
  winnerId: string;
  reason: string;
  events: EventView[];
}

export interface FightView {
  a: FighterView;
  b: FighterView;
  location: string;
  scene: string;
  sceneImage: string;
  round: number;
  fate: Record<string, number>;
  resolving: boolean;
  events: RoundEventView[];
  draw: boolean;
  myOptions: string[];
  readyAt: number; // unix ms — round-1 actions unlock
  verdict: VerdictView | null;
}

export interface State {
  type: "state";
  room: string;
  phase: Phase;
  round: number;
  you: string;
  hostId: string;
  level: string; // "middle" | "high" | "college"
  players: PlayerView[];
  bets: BetView[];
  winnerId: string; // set when phase === "over"
  fight: FightView | null;
  hand: Card[];
  templates: ForgeTemplate[]; // owned forge templates
  shop: ForgeTemplate[];      // buyable, only in shop phase
  shopTurn: string;           // player whose buy turn is live
  packCost: number;           // coins per 3-card pack
  locOptions: string[];
  locVotes: Record<string, number>;
  myVote: string;
}

export interface ServerMsg extends State {
  message?: string;
}
