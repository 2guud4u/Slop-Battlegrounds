import type { State } from "../components/types";

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

// Connect opens the socket, sends the join message, and calls back with
// every state update. Returns a send function once joined.
export function connect(
  opts: JoinOpts,
  onState: (s: State) => void,
  onError: (msg: string) => void,
  onClose: () => void,
): Send {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws`);

  ws.onopen = () => {
    ws.send(JSON.stringify({ type: "join", ...opts }));
  };
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
