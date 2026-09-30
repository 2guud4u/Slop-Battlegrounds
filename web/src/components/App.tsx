import { useEffect, useRef, useState } from "react";
import { connect, type JoinOpts, type Send } from "../services/socket";
import type { State } from "./types";
import { Rail } from "./shared";
import { JoinScreen, Lobby } from "./Lobby";
import { Draft, HandDock } from "./Draft";
import { Betting, Combat, VerdictView } from "./Fight";
import { GameOver, ShopView } from "./Shop";

// Seat key saved per tab so a reload reclaims the same seat (hand, champion,
// coins) — the server holds a dropped seat for a few seconds.
const SEAT_KEY = "slop.seat";
type Seat = { room: string; token: string };

function savedSeat(): Seat | null {
  try {
    const s = JSON.parse(sessionStorage.getItem(SEAT_KEY) ?? "null");
    return s?.room && s?.token ? s : null;
  } catch {
    return null;
  }
}

export default function App() {
  const [state, setState] = useState<State | null>(null);
  const [err, setErr] = useState("");
  const sendRef = useRef<Send>(() => {});
  const [joined, setJoined] = useState(false);
  const [rejoining, setRejoining] = useState(() => !!savedSeat());

  const join = (opts: JoinOpts) => {
    setErr("");
    sendRef.current = connect(
      opts,
      (s) => {
        sessionStorage.setItem(SEAT_KEY, JSON.stringify({ room: s.room, token: s.token }));
        setState(s);
        setJoined(true);
        setRejoining(false);
      },
      (msg) => {
        // The saved seat is gone (room closed / server restarted) — start fresh.
        if (opts.token) sessionStorage.removeItem(SEAT_KEY);
        setRejoining(false);
        setErr(opts.token ? "" : msg);
      },
      () => setJoined(false),
    );
  };

  const autoJoined = useRef(false);
  useEffect(() => {
    const seat = savedSeat();
    if (!seat || autoJoined.current) return; // StrictMode runs effects twice in dev
    autoJoined.current = true;
    join({ name: "", room: seat.room, token: seat.token });
  }, []);

  const send = (msg: Record<string, unknown>) => sendRef.current(msg);

  if (!joined || !state) {
    if (rejoining) return <div className="landing"><div className="join"><p className="sub">Reclaiming your seat…</p></div></div>;
    return <JoinScreen onJoin={join} error={err} />;
  }
  const me = state.players.find((p) => p.id === state.you);
  const isHost = state.hostId === state.you;
  const inFight = state.phase === "combat" || state.phase === "verdict";

  return (
    <div className={"app" + (state.phase === "lobby" ? " backdrop" : "")}>
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
