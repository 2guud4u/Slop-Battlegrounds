import { useEffect, useRef, useState } from "react";
import { connect, type JoinOpts, type Send } from "./ws";
import type { AvatarInfo, Card, FighterView, RoundEventView, State } from "./types";

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
          {state.phase === "shop" && <ShopView state={state} send={send} />}
          {state.phase === "over" && <GameOver state={state} />}
        </main>
      </div>
      <HandDock state={state} send={send} />
      {err && <p className="err">{err}</p>}
    </div>
  );
}

// Rail: vertical player strip — avatar, name, wins, coins, draft/fight status.
function Rail({ state }: { state: State }) {
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

// ---------- join / lobby ----------

function JoinScreen({ onJoin, error }: { onJoin: (o: JoinOpts) => void; error: string }) {
  const [room, setRoom] = useState("");
  const [showKeys, setShowKeys] = useState(false);
  const [joining, setJoining] = useState(false);
  const [llmKey, setLlmKey] = useState("");
  const [cfAccount, setCfAccount] = useState("");
  const [geminiKey, setGeminiKey] = useState("");
  const [cfToken, setCfToken] = useState("");

  // Failed joins land as errors — release the spinner so they can retry.
  useEffect(() => {
    if (error) setJoining(false);
  }, [error]);

  return (
    <div className="join">
      <h1>⚔️ Slop Battlegrounds</h1>
      <p className="sub">Build a champion. Judge by AI. Bet on slop.</p>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (joining) return;
          setJoining(true);
          onJoin({
            name: "",
            room: room.trim() || undefined,
            llmKey: llmKey.trim() || undefined,
            imageKey: geminiKey.trim() || undefined,
            imageAccount: cfAccount.trim() || undefined,
            imageToken: cfToken.trim() || undefined,
          });
        }}
      >
        <input placeholder="Room code (empty = create)" value={room} onChange={(e) => setRoom(e.target.value)} autoFocus />
        <button type="button" className="link" onClick={() => setShowKeys(!showKeys)}>
          {showKeys ? "▾ hide" : "▸"} host API keys (optional)
        </button>
        {showKeys && (
          <div className="keys">
            <p className="hint">
              Creating a room? Paste your own keys — otherwise the server defaults (or mocks) are used.
            </p>
            <input placeholder="Groq API key (LLM judge)" value={llmKey} onChange={(e) => setLlmKey(e.target.value)} />
            <input placeholder="Gemini API key (image gen — easiest)" value={geminiKey} onChange={(e) => setGeminiKey(e.target.value)} />
            <input placeholder="Cloudflare account ID (image gen)" value={cfAccount} onChange={(e) => setCfAccount(e.target.value)} />
            <input placeholder="Cloudflare API token (image gen)" value={cfToken} onChange={(e) => setCfToken(e.target.value)} />
          </div>
        )}
        <button type="submit" disabled={joining}>
          {joining ? (room.trim() ? "Joining room…" : "Creating room…") : room.trim() ? "Join room" : "Create room"}
        </button>
      </form>
      {error && <p className="err">{error}</p>}
    </div>
  );
}

function Lobby({ state, isHost, send }: { state: State; isHost: boolean; send: Send }) {
  const [avatars, setAvatars] = useState<AvatarInfo[]>([]);
  const me = state.players.find((p) => p.id === state.you);
  const [name, setName] = useState("");
  const taken: Record<string, string> = {};
  for (const p of state.players) if (p.avatar) taken[p.avatar] = p.name;

  useEffect(() => {
    fetch("/api/avatars").then((r) => r.json()).then(setAvatars).catch(() => {});
  }, []);
  useEffect(() => {
    if (me) setName(me.name);
  }, [me?.name]);

  const commitName = () => {
    const n = name.trim();
    if (n && n !== me?.name) send({ type: "name", name: n });
  };

  return (
    <section className="pane lobby">
      <h2>Lobby</h2>
      <p>Share room code <b>{state.room}</b> — need 2+ players.</p>
      <input
        className="namein"
        placeholder="Your name"
        value={name}
        maxLength={24}
        onChange={(e) => setName(e.target.value)}
        onBlur={commitName}
        onKeyDown={(e) => e.key === "Enter" && commitName()}
      />
      <p className="sub">Pick your manager:</p>
      <div className="avatars">
        {avatars.map((a) => {
          const mine = me?.avatar === a.key;
          const owner = taken[a.key];
          const locked = !mine && !!owner;
          return (
            <button
              key={a.key}
              className={"avatar" + (mine ? " sel" : "") + (locked ? " taken" : "")}
              title={locked ? `${a.name} — taken by ${owner}` : a.name}
              disabled={locked}
              onClick={() => send({ type: "avatar", avatar: a.key })}
            >
              <img src={a.url} alt={a.name} />
              <span>{a.name}</span>
              {locked && <span className="owner">@{owner}</span>}
            </button>
          );
        })}
      </div>
      <div className="lvlrow">
        <span className="sub">Story level:</span>
        {(["middle", "high", "college"] as const).map((lv) => (
          <button
            key={lv}
            className={"tchip" + (state.level === lv ? " sel" : "")}
            disabled={!isHost}
            title={isHost ? "set narration complexity" : "host sets this"}
            onClick={() => send({ type: "level", message: lv })}
          >
            {lv === "middle" ? "Middle School" : lv === "high" ? "High School" : "College"}
          </button>
        ))}
      </div>
      {isHost ? (
        <button className="cta" disabled={state.players.length < 2} onClick={() => send({ type: "start" })}>
          Start game
        </button>
      ) : (
        <p>Waiting for host to start…</p>
      )}
    </section>
  );
}

// ---------- draft ----------

function Draft({ state, me, send }: { state: State; me: string; send: Send }) {
  const meP = state.players.find((p) => p.id === me);
  const f = state.fight;
  const isFighter = !!f && (f.a.playerId === me || f.b.playerId === me);
  const done = !!meP?.champion;
  const broke = isFighter && !done && !canForge(state); // hand can't cover any owned template
  const [forging, setForging] = useState(false);

  useEffect(() => {
    if (done) {
      setForging(true);
      const t = setTimeout(() => setForging(false), 1600);
      return () => clearTimeout(t);
    }
    setForging(false);
  }, [done]);

  return (
    <section className="pane">
      <div className="draftcols">
        <div className="draftmain">
          <h2>
            {f ? `⚔️ ${f.a.name} vs ${f.b.name}` : "Draft your champion"}
          </h2>
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
        </div>
        <LocationVote state={state} send={send} />
      </div>
    </section>
  );
}

// canForge mirrors the server check: can the remaining hand satisfy any owned
// template? (Slots needing a literal are free.)
function canForge(state: State): boolean {
  const counts: Record<string, number> = {};
  for (const c of state.hand) counts[c.kind] = (counts[c.kind] ?? 0) + 1;
  return state.templates.some((t) =>
    t.slots.every((s) => isLiteral(s) || (counts[s] ?? 0) >= t.slots.filter((x) => x === s).length),
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
function HandDock({ state, send }: { state: State; send: Send }) {
  const [tmplID, setTmplID] = useState("");
  const [filled, setFilled] = useState<Record<number, Card>>({});
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
    const idx = slots.findIndex((s, i) => s === c.kind && !filled[i]);
    if (idx < 0 || usedIds.has(c.id)) return;
    setFilled({ ...filled, [idx]: c });
  };
  const clear = (i: number) => {
    const next = { ...filled };
    delete next[i];
    setFilled(next);
  };
  const pickTmpl = (id: string) => { setTmplID(id); setFilled({}); };

  return (
    <div className="dock">
      <div className="tchips">
        {state.templates.map((t) => (
          <button
            key={t.id}
            className={"tchip" + (tmpl.id === t.id ? " sel" : "")}
            onClick={() => pickTmpl(t.id)}
            title={t.example}
          >
            {t.name}
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
              <button className={"slotph " + s} onClick={() => clear(i)}>
                __{s}__
              </button>
            )}
          </div>
        ))}
      </div>
      <button
        className="forge"
        disabled={!allFilled}
        onClick={() => {
          send({
            type: "draft",
            template: tmpl.id,
            cards: slots.map((s, i) => (isLiteral(s) ? null : filled[i].id)).filter((x): x is string => !!x),
          });
          setFilled({});
        }}
      >
        ⚒ Forge champion
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

function isLiteral(slot: string) {
  return !(["adj", "noun", "verb", "adv", "pron", "prep", "conj", "inter"] as string[]).includes(slot);
}

// ChampionCard: the forged name + the actual cards that made it.
function ChampionCard({ champ, label, hero }: { champ: { name: string; cards: Card[] }; label?: string; hero?: boolean }) {
  return (
    <div className={hero ? "champ hero" : "champ"}>
      <div>
        {label && <small>{label}</small>}
        <b>{champ.name}</b>
      </div>
    </div>
  );
}

// ---------- shop ----------

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

function ShopView({ state, send }: { state: State; send: Send }) {
  const me = state.players.find((p) => p.id === state.you);
  const owned = new Set(state.templates.map((t) => t.id));
  const myTurn = state.shopTurn === state.you;
  const turnName = state.players.find((p) => p.id === state.shopTurn)?.name ?? "…";
  const coins = me?.coins ?? 0;
  const handFull = state.hand.length + 3 > 7;
  return (
    <section className="pane">
      <h2>Forge Template Shop</h2>
      <p className="sub">
        {myTurn
          ? "Your turn — buy templates and card packs, then finish."
          : `Waiting for ${turnName} to shop… (fewest wins goes first)`}
      </p>
      <p className="coins big">{coins}🪙</p>

      <h3 className="shoph">Card packs — {state.packCost}🪙 for 3 cards (hand {state.hand.length}/7)</h3>
      <div className="shopgrid packs">
        {packKinds.map((p) => (
          <button
            key={p.kind}
            className={"pack " + p.kind}
            disabled={!myTurn || coins < state.packCost || handFull}
            onClick={() => send({ type: "buy", pack: p.kind })}
          >
            <span className="pkicon">{p.icon}</span>
            <b>{p.label}</b>
            <small>{handFull ? "hand full" : `3× ${p.label.toLowerCase()}`}</small>
          </button>
        ))}
      </div>

      <h3 className="shoph">Forge templates — weirder grammar, richer champions</h3>
      <div className="shopgrid">
        {state.shop.filter((t) => t.cost > 0).map((t) => (
          <div className={"tmpl" + (owned.has(t.id) ? " owned" : "")} key={t.id}>
            <b className="pat">{t.name}</b>
            <em className="ex">e.g. “{t.example}”</em>
            {owned.has(t.id) ? (
              <span className="ownedtag">owned</span>
            ) : (
              <button
                disabled={!myTurn || coins < t.cost}
                onClick={() => send({ type: "buy", template: t.id })}
              >
                Buy — {t.cost}🪙
              </button>
            )}
          </div>
        ))}
      </div>
      <button className="forge" disabled={!myTurn} onClick={() => send({ type: "pass" })}>
        {myTurn ? "Done shopping →" : `${turnName} is shopping…`}
      </button>
    </section>
  );
}

// GameOver: cards are gone — richest player takes the room.
function GameOver({ state }: { state: State }) {
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

// ---------- betting ----------

function Betting({ state, you, send }: { state: State; you: string; send: Send }) {
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

function nameOf(s: State, id: string): string {
  return s.players.find((p) => p.id === id)?.name ?? id;
}

// ---------- combat ----------

function Arena({ state }: { state: State }) {
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

// Full-width beat: generated fight image with narration under it.
function Beat({ ev }: { ev: RoundEventView }) {
  return (
    <figure className="beat">
      <FadeImg src={ev.imageUrl} alt={`round ${ev.round}`} ph="painting the scene…" />
      <figcaption>{ev.text}</figcaption>
    </figure>
  );
}

// FadeImg: shimmer skeleton until the image actually decodes, then fades in.
function FadeImg({ src, alt, ph }: { src: string; alt: string; ph: string }) {
  const [loaded, setLoaded] = useState(false);
  useEffect(() => setLoaded(false), [src]); // new image → re-skeleton
  if (!src) return <div className="beatskel"><div className="paintspin">🎨</div><span>{ph}</span></div>;
  return (
    <div className={"imgwrap" + (loaded ? " loaded" : "")}>
      {!loaded && <div className="beatskel"><div className="paintspin">🎨</div><span>{ph}</span></div>}
      <img src={src} alt={alt} onLoad={() => setLoaded(true)} />
    </div>
  );
}

// BeatDeck: one beat at a time with prev/next — a slideshow, not a stack.
function BeatDeck({ events }: { events: RoundEventView[] }) {
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

function FighterCard({ f }: { f: FighterView }) {
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


// Reveal: the pre-bell show — both players' cards hit the table, a puff of
// smoke turns them into champions, names clash, then the bell. Buys the
// scene image a few more seconds.
function Reveal({ f, now }: { f: NonNullable<State["fight"]>; now: number }) {
  const left = f.readyAt - now; // ms until the bell
  const stage = left > 4000 ? "throw" : left > 2000 ? "smoke" : left > 1000 ? "vs" : "fight";
  return (
    <div className={"reveal " + stage}>
      {stage === "throw" && (
        <div className="throw">
          {[f.a, f.b].map((fv, side) => (
            <div key={fv.playerId} className={"throwrow " + (side === 0 ? "top" : "bot")}>
              {fv.champion?.cards?.map((c, i) => (
                <span
                  key={c.id}
                  className={"card mini " + c.kind}
                  data-kind={c.kind}
                  style={{ animationDelay: `${side * 900 + i * 140}ms` }}
                >
                  {c.text}
                </span>
              ))}
            </div>
          ))}
        </div>
      )}
      {stage === "smoke" && (
        <>
          <span className="puff">💨</span><span className="puff d2">💨</span><span className="puff d3">💨</span>
          <div className="smokename">
            <b>{f.a.champion?.name ?? f.a.name}</b>
            <b>{f.b.champion?.name ?? f.b.name}</b>
          </div>
        </>
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
function Combat({ state, you, send }: { state: State; you: string; send: Send }) {
  const f = state.fight;
  if (!f) return null;
  const isFighter = f.a.playerId === you || f.b.playerId === you;
  const me = isFighter ? (f.a.playerId === you ? f.a : f.b) : null;
  const acted = !!me && me.moves.some((m) => m.round === f.round);
  const hasBeats = f.events.length > 0;
  // Re-render around the bell — countdown must expire without a WS push.
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(t);
  }, []);
  const countdown = !hasBeats && f.readyAt > now; // round 1, pre-bell
  return (
    <section className="pane battle">
      <header className="bhead">
        <h2>Fight! — Round {f.round}/3</h2>
        {f.location && <span className="locbanner">📍 {f.location}</span>}
      </header>
      <div className="bcols">
        <div className="bmain">
          {countdown && <Reveal f={f} now={now} />}
          {!countdown && !hasBeats && f.scene && (
            <figure className="beat">
              <FadeImg src={f.sceneImage} alt="the arena" ph="the judges are painting the arena…" />
              <figcaption>{f.scene}</figcaption>
            </figure>
          )}
          {hasBeats ? <BeatDeck events={f.events} /> : !countdown && !f.scene && <div className="beatph big"><div className="paintspin">🎬</div><span>the judges are setting the scene…</span></div>}
          {f.resolving && <p className="sub">⚔️ the judges are scoring round {f.round}…</p>}
        </div>
        <aside className="blog">
          <Arena state={state} />
          <div className="acts">
            {!f.resolving && !countdown && isFighter && !acted && (
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

// ---------- verdict ----------

function VerdictView({ state, send }: { state: State; send: Send }) {
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
