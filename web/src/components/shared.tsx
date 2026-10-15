import { useState, type ReactNode } from "react";
import type { Card, FighterView, RoundEventView, State } from "./types";

// canForge / isLiteral moved to ../view — re-exported for existing imports.
export { canForge, isLiteral } from "../view";

export function nameOf(s: State, id: string): string {
  return s.players.find((p) => p.id === id)?.name ?? id;
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

// Theater: the fight is a broadcast. A wood-console CRT hangs over the table
// on chains; while the judges work (`hold`, or the image hasn't decoded) the
// screen is static + a PLEASE STAND BY placard. When the art lands it zaps on
// like a channel switch. Narration runs under the TV; `side` (the fighter's
// moves, the verdict) sits beneath and is told whether the broadcast is live.
export function Theater({
  src,
  caption,
  closedLabel,
  hold,
  side,
}: {
  src?: string;
  caption?: string;
  closedLabel: string;
  hold?: boolean; // force static (judges scoring, etc.)
  side?: (broadcasting: boolean) => ReactNode;
}) {
  const [loadedSrc, setLoadedSrc] = useState("");
  const open = !!src && loadedSrc === src && !hold;
  return (
    <div className="theater">
      <div className="stagecol">
        <div className="cabinet">
          <div className="screen">
            {src && <img key={src} src={src} alt="" onLoad={() => setLoadedSrc(src)} />}
            <div className={"tvsnow" + (open ? " off" : "")} />
            {!open && (
              <div className="standby">
                <b>PLEASE STAND BY</b>
                <span>{closedLabel}</span>
              </div>
            )}
            {open && <span className="tvbug">SLOP·TV</span>}
          </div>
          <div className="tvpanel">
            <div className="knob" />
            <span className="chnum">CH 3</span>
            <div className="knob small" />
            <div className="speaker" />
          </div>
        </div>
        {caption && (
          <div className="apron">
            <p className={"playbill" + (open ? " lit" : "")}>{caption}</p>
          </div>
        )}
      </div>
      {side && <aside className="wings">{side(open)}</aside>}
    </div>
  );
}

// BeatDeck: one beat at a time on the TV, with prev/next.
export function BeatDeck({ events, hold, closedLabel, side }: { events: RoundEventView[]; hold?: boolean; closedLabel?: string; side?: (onair: boolean) => ReactNode }) {
  const [idx, setIdx] = useState(-1); // -1 = latest
  const cur = idx === -1 || idx >= events.length ? events.length - 1 : idx;
  const ev = events[cur];
  if (!ev) return null;
  return (
    <>
      <Theater src={ev.imageUrl} caption={ev.degraded ? `${ev.text} — 📻 the judges are unreachable` : ev.text} hold={hold} closedLabel={closedLabel ?? `filming act ${ev.round}…`} side={side} />
      {events.length > 1 && (
        <div className="beatnav">
          <button className="link" disabled={cur <= 0} onClick={() => setIdx(cur - 1)}>◀ act {cur}</button>
          <span className="sub">act {ev.round} of {events.length}</span>
          <button className="link" disabled={cur >= events.length - 1} onClick={() => setIdx(cur + 1)}>act {cur + 2} ▶</button>
        </div>
      )}
    </>
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

// Billing: the marquee over the stage — A on the left, VS medallion (with the
// act/location under it), B on the right, each with their latest move.
export function Billing({ state, center }: { state: State; center?: ReactNode }) {
  const f = state.fight;
  if (!f) return null;
  const side = (fv: FighterView, cls: string) => {
    const last = fv.moves[fv.moves.length - 1];
    return (
      <div className={"bill " + cls + (fv.playerId === state.you ? " me" : "")}>
        {fv.avatarUrl && <img className="av big" src={fv.avatarUrl} alt="" />}
        <div className="billtext">
          <small>{fv.name}{fv.playerId === state.you ? " (you)" : ""}</small>
          <b>{fv.champion?.name ?? "forging…"}</b>
          {last && <span className="lastmove">{last.fate ? "🎲" : "⚡"} {last.verb}</span>}
        </div>
      </div>
    );
  };
  return (
    <div className="billing">
      {side(f.a, "a")}
      <div className="billcenter">
        <div className="vsmedal">VS</div>
        {center}
      </div>
      {side(f.b, "b")}
    </div>
  );
}
