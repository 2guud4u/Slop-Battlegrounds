import type { AvatarInfo, State } from "../components/types";

export interface JoinOpts {
  room?: string;
  name: string;
  avatar?: string;
  token?: string; // seat key from a previous session
  llmProvider?: string;
  llmKey?: string;
  imageProvider?: string;
  imageAccount?: string;
  imageToken?: string;
  imageKey?: string;
}

// Session is the room's client protocol: one typed verb per game action.
// Components speak verbs, never wire shapes — the JSON field names live here.
export interface Session {
  start(): void;
  draft(template: string, cards: string[]): void;
  bet(on: string, amount: number): void;
  verb(verb: string): void;
  pass(): void;
  buyPack(kind: string): void;
  buyTemplate(id: string): void;
  discard(cardId: string): void;
  vote(loc: string): void;
  setName(name: string): void;
  setAvatar(key: string): void;
  setLevel(level: string): void;
}

// The join screen pre-opens the socket so clicking Create/Join costs one
// message instead of a TCP + upgrade handshake first.
let warm: WebSocket | null = null;

function wsURL() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${location.host}/ws`;
}

export function preconnect() {
  if (warm && warm.readyState <= WebSocket.OPEN) return;
  warm = new WebSocket(wsURL());
}

// Connect sends the join message on the warm socket (or a fresh one), and
// calls back with every state update. Returns the session's verbs.
export function connect(
  opts: JoinOpts,
  onState: (s: State) => void,
  onError: (msg: string) => void,
  onClose: () => void,
): Session {
  const reuse = warm && warm.readyState <= WebSocket.OPEN;
  const ws = reuse ? warm! : new WebSocket(wsURL());
  warm = null; // one join per socket — the server closes it on a failed join
  const join = () => ws.send(JSON.stringify({ type: "join", ...opts }));

  if (ws.readyState === WebSocket.OPEN) join();
  else ws.onopen = join;
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    if (m.type === "error") {
      onError(m.message ?? "error");
      return;
    }
    if (m.type === "state") onState(m as State);
  };
  ws.onclose = () => onClose();
  ws.onerror = () => onError("connection failed");

  const send = (msg: Record<string, unknown>) => {
    if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
  };

  return {
    start: () => send({ type: "start" }),
    draft: (template, cards) => send({ type: "draft", template, cards }),
    bet: (on, amount) => send({ type: "bet", on, amount }),
    verb: (verb) => send({ type: "verb", verb }),
    pass: () => send({ type: "pass" }),
    buyPack: (pack) => send({ type: "buy", pack }),
    buyTemplate: (template) => send({ type: "buy", template }),
    discard: (card) => send({ type: "discard", card }),
    vote: (loc) => send({ type: "loc", loc }),
    setName: (name) => send({ type: "name", name }),
    setAvatar: (avatar) => send({ type: "avatar", avatar }),
    setLevel: (level) => send({ type: "level", level }),
  };
}

// Avatar list is static — start fetching at page load (and warm the image
// cache) so the lobby grid doesn't wait extra round trips after the join lands.
let avatarList: Promise<AvatarInfo[]> | null = null;
export function fetchAvatars(): Promise<AvatarInfo[]> {
  avatarList ??= fetch("/api/avatars")
    .then((r) => r.json() as Promise<AvatarInfo[]>)
    .then((list) => {
      for (const a of list) new Image().src = a.url;
      return list;
    })
    .catch(() => {
      avatarList = null; // let the next caller retry
      return [];
    });
  return avatarList;
}
