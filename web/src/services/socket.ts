import type { AvatarInfo, State } from "../components/types";

export interface JoinOpts {
  room?: string;
  name: string;
  avatar?: string;
  llmProvider?: string;
  llmKey?: string;
  imageProvider?: string;
  imageAccount?: string;
  imageToken?: string;
  imageKey?: string;
}

export type Send = (msg: Record<string, unknown>) => void;

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
// calls back with every state update. Returns a send function.
export function connect(
  opts: JoinOpts,
  onState: (s: State) => void,
  onError: (msg: string) => void,
  onClose: () => void,
): Send {
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

  return (msg) => ws.readyState === WebSocket.OPEN && ws.send(JSON.stringify(msg));
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
