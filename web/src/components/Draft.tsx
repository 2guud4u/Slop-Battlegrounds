import { useEffect, useState, type CSSProperties } from "react";
import { useNow } from "../hooks/useNow";
import type { Send } from "../services/socket";
import type { Card, State } from "./types";
import { canForge, isLiteral, Pattern, ChampionCard } from "./shared";

export function Draft({ state, me, send }: { state: State; me: string; send: Send }) {
  const meP = state.players.find((p) => p.id === me);
  const f = state.fight;
  const isFighter = !!f && (f.a.playerId === me || f.b.playerId === me);
  const done = !!meP?.champion;
  const broke = isFighter && !done && !canForge(state); // hand can't cover any owned template
  const [forging, setForging] = useState(false);
  const now = useNow(state.revealAt ? 200 : 0);
  const revealing = !!f && state.revealAt > now;

  useEffect(() => {
    if (done) {
      setForging(true);
      const t = setTimeout(() => setForging(false), 1600);
      return () => clearTimeout(t);
    }
    setForging(false);
  }, [done]);

  // Tick while the reveal window is open — the stage steps on its own.

  return (
    <section className="pane">
      <div className="draftcols">
        <div className="draftmain">
          <h2>
            {f ? `⚔️ ${f.a.name} vs ${f.b.name}` : "Draft your champion"}
          </h2>
          {revealing ? (
            <Reveal f={f!} until={state.revealAt} now={now} />
          ) : (
            <>
              <p className="sub">
                {done
                  ? `Champion forged — waiting for your opponent…`
                  : !isFighter
                    ? "You're the audience this bout — vote for the battleground, bets open after both champions are forged."
                    : broke
                      ? "Out of cards — you can't forge. You sit this one out."
                      : "Play any cards to the table — left to right is the name."}
              </p>
              {done &&
                (forging ? (
                  <ForgeAnim champ={meP!.champion!} />
                ) : (
                  <ChampionCard champ={meP!.champion!} label="Your champion" hero />
                ))}
              <ChampionGallery state={state} />
            </>
          )}
        </div>
        <LocationVote state={state} send={send} />
      </div>
    </section>
  );
}

// LocationVote: spectators pick where the fight happens. Resolved at combat start.
function LocationVote({ state, send }: { state: State; send: Send }) {
  const opts = state.locOptions ?? [];
  const f = state.fight;
  const fighting = !!f && (f.a.playerId === state.you || f.b.playerId === state.you);
  if (fighting) return null;
  if (!opts.length)
    return (
      <div className="locvote">
        <h3>🗳️ Battleground</h3>
        <p className="sub">the judges are scouting venues…</p>
      </div>
    );
  return (
    <div className="locvote">
      <h3>🗳️ Battleground</h3>
      <p className="sub">Where does it go down?</p>
      {opts.map((o) => (
        <button
          key={o}
          className={"loc" + (state.myVote === o ? " sel" : "")}
          onClick={() => send({ type: "loc", loc: o })}
        >
          <span>{o}</span>
          <b>{state.locVotes?.[o] ?? 0}</b>
        </button>
      ))}
    </div>
  );
}

function ForgeAnim({ champ }: { champ: { name: string; cards: Card[] } }) {
  return (
    <div className="forgeanim">
      <div className="fa burst">{champ.name}</div>
      <div className="tabledcards">
        {champ.cards.map((c, i) => (
          <span key={c.id} className={"card ontable " + c.kind} data-kind={c.kind} style={{ animationDelay: `${i * 90}ms` }}>
            {c.text}
          </span>
        ))}
      </div>
    </div>
  );
}

// HandDock: forge templates on top, template slots on the table, hand below.
export function HandDock({ state, send }: { state: State; send: Send }) {
  const [tmplID, setTmplID] = useState("");
  const [filled, setFilled] = useState<Record<number, Card>>({});
  const [forging, setForging] = useState(false); // forge click → server ack
  const meP = state.players.find((p) => p.id === state.you);
  const drafted = !!meP?.champion;
  const draftMode = state.phase === "draft" && !drafted
    && !!state.fight && (state.fight.a.playerId === state.you || state.fight.b.playerId === state.you);

  if (!draftMode || !canForge(state)) return null; // only fighters forge
  const order: Record<string, number> = { noun: 0, pron: 1, adj: 2, verb: 3, adv: 4, prep: 5, conj: 6, inter: 7 };
  const tmpl = state.templates.find((t) => t.id === tmplID) ?? state.templates.find((t) => t.id === "adj_noun") ?? state.templates[0];
  const slots = tmpl?.slots ?? [];
  const usedIds = new Set(Object.values(filled).map((c) => c.id));
  const hand = [...state.hand].sort((a, b) => (order[a.kind] ?? 9) - (order[b.kind] ?? 9));
  const allFilled = slots.every((s, i) => isLiteral(s) || !!filled[i]);

  // fill: a card goes into the first empty slot of its kind
  const fill = (c: Card) => {
    if (forging) return;
    const idx = slots.findIndex((s, i) => s === c.kind && !filled[i]);
    if (idx < 0 || usedIds.has(c.id)) return;
    setFilled({ ...filled, [idx]: c });
  };
  const clear = (i: number) => {
    if (forging) return; // staged cards are locked while the forge flies
    const next = { ...filled };
    delete next[i];
    setFilled(next);
  };
  const pickTmpl = (id: string) => { if (forging) return; setTmplID(id); setFilled({}); };

  return (
    <div className={"dock" + (forging ? " forging" : "")}>
      <div className="tchips">
        {state.templates.map((t) => (
          <button
            key={t.id}
            className={"tchip" + (tmpl.id === t.id ? " sel" : "")}
            onClick={() => pickTmpl(t.id)}
            title={t.example}
          >
            <Pattern name={t.name} />
          </button>
        ))}
      </div>
      <div className="staged" aria-label="forge slots">
        {slots.map((s, i) => (
          <div className="slot" key={i}>
            {isLiteral(s) ? (
              <span className="lit">{s}</span>
            ) : filled[i] ? (
              <button
                className={"card ontable " + filled[i].kind}
                data-kind={filled[i].kind}
                onClick={() => clear(i)}
                title="return to hand"
              >
                {filled[i].text}
              </button>
            ) : (
              <div className={"slotph " + s} data-kind={s}>
                {s}
              </div>
            )}
          </div>
        ))}
      </div>
      <button
        className="forge"
        disabled={!allFilled || forging}
        onClick={() => {
          send({
            type: "draft",
            template: tmpl.id,
            cards: slots.map((s, i) => (isLiteral(s) ? null : filled[i].id)).filter((x): x is string => !!x),
          });
          setForging(true); // cards stay staged until the champion lands
        }}
      >
        {forging ? "⚒ Forging…" : "⚒ Forge champion"}
      </button>
      <div className="hand">
        {hand.map((c) => {
          const fits = slots.some((s, i) => s === c.kind && !filled[i]);
          const dead = usedIds.has(c.id) || !fits;
          return (
            <button
              key={c.id}
              className={"card " + c.kind + (dead ? " dead" : "")}
              data-kind={c.kind}
              disabled={dead}
              onClick={() => fill(c)}
            >
              {c.text}
            </button>
          );
        })}
      </div>
    </div>
  );
}

// Face-down until both fighters forge — then all revealed.
function ChampionGallery({ state }: { state: State }) {
  const drafted = state.players.filter((p) => p.champion && p.id !== state.you);
  if (!drafted.length) return null;
  const revealed = !!state.fight?.a.champion && !!state.fight.b.champion;
  return (
    <div className="gallery">
      {drafted.map((p) => (
        <div key={p.id}>
          {revealed ? (
            <ChampionCard champ={p.champion!} />
          ) : (
            <div className="champ"><div className="imgph">🂠</div><div><b>Forged</b></div></div>
          )}
          <small>{p.name}</small>
        </div>
      ))}
    </div>
  );
}

// Reveal: once both champions are forged the players take turns playing their
// real cards from the hand onto the table — a puff of smoke turns them into
// names, VS, then bets open.
function Reveal({ f, until, now }: { f: NonNullable<State["fight"]>; until: number; now: number }) {
  const left = until - now; // ms until the reveal ends
  const stage = left > 2500 ? "throw" : left > 1200 ? "smoke" : left > 500 ? "vs" : "fight";
  const rows = [f.a, f.b].map((fv) => fv.champion?.cards ?? []);
  // Turns alternate A, B, A, B… and squeeze into the ~2s the throw stage has.
  const lastTurn = Math.max(1, (rows[0].length - 1) * 2, (rows[1].length - 1) * 2 + 1);
  const stagger = Math.min(320, 1500 / lastTurn);
  return (
    <div className={"reveal " + stage}>
      <div className={"throw " + (stage === "throw" || stage === "smoke" ? "dealt" : "")}>
        {[f.a, f.b].map((fv, side) => (
          <div key={fv.playerId} className={"throwrow " + (side === 0 ? "top" : "bot")}>
            <span className="rowtag">{fv.name} plays</span>
            {rows[side].map((c, i) => (
              <span
                key={c.id}
                className={"card ontable playcard " + c.kind}
                data-kind={c.kind}
                style={playStyle(i, rows[side].length, side, c.id, (i * 2 + side) * stagger)}
              >
                {c.text}
                <span className="cardback" />
              </span>
            ))}
          </div>
        ))}
        {stage === "smoke" && (
          <span className="smokecloud">
            <span className="puff">💨</span><span className="puff d2">💨</span><span className="puff d3">💨</span>
          </span>
        )}
      </div>
      {stage === "smoke" && (
        <div className="smokename">
          <b>{f.a.champion?.name ?? f.a.name}</b>
          <b>{f.b.champion?.name ?? f.b.name}</b>
        </div>
      )}
      {stage === "vs" && (
        <div className="vsbanner">
          <span className="sa">{f.a.champion?.name ?? f.a.name}</span>
          <span className="svs">VS</span>
          <span className="sb">{f.b.champion?.name ?? f.b.name}</span>
        </div>
      )}
      {stage === "fight" && <div className="bellcd">FIGHT!</div>}
    </div>
  );
}

// playStyle places a card's start pose in its owner's fanned hand (above the
// table for A, below for B) and gives it a small, stable landing tilt.
function playStyle(i: number, n: number, side: number, id: string, delay: number): CSSProperties {
  const mid = (n - 1) / 2;
  const slot = 104; // card width + gap on the table
  let h = 0;
  for (const ch of id) h = (h * 31 + ch.charCodeAt(0)) | 0;
  const dir = side === 0 ? -1 : 1;
  return {
    animationDelay: `${delay}ms`,
    "--sx": `${(mid - i) * slot + (i - mid) * 22}px`, // hand is bunched at row center
    "--sy": `${dir * 190}px`,
    "--fan": `${dir * (i - mid) * -9}deg`, // fanned like a held hand
    "--arc": `${dir * -18}px`, // flight overshoots toward the table
    "--tilt": `${(Math.abs(h) % 7) - 3}deg`,
  } as CSSProperties;
}
