import { useEffect, useState } from "react";
import type { JoinOpts, Send } from "./ws";
import type { AvatarInfo, State } from "./types";

export function JoinScreen({ onJoin, error }: { onJoin: (o: JoinOpts) => void; error: string }) {
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

export function Lobby({ state, isHost, send }: { state: State; isHost: boolean; send: Send }) {
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
