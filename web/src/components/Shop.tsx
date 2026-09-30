import type { Send } from "../services/socket";
import type { State } from "./types";
import { Pattern } from "./shared";

// Card packs available in the shop — one per drawable kind.
const packKinds: { kind: string; label: string; icon: string }[] = [
  { kind: "noun", label: "Nouns", icon: "🂡" },
  { kind: "adj", label: "Adjectives", icon: "🅰️" },
  { kind: "verb", label: "Verbs", icon: "⚡" },
  { kind: "adv", label: "Adverbs", icon: "💨" },
  { kind: "pron", label: "Pronouns", icon: "👤" },
  { kind: "prep", label: "Prepositions", icon: "📍" },
  { kind: "conj", label: "Conjunctions", icon: "🔗" },
];

export function ShopView({ state, send }: { state: State; send: Send }) {
  const me = state.players.find((p) => p.id === state.you);
  const owned = new Set(state.templates.map((t) => t.id));
  const coins = me?.coins ?? 0;
  const over = state.hand.length - 7; // >0 → discard mode
  const done = !!me?.ready;
  const waiting = state.players.filter((p) => p.connected && !p.ready);
  return (
    <section className="pane">
      <h2>Forge Template Shop</h2>
      <p className="sub">
        {over > 0
          ? `Hand's ${state.hand.length}/7 — click ${over} card${over > 1 ? "s" : ""} below to discard.`
          : done
            ? `Cashed out (+2 cards). Waiting for ${waiting.map((p) => p.name).join(", ")}…`
            : "Everyone shops at once — templates are one-of-a-kind, first buy wins."}
      </p>
      <p className="coins big">{coins}🪙</p>

      {over > 0 ? (
        <div className="hand discard">
          {state.hand.map((c) => (
            <button
              key={c.id}
              className={"card " + c.kind}
              data-kind={c.kind}
              onClick={() => send({ type: "discard", card: c.id })}
            >
              {c.text}
            </button>
          ))}
        </div>
      ) : !done && (
        <>
          <h3 className="shoph">Card packs — {state.packCost}🪙 for 3 cards (hand {state.hand.length}/7)</h3>
          <div className="shopgrid packs">
            {packKinds.map((p) => (
              <button
                key={p.kind}
                className={"pack " + p.kind}
                disabled={coins < state.packCost}
                onClick={() => send({ type: "buy", pack: p.kind })}
              >
                <span className="pkicon">{p.icon}</span>
                <b>{p.label}</b>
                <small>3× {p.label.toLowerCase()}</small>
              </button>
            ))}
          </div>

          <h3 className="shoph">Forge templates — one copy each, first come first served</h3>
          <div className="shopgrid">
            {state.shop.filter((t) => t.cost > 0).map((t) => (
              <div className={"tmpl" + (owned.has(t.id) ? " owned" : "") + (t.sold && !owned.has(t.id) ? " soldout" : "")} key={t.id}>
                <div className="patline"><Pattern name={t.name} /></div>
                <em className="ex">e.g. “{t.example}”</em>
                {owned.has(t.id) ? (
                  <span className="ownedtag">owned</span>
                ) : t.sold ? (
                  <span className="soldtag">sold</span>
                ) : (
                  <button
                    disabled={coins < t.cost}
                    onClick={() => send({ type: "buy", template: t.id })}
                  >
                    Buy — {t.cost}🪙
                  </button>
                )}
              </div>
            ))}
          </div>
          <button className="forge" onClick={() => send({ type: "pass" })}>
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
