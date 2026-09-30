import { useState } from "react";
import type { Send } from "../services/socket";
import type { State } from "./types";
import { BeatDeck, Billing, FighterCard, nameOf, Theater } from "./shared";

export function Betting({ state, you, send }: { state: State; you: string; send: Send }) {
  const f = state.fight;
  const [amount, setAmount] = useState(10);
  if (!f) return null;
  const isFighter = f.a.playerId === you || f.b.playerId === you;
  const myBet = state.bets.find((b) => b.bettorId === you);
  const bettors = state.players.filter((p) => p.id !== f.a.playerId && p.id !== f.b.playerId);
  const doneCount = state.bets.length;

  return (
    <section className="pane">
      <h2>Betting</h2>
      {f.location && <p className="locbanner">📍 {f.location}</p>}
      <Theater src={f.sceneImage} caption={f.scene} closedLabel={f.scene ? "the stagehands are painting the set…" : "the judges are writing the opening act…"} />
      <div className="arena">
        <FighterCard f={f.a} />
        <div className="vs">VS</div>
        <FighterCard f={f.b} />
      </div>
      {isFighter ? (
        <p className="sub">You're fighting — spectators are placing bets.</p>
      ) : myBet ? (
        <p>
          Bet placed{myBet.pass ? " (passed)" : `: ${myBet.amount}🪙 on ${nameOf(state, myBet.on)}`}. Waiting… ({doneCount}/{bettors.length})
        </p>
      ) : (
        <div className="betslip">
          <input type="number" min={10} value={amount} onChange={(e) => setAmount(Number(e.target.value))} />
          <button onClick={() => send({ type: "bet", on: f.a.playerId, amount })}>
            Bet {f.a.champion?.name ?? f.a.name}
          </button>
          <button onClick={() => send({ type: "bet", on: f.b.playerId, amount })}>
            Bet {f.b.champion?.name ?? f.b.name}
          </button>
          <button className="link" onClick={() => send({ type: "pass" })}>Pass</button>
        </div>
      )}
    </section>
  );
}

export function Combat({ state, you, send }: { state: State; you: string; send: Send }) {
  const f = state.fight;
  if (!f) return null;
  const isFighter = f.a.playerId === you || f.b.playerId === you;
  const me = isFighter ? (f.a.playerId === you ? f.a : f.b) : null;
  const acted = !!me && me.moves.some((m) => m.round === f.round);
  const hasBeats = f.events.length > 0;
  const scoring = `the judges are scoring act ${f.round}…`;

  // What sits in the wings — the fighter's hand of moves, or a stage note.
  const wings = (
    <>
      {!f.resolving && isFighter && !acted && (
        <>
          <p className="cue">
            {f.myOptions && f.myOptions.length > 0
              ? `Your cue — play ${me!.champion?.name ?? me!.name}'s move:`
              : "🎬 the judges are drafting your moves…"}
          </p>
          <div className="acts">
            {f.myOptions?.map((o, i) => (
              <button key={i} className="opt" onClick={() => send({ type: "verb", verb: o })}>
                {o}
              </button>
            ))}
            <button className="opt fate" onClick={() => send({ type: "pass" })}>
              🎲 Let fate decide
            </button>
          </div>
        </>
      )}
      {!f.resolving && isFighter && acted && (
        <p className="cue">{f.fate?.[you] === f.round ? "🎲 Fate will decide." : "Move committed — waiting on your opponent."}</p>
      )}
      {!f.resolving && !isFighter && <p className="cue">The battle rages…</p>}
      {f.resolving && <p className="cue">⚔️ {scoring}</p>}
    </>
  );

  return (
    <section className="pane battle">
      <Billing
        state={state}
        center={<span className="billact">Act {f.round} of 3{f.location && <small>📍 {f.location}</small>}</span>}
      />
      <div className="bstage">
        {hasBeats ? (
          <BeatDeck events={f.events} hold={f.resolving} closedLabel={f.resolving ? scoring : undefined} side={wings} />
        ) : (
          <Theater
            src={f.sceneImage}
            caption={f.scene}
            hold={f.resolving}
            closedLabel={f.resolving ? scoring : f.scene ? "the stagehands are painting the set…" : "the judges are writing the opening act…"}
            side={wings}
          />
        )}
      </div>
    </section>
  );
}

export function VerdictView({ state, send }: { state: State; send: Send }) {
  const f = state.fight;
  if (!f) return null;
  const v = f.verdict;
  const winnerName = v && v.winnerId ? nameOf(state, v.winnerId) : "";
  const connected = state.players.filter((p) => p.connected);
  const readyCount = connected.filter((p) => p.ready).length;
  const meReady = !!state.players.find((p) => p.id === state.you)?.ready;

  return (
    <section className="pane battle">
      <Billing
        state={state}
        center={<span className="billact">Verdict{f.location && <small>📍 {f.location}</small>}</span>}
      />
      <div className="bstage">
        <BeatDeck
          events={f.events}
          side={
            v && (
              <div className="verdict">
                {f.draw ? <h3>🤝 Draw!</h3> : <h3>🏆 {winnerName} wins!</h3>}
                <p>{v.reason}</p>
                <button className="cta" disabled={meReady} onClick={() => send({ type: "pass" })}>
                  {meReady ? "Waiting for others…" : "Continue →"}
                </button>
                <p className="sub">{readyCount}/{connected.length} ready</p>
              </div>
            )
          }
        />
      </div>
    </section>
  );
}
