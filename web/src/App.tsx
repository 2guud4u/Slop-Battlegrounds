import { useEffect, useRef, useState } from "react";
import { connect, type JoinOpts, type Send } from "./ws";
import type { AvatarInfo, Card, State } from "./types";

export default function App() {
  const [state, setState] = useState<State | null>(null);
  const [err, setErr] = useState("");
  const sendRef = useRef<Send>(() => {});
  const [joined, setJoined] = useState(false);

  const join = (opts: JoinOpts) => {
    setErr("");
    sendRef.current = connect(
      opts,
      (s) => {
        setState(s);
        setJoined(true);
      },
      setErr,
      () => setJoined(false),
    );
  };

  const send = (msg: Record<string, unknown>) => sendRef.current(msg);

  if (!joined || !state) {
    return <JoinScreen onJoin={join} error={err} />;
  }
  const me = state.players.find((p) => p.id === state.you);
  const isHost = state.hostId === state.you;

  return (
    <div className="app">
      <header>
        <h1>⚔️ Slop Battlegrounds</h1>
        <span className="badge">
          room <b>{state.room}</b> · round {state.round} · {state.phase}
        </span>
      </header>
      <PlayerBar state={state} />
      {state.phase === "lobby" && <Lobby state={state} isHost={isHost} send={send} />}
      {state.phase === "draft" && me && <Draft state={state} me={me.id} send={send} />}
      {state.phase === "betting" && <Betting state={state} you={state.you} send={send} />}
      {state.phase === "combat" && <Combat state={state} you={state.you} send={send} />}
      {state.phase === "verdict" && <VerdictView state={state} isHost={isHost} send={send} />}
      {err && <p className="err">{err}</p>}
    </div>
  );
}

// ---------- join / lobby ----------

function JoinScreen({ onJoin, error }: { onJoin: (o: JoinOpts) => void; error: string }) {
  const [name, setName] = useState("");
  const [room, setRoom] = useState("");
  const [showKeys, setShowKeys] = useState(false);
  const [llmKey, setLlmKey] = useState("");
  const [cfAccount, setCfAccount] = useState("");
  const [geminiKey, setGeminiKey] = useState("");
  const [cfToken, setCfToken] = useState("");

  return (
    <div className="join">
      <h1>⚔️ Slop Battlegrounds</h1>
      <p className="sub">Build a champion. Judge by AI. Bet on slop.</p>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (!name.trim()) return;
          onJoin({
            name: name.trim(),
            room: room.trim() || undefined,
            llmKey: llmKey.trim() || undefined,
            imageKey: geminiKey.trim() || undefined,
            imageAccount: cfAccount.trim() || undefined,
            imageToken: cfToken.trim() || undefined,
          });
        }}
      >
        <input placeholder="Your name" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
        <input placeholder="Room code (empty = create)" value={room} onChange={(e) => setRoom(e.target.value)} />
        <button type="button" className="link" onClick={() => setShowKeys(!showKeys)}>
          {showKeys ? "▾ hide" : "▸"} host API keys (optional)
        </button>
        {showKeys && (
          <div className="keys">
            <p className="hint">
              Creating a room? Paste your own keys — otherwise the server defaults (or mocks) are used.
            </p>
            <input placeholder="Groq API key (LLM judge)" value={llmKey} onChange={(e) => setLlmKey(e.target.value)} />
            <input
              placeholder="Gemini API key (image gen — easiest)"
              value={geminiKey}
              onChange={(e) => setGeminiKey(e.target.value)}
            />
            <input
              placeholder="Cloudflare account ID (image gen)"
              value={cfAccount}
              onChange={(e) => setCfAccount(e.target.value)}
            />
            <input
              placeholder="Cloudflare API token (image gen)"
              value={cfToken}
              onChange={(e) => setCfToken(e.target.value)}
            />
          </div>
        )}
        <button type="submit">{room.trim() ? "Join room" : "Create room"}</button>
      </form>
      {error && <p className="err">{error}</p>}
    </div>
  );
}

function PlayerBar({ state }: { state: State }) {
  return (
    <div className="players">
      {state.players.map((p) => (
        <span key={p.id} className={"chip" + (p.connected ? "" : " off")}>
          {p.avatarUrl && <img className="av" src={p.avatarUrl} alt="" />}
          {p.host && "👑 "}
          {p.name} <span className="coinchip">{p.coins}</span>
          {state.phase === "draft" && (p.drafted ? " ✅" : " …")}
        </span>
      ))}
    </div>
  );
}

function Lobby({ state, isHost, send }: { state: State; isHost: boolean; send: Send }) {
  const [avatars, setAvatars] = useState<AvatarInfo[]>([]);
  const me = state.players.find((p) => p.id === state.you);
  useEffect(() => {
    fetch("/api/avatars").then((r) => r.json()).then(setAvatars).catch(() => {});
  }, []);
  return (
    <section>
      <h2>Lobby</h2>
      <p>
        Share room code <b>{state.room}</b>. Need 2+ players.
      </p>
      <p className="sub">Pick your manager:</p>
      <div className="avatars">
        {avatars.map((a) => (
          <button
            key={a.key}
            className={"avatar" + (me?.avatar === a.key ? " sel" : "")}
            title={a.name}
            onClick={() => send({ type: "avatar", avatar: a.key })}
          >
            <img src={a.url} alt={a.name} />
            <span>{a.name}</span>
          </button>
        ))}
      </div>
      {isHost ? (
        <button disabled={state.players.length < 2} onClick={() => send({ type: "start" })}>
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
  const [adj, setAdj] = useState("");
  const [noun, setNoun] = useState("");
  const myHand = state.hand;
  const meP = state.players.find((p) => p.id === me);
  const done = !!meP?.champion;

  return (
    <section>
      <h2>Draft your champion</h2>
      <p className="sub">Pick an adjective + a noun.</p>
      {done ? (
        <ChampionCard champ={meP!.champion!} label="Your champion" />
      ) : (
        <>
          <CardRow
            cards={myHand.filter((c) => c.kind === "adj")}
            selected={adj}
            onPick={setAdj}
          />
          <CardRow
            cards={myHand.filter((c) => c.kind === "noun")}
            selected={noun}
            onPick={setNoun}
          />
          <button
            disabled={!adj || !noun}
            onClick={() => {
              send({ type: "draft", adj, noun });
              setAdj("");
              setNoun("");
            }}
          >
            Forge champion
          </button>
        </>
      )}
      <ChampionGallery state={state} />
    </section>
  );
}

function CardRow({
  cards,
  selected,
  onPick,
}: {
  cards: Card[];
  selected: string;
  onPick: (id: string) => void;
}) {
  return (
    <div className="cardrow">
      {cards.map((c) => (
        <button
          key={c.id}
          className={"card " + c.kind + (selected === c.id ? " sel" : "")}
          data-kind={c.kind}
          onClick={() => onPick(selected === c.id ? "" : c.id)}
        >
          {c.text}
        </button>
      ))}
    </div>
  );
}

function ChampionCard({ champ, label }: { champ: { adj: string; noun: string; imageUrl: string }; label?: string }) {
  return (
    <div className="champ">
      {champ.imageUrl ? <img src={champ.imageUrl} alt="" /> : <div className="imgph">…</div>}
      <div>
        {label && <small>{label}</small>}
        <b>
          {champ.adj} {champ.noun}
        </b>
      </div>
    </div>
  );
}

function ChampionGallery({ state }: { state: State }) {
  const drafted = state.players.filter((p) => p.champion);
  if (!drafted.length) return null;
  return (
    <div className="gallery">
      {drafted.map((p) => (
        <div key={p.id}>
          <ChampionCard champ={p.champion!} />
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
    <section>
      <h2>Betting</h2>
      <Arena state={state} />
      {isFighter ? (
        <p className="sub">You're fighting — spectators are placing bets.</p>
      ) : myBet ? (
        <p>
          Bet placed{myBet.pass ? " (passed)" : `: ${myBet.amount}🪙 on ${nameOf(state, myBet.on)}`}. Waiting… ({doneCount}/{bettors.length})
        </p>
      ) : (
        <div className="betslip">
          <input
            type="number"
            min={10}
            value={amount}
            onChange={(e) => setAmount(Number(e.target.value))}
          />
          <button onClick={() => send({ type: "bet", on: f.a.playerId, amount })}>
            Bet on {f.a.champion.adj} {f.a.champion.noun}
          </button>
          <button onClick={() => send({ type: "bet", on: f.b.playerId, amount })}>
            Bet on {f.b.champion.adj} {f.b.champion.noun}
          </button>
          <button className="link" onClick={() => send({ type: "pass" })}>
            Pass
          </button>
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
    <div className="arena">
      <FighterCard f={f.a} />
      <div className="vs">VS</div>
      <FighterCard f={f.b} />
    </div>
  );
}

function FighterCard({ f }: { f: { playerId: string; name: string; avatarUrl: string; champion: { adj: string; noun: string; imageUrl: string }; verbs: string[] } }) {
  return (
    <div className="fighter">
      <div className="fighter-head">
        {f.avatarUrl && <img className="av big" src={f.avatarUrl} alt="" />}
        <ChampionCard champ={f.champion} label={f.name} />
      </div>
      <ul className="verbs">
        {f.verbs.map((v, i) => (
          <li key={i}>{v}</li>
        ))}
      </ul>
    </div>
  );
}

function Combat({ state, you, send }: { state: State; you: string; send: Send }) {
  const f = state.fight;
  if (!f) return null;
  const isFighter = f.a.playerId === you || f.b.playerId === you;
  const myMoves = isFighter ? (f.a.playerId === you ? f.a : f.b).verbs.length : 0;

  return (
    <section>
      <h2>Fight!</h2>
      <Arena state={state} />
      {isFighter ? (
        <>
          <p className="sub">
            Play verb cards to influence the judge ({myMoves}/2 played).
          </p>
          <div className="cardrow">
            {state.hand
              .filter((c) => c.kind === "verb")
              .map((c) => (
                <button key={c.id} className="card verb" data-kind="verb" onClick={() => send({ type: "verb", verb: c.id })}>
                  {c.text}
                </button>
              ))}
          </div>
          <button className="link" onClick={() => send({ type: "pass" })}>
            Done fighting
          </button>
        </>
      ) : (
        <p className="sub">The battle rages… fighters are playing their moves.</p>
      )}
    </section>
  );
}

// ---------- verdict ----------

function VerdictView({ state, isHost, send }: { state: State; isHost: boolean; send: Send }) {
  const f = state.fight;
  if (!f) return null;
  const v = f.verdict;
  const winnerName = v ? nameOf(state, v.winnerId) : "";

  return (
    <section>
      <h2>Verdict</h2>
      <Arena state={state} />
      {!v ? (
        <p className="sub">The judge is deliberating…</p>
      ) : (
        <div className="verdict">
          <h3>🏆 {winnerName} wins!</h3>
          <p>{v.reason}</p>
          {v.events.map((e, i) => (
            <div key={i} className="event">
              <span className="evnum">{i + 1}</span>
              {e.imageUrl && <img src={e.imageUrl} alt="" />}
              <p>{e.text}</p>
            </div>
          ))}
          {isHost && (
            <button onClick={() => send({ type: "next" })}>Next round</button>
          )}
        </div>
      )}
    </section>
  );
}

