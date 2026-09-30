import { useState, type ReactNode } from "react";
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

// Theater: the fight is a play. A gold proscenium frames the generated image;
// velvet curtains stay shut until the image has actually decoded (or while the
// judges deliberate), then part. Narration runs as a playbill line under the
// stage; anything passed as children sits on the apron in front of it.
export function Theater({
  src,
  caption,
  closedLabel,
  hold,
  children,
}: {
  src?: string;
  caption?: string;
  closedLabel: string;
  hold?: boolean; // force the curtains shut (judges scoring, etc.)
  children?: ReactNode;
}) {
  const [loadedSrc, setLoadedSrc] = useState("");
  const open = !!src && loadedSrc === src && !hold;
  return (
    <div className="theater">
      <div className="proscenium">
        <div className="valance" />
        <div className="stageview">
          {src && <img key={src} src={src} alt="" onLoad={() => setLoadedSrc(src)} />}
          <div className={"curtain left" + (open ? " open" : "")} />
          <div className={"curtain right" + (open ? " open" : "")} />
          {!open && (
            <div className="curtaincall">
              <span className="spot">🎭</span>
              <span>{closedLabel}</span>
            </div>
          )}
        </div>
        <div className="footlights" />
      </div>
      {(caption || children) && (
        <div className="apron">
          {children}
          {caption && <p className={"playbill" + (open ? " lit" : "") + (children ? " after" : "")}>{caption}</p>}
        </div>
      )}
    </div>
  );
}

// BeatDeck: one beat at a time on the theater stage, with prev/next.
export function BeatDeck({ events, hold, closedLabel, children }: { events: RoundEventView[]; hold?: boolean; closedLabel?: string; children?: ReactNode }) {
  const [idx, setIdx] = useState(-1); // -1 = latest
  const cur = idx === -1 || idx >= events.length ? events.length - 1 : idx;
  const ev = events[cur];
  if (!ev) return null;
  return (
    <>
      <Theater src={ev.imageUrl} caption={ev.text} hold={hold} closedLabel={closedLabel ?? `painting act ${ev.round}…`}>
        {children}
      </Theater>
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
