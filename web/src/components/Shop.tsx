import type { CSSProperties } from "react";
import type { Session } from "../services/socket";
import type { State } from "./types";
import { Pattern } from "./shared";

// Card packs available in the shop — one per drawable kind. `pk` is the
// kind-colored edge on the top card of each pack stack.
const packKinds: { kind: string; label: string; pk: string }[] = [
  { kind: "noun", label: "Nouns", pk: "#1e7d5a" },
  { kind: "adj", label: "Adjectives", pk: "#6a4fd8" },
  { kind: "verb", label: "Verbs", pk: "#b0304a" },
  { kind: "adv", label: "Adverbs", pk: "#c07f1a" },
  { kind: "pron", label: "Pronouns", pk: "#2376c8" },
  { kind: "prep", label: "Prepositions", pk: "#7a7a5a" },
  { kind: "conj", label: "Conjunctions", pk: "#8a5ac0" },
];

export function ShopView({ state, session }: { state: State; session: Session }) {
  const me = state.players.find((p) => p.id === state.you);
  const owned = new Set(state.templates.map((t) => t.id));
  const coins = me?.coins ?? 0;
  const over = state.hand.length - state.handMax; // >0 → discard mode
  const done = !!me?.ready;
  const waiting = state.players.filter((p) => p.connected && !p.ready);
  return (
    <section className="pane cage">
      <div className="bankroll"><span>Bankroll:</span> <b>{coins}🪙</b> <span className="sub">· packs {state.packCost}🪙 · hand {state.hand.length}/{state.handMax}</span></div>
      <h2 className="cagetitle">The Cage — template window</h2>
      <p className="sub">
        {over > 0
          ? `Hand's ${state.hand.length}/7 — click ${over} card${over > 1 ? "s" : ""} below to discard.`
          : done
            ? `Cashed out (+2 cards). Waiting for ${waiting.map((p) => p.name).join(", ")}…`
            : "Everyone shops at once — templates are one-of-a-kind, first buy wins."}
      </p>
      {over > 0 ? (
        <div className="hand discard">
          {state.hand.map((c) => (
            <button
              key={c.id}
              className={"card " + c.kind}
              data-kind={c.kind}
              onClick={() => session.discard(c.id)}
            >
              {c.text}
            </button>
          ))}
        </div>
      ) : !done && (
        <>
          <h3 className="shoph">Card packs — {state.packCost}🪙 for 3 cards (hand {state.hand.length}/{state.handMax})</h3>
          <div className="shopgrid packs">
            {packKinds.map((p) => (
              <button
                key={p.kind}
                className={"pack " + p.kind}
                disabled={coins < state.packCost}
                onClick={() => session.buyPack(p.kind)}
              >
                <span className="packstack" style={{ "--pk": p.pk } as CSSProperties}>
                  <span className="mini" />
                  <span className="mini" />
                  <span className="mini" />
                  <span className="count">3</span>
                </span>
                <small>{p.label}</small>
                <span className="pkprice">{state.packCost}🪙</span>
              </button>
            ))}
          </div>

          <h3 className="shoph">Forge templates — one copy each, first come first served</h3>
          <div className="shopgrid">
            {(state.shop ?? []).filter((t) => t.cost > 0).map((t) => (
              <div className={"voucher" + (owned.has(t.id) ? " owned" : "") + (t.sold && !owned.has(t.id) ? " soldout" : "")} key={t.id}>
                <span className="price">{t.cost}🪙</span>
                <div className="patline"><Pattern name={t.name} /></div>
                <em className="ex">e.g. “{t.example}”</em>
                {owned.has(t.id) ? (
                  <span className="ownedtag">owned</span>
                ) : t.sold ? (
                  <span className="soldtag">sold</span>
                ) : (
                  <button
                    disabled={coins < t.cost}
                    onClick={() => session.buyTemplate(t.id)}
                  >
                    Buy
                  </button>
                )}
              </div>
            ))}
          </div>
          <button className="forge" onClick={() => session.pass()}>
            Done shopping → +2 cards
          </button>
        </>
      )}
    </section>
  );
}

// GameOver: cards are gone — richest player takes the room.
export function GameOver({ state }: { state: State }) {
  const standings = [...state.players].sort((a, b) => b.coins - a.coins);
  const winner = state.players.find((p) => p.id === state.winnerId);
  return (
    <section className="pane">
      <h2>👑 {winner ? `${winner.name} takes the room!` : "Game over"}</h2>
      <p className="sub">The decks ran dry — nobody could field two champions.</p>
      <ol className="standings">
        {standings.map((p, i) => (
          <li key={p.id} className={p.id === state.winnerId ? "first" : ""}>
            <span>{i + 1}.</span>
            <b>{p.name}</b>
            <span className="sub">{p.coins}🪙 · {p.wins}🏆</span>
          </li>
        ))}
      </ol>
    </section>
  );
}
