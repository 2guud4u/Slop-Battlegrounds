import { useRef, useState } from "react";
import { connect, type JoinOpts, type Send } from "../services/socket";
import type { State } from "./types";
import { Rail } from "./shared";
import { JoinScreen, Lobby } from "./Lobby";
import { Draft, HandDock } from "./Draft";
import { Betting, Combat, VerdictView } from "./Fight";
import { GameOver, ShopView } from "./Shop";

export default function App() {
  const [state, setState] = useState<State | null>(null);
  const [err, setErr] = useState("");
  const sendRef = useRef<Send>(() => {});
  const [joined, setJoined] = useState(false);

  const join = (opts: JoinOpts) => {
    setErr("");
    // Persist the lobby-picked name so a refresh reconnects the same seat.
    const named = { ...opts, name: opts.name || sessionStorage.getItem("slop.name") || "" };
    sendRef.current = connect(
      named,
      (s) => {
        setState(s);
        setJoined(true);
      },
      setErr,
      () => setJoined(false),
    );
  };

  const send = (msg: Record<string, unknown>) => {
    if (msg.type === "name" && typeof msg.name === "string" && msg.name) {
      sessionStorage.setItem("slop.name", msg.name);
    }
    sendRef.current(msg);
  };

  if (!joined || !state) {
    return <JoinScreen onJoin={join} error={err} />;
  }
  const me = state.players.find((p) => p.id === state.you);
  const isHost = state.hostId === state.you;
  const inFight = state.phase === "combat" || state.phase === "verdict";

  return (
    <div className="app">
      <header>
        <h1>⚔️ Slop Battlegrounds</h1>
        <span className="badge">
          room <b>{state.room}</b> · round {state.round} · {state.phase}
        </span>
      </header>
      <div className="frame">
        <Rail state={state} />
        <main className={"stage" + (inFight ? " fight" : "")}>
          {state.phase === "lobby" && <Lobby state={state} isHost={isHost} send={send} />}
          {state.phase === "draft" && me && <Draft state={state} me={me.id} send={send} />}
          {state.phase === "betting" && <Betting state={state} you={state.you} send={send} />}
          {state.phase === "combat" && <Combat state={state} you={state.you} send={send} />}
          {state.phase === "verdict" && <VerdictView state={state} send={send} />}
          {state.phase === "shop" && <ShopView state={state} send={send} />}
          {state.phase === "over" && <GameOver state={state} />}
        </main>
      </div>
      <HandDock state={state} send={send} />
      {err && <p className="err">{err}</p>}
    </div>
  );
}
