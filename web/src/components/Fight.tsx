import { useState } from "react";
import type { Send } from "../services/socket";
import type { State } from "./types";
import { Arena, BeatDeck, FadeImg, FighterCard, nameOf } from "./shared";

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
      {(f.scene || f.sceneImage) && (
        <figure className="beat">
          <FadeImg src={f.sceneImage} alt="the arena" ph="🎨 painting the scene…" />
          {f.scene && <figcaption>{f.scene}</figcaption>}
        </figure>
      )}
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
  return (
    <section className="pane battle">
      <header className="bhead">
        <h2>Fight! — Round {f.round}/3</h2>
        {f.location && <span className="locbanner">📍 {f.location}</span>}
      </header>
      <div className="bcols">
        <div className="bmain">
          {!hasBeats && f.scene && (
            <figure className="beat">
              <FadeImg src={f.sceneImage} alt="the arena" ph="the judges are painting the arena…" />
              <figcaption>{f.scene}</figcaption>
            </figure>
          )}
          {hasBeats ? <BeatDeck events={f.events} /> : !f.scene && <div className="beatph big"><div className="paintspin">🎬</div><span>the judges are setting the scene…</span></div>}
          {f.resolving && <p className="sub">⚔️ the judges are scoring round {f.round}…</p>}
        </div>
        <aside className="blog">
          <Arena state={state} />
          <div className="acts">
            {!f.resolving && isFighter && !acted && (
              <>
                {f.myOptions && f.myOptions.length > 0 ? (
                  <>
                    <p className="sub">Pick {me!.champion?.name ?? me!.name}'s move — or let fate decide:</p>
                    {f.myOptions.map((o, i) => (
                      <button key={i} className="opt" onClick={() => send({ type: "verb", verb: o })}>
                        {o}
                      </button>
                    ))}
                  </>
                ) : (
                  <p className="sub">🎬 the judges are drafting your moves…</p>
                )}
                <button className="fate" onClick={() => send({ type: "pass" })}>🎲 Let fate decide</button>
              </>
            )}
            {!f.resolving && isFighter && acted && (
              <p className="sub">
                {f.fate?.[you] === f.round ? "🎲 Fate will decide." : "Move committed — waiting on your opponent."}
              </p>
            )}
            {!f.resolving && !isFighter && <p className="sub">The battle rages…</p>}
          </div>
        </aside>
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
      <header className="bhead">
        <h2>Verdict</h2>
        {f.location && <span className="locbanner">📍 {f.location}</span>}
      </header>
      <div className="bcols">
        <div className="bmain scroll">
          <BeatDeck events={f.events} />
        </div>
        <aside className="blog">
          <Arena state={state} />
          {v && (
            <div className="verdict">
              {f.draw ? <h3>🤝 Draw!</h3> : <h3>🏆 {winnerName} wins!</h3>}
              <p>{v.reason}</p>
              <button className="cta" disabled={meReady} onClick={() => send({ type: "pass" })}>
                {meReady ? "Waiting for others…" : "Continue →"}
              </button>
              <p className="sub">{readyCount}/{connected.length} ready</p>
            </div>
          )}
        </aside>
      </div>
    </section>
  );
}
