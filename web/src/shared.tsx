import { useEffect, useState } from "react";
import type { Card, FighterView, RoundEventView, State } from "./types";

// ---------- shared bits ----------

export function isLiteral(slot: string) {
  return !(["adj", "noun", "verb", "adv", "pron", "prep", "conj", "inter"] as string[]).includes(slot);
}

export function nameOf(s: State, id: string): string {
  return s.players.find((p) => p.id === id)?.name ?? id;
}

// canForge mirrors the server check: can the remaining hand satisfy any owned
// template? (Slots needing a literal are free.)
export function canForge(state: State): boolean {
  const counts: Record<string, number> = {};
  for (const c of state.hand) counts[c.kind] = (counts[c.kind] ?? 0) + 1;
  return state.templates.some((t) =>
    t.slots.every((s) => isLiteral(s) || (counts[s] ?? 0) >= t.slots.filter((x) => x === s).length),
  );
}

// Rail: vertical player strip — avatar, name, wins, coins, draft/fight status.
export function Rail({ state }: { state: State }) {
  const f = state.fight;
  return (
    <aside className="rail">
      {state.players.map((p) => {
        const fighting = !!f && (p.id === f.a.playerId || p.id === f.b.playerId);
        return (
          <div
            key={p.id}
            className={
              "rseat" +
              (p.connected ? "" : " off") +
              (p.id === state.you ? " me" : "") +
              (fighting ? " fighting" : "")
            }
          >
            {p.avatarUrl ? <img src={p.avatarUrl} alt="" /> : <div className="railph">?</div>}
            <div className="rmeta">
              <b>{p.host ? "👑 " : ""}{p.name}{p.id === state.you ? " (you)" : ""}</b>
              <span className="rrow">🏆 {p.wins} <span className="coinchip">{p.coins}</span></span>
              {state.phase === "draft" && <span className="rrow dim">{p.drafted ? "✅ forged" : "…"}</span>}
              {fighting && state.phase !== "lobby" && <span className="rrow dim">⚔️</span>}
            </div>
          </div>
        );
      })}
    </aside>
  );
}

// Pattern renders "__adj__ of __noun__" as colored kind chips + literal words.
export function Pattern({ name }: { name: string }) {
  return (
    <span className="pat">
      {name.split(" ").map((tok, i) => {
        const m = tok.match(/^__(\w+)__$/);
        return m ? (
          <span key={i} className={"kchip " + m[1]}>{m[1]}</span>
        ) : (
          <span key={i} className="plit">{tok}</span>
        );
      })}
    </span>
  );
}

// ChampionCard: the forged name + the actual cards that made it.
export function ChampionCard({ champ, label, hero }: { champ: { name: string; cards: Card[] }; label?: string; hero?: boolean }) {
  return (
    <div className={hero ? "champ hero" : "champ"}>
      <div>
        {label && <small>{label}</small>}
        <b>{champ.name}</b>
      </div>
    </div>
  );
}

// FadeImg: shimmer skeleton until the image actually decodes, then fades in.
export function FadeImg({ src, alt, ph }: { src: string; alt: string; ph: string }) {
  const [loaded, setLoaded] = useState(false);
  useEffect(() => setLoaded(false), [src]); // new image → re-skeleton
  if (!src) return <div className="beatskel"><div className="paintspin">🎨</div><span>{ph}</span></div>;
  return (
    <div className={"imgwrap" + (loaded ? " loaded" : "")}>
      <img src={src} alt={alt} onLoad={() => setLoaded(true)} />
      {!loaded && <div className="beatskel cover"><div className="paintspin">🎨</div><span>{ph}</span></div>}
    </div>
  );
}

// Full-width beat: generated fight image with narration under it.
export function Beat({ ev }: { ev: RoundEventView }) {
  return (
    <figure className="beat">
      <FadeImg src={ev.imageUrl} alt={`round ${ev.round}`} ph="painting the scene…" />
      <figcaption>{ev.text}</figcaption>
    </figure>
  );
}

// BeatDeck: one beat at a time with prev/next — a slideshow, not a stack.
export function BeatDeck({ events }: { events: RoundEventView[] }) {
  const [idx, setIdx] = useState(-1); // -1 = latest
  const cur = idx === -1 || idx >= events.length ? events.length - 1 : idx;
  const ev = events[cur];
  if (!ev) return null;
  return (
    <div>
      <Beat ev={ev} />
      {events.length > 1 && (
        <div className="beatnav">
          <button className="link" disabled={cur <= 0} onClick={() => setIdx(cur - 1)}>◀ prev round</button>
          <span className="sub">round {ev.round} of {events.length}</span>
          <button className="link" disabled={cur >= events.length - 1} onClick={() => setIdx(cur + 1)}>next round ▶</button>
        </div>
      )}
    </div>
  );
}

export function FighterCard({ f }: { f: FighterView }) {
  return (
    <div className="fighter">
      <div className="fighter-head">
        {f.avatarUrl && <img className="av big" src={f.avatarUrl} alt="" />}
        {f.champion ? <ChampionCard champ={f.champion} label={f.name} /> : <div className="sub">forging…</div>}
      </div>
      {f.moves.length > 0 && (
        <ul className="movelog">
          {f.moves.map((m, i) => (
            <li key={i}>{m.fate ? "🎲" : "⚡"} {m.verb}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function Arena({ state }: { state: State }) {
  const f = state.fight;
  if (!f) return null;
  return (
    <div className="arena slim">
      <FighterCard f={f.a} />
      <div className="vs">VS</div>
      <FighterCard f={f.b} />
    </div>
  );
}
