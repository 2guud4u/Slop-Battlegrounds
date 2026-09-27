# Slop-Battlegrounds

tech stack is react and node.js typescript with Go backend

The game is like cards against humanity where players get cards with nouns, adjectives, verbs

 then they can combine the nouns and adjectives to create a monster/champion to fight for them

 two players are pitted against each other, they play the cards to create their champions, then we have ai generate images of each champions and we have llms decide who wins, during the fight, players can play verbs to influence the llm decisions, at the end te llm will generate series of events about the fight with associated images

 before the fight, the non fighting players can place bets, where they win coins, coins can be used in shop phase to buy different effects.

 set up a bare minium scaffold of the game for now and search up free apis. can use for the llm and image generation.

## Running

```bash
# backend (serves ws on :8080, static frontend if web/dist exists)
go run ./cmd/server

# frontend dev server (proxies /ws + /gen to :8080)
cd web && npm install && npm run dev    # http://localhost:5173

# prod-ish: build once, Go serves everything on :8080
cd web && npm run build && cd .. && go run ./cmd/server
```

## AI providers

Both the judge (LLM) and image generation sit behind small interfaces and have
`mock` implementations, so the game is fully playable with zero keys (mock judge,
picsum placeholder images).
| Service | Provider | Keys needed |
|---|---|---|
| LLM judge | Groq (`llama-3.3-70b-versatile`) | `GROQ_API_KEY` |
| Image gen | Google AI Studio (`gemini-2.5-flash-image` — easiest, single key) | `GEMINI_API_KEY` |
| Image gen | Cloudflare Workers AI (`@cf/black-forest-labs/flux-1-schnell`) | `CF_ACCOUNT_ID` + `CF_API_TOKEN` |
| Image gen | Pollinations (opt-in, no key — quality is rough) | none |

Server-level env: `GROQ_API_KEY`, `GEMINI_API_KEY`, `CF_ACCOUNT_ID`, `CF_API_TOKEN`, `ADDR` (default `:8080`), `GEN_DIR` (default `$TMPDIR/slop-gen`).

`auto` resolution order: Gemini → Cloudflare → picsum mock.

**Host keys:** whoever creates a room can paste their own Groq / Gemini /
Cloudflare keys on the join screen — they apply to that room only,
are held in memory, and are never sent to other clients.

## Game loop

`lobby → draft → betting → combat → verdict → draft …`

- **draft**: pick adjective + noun → champion, image generated async
- **betting**: non-fighters stake coins (min 10, winners get 2× back); skipped with only 2 players
- **combat**: fighters play up to 2 verb cards each to sway the judge
- **verdict**: LLM picks a winner + narrates events (fallback: mock judge); winner +25🪙, everyone +20🪙
- phase timeouts auto-advance; host can also skip the verdict screen

Shop phase is intentionally not implemented yet.

## Card pools

Adjective/noun/verb pools in `internal/game/cards.go` were mined from the
Cards Against Humanity corpus ([FireRat666/Json-Against-Humanity](https://github.com/FireRat666/json-against-humanity),
CC BY-NC-SA 4.0 — non-commercial use only), filtered to short phrases.

## Avatars

Lobby picker uses the italian brainrot cast. Images scraped from
[brainrothub.com](https://brainrothub.com/blog/italian-brainrot-characters-abilities-lore)
into `web/public/avatars/` (webp, served statically). Selection is a `{"type":"avatar"}`
ws message; lobby-only, shown in the player bar and over fighter cards.

## Layout

```
cmd/server/          entrypoint
internal/game/       room state machine (lobby→draft→betting→combat→verdict)
internal/llm/        groq.go, mock.go   (game.Judge)
internal/image/      gemini, cloudflare, pollinations, picsum-mock (game.Imager)
internal/server/     websocket hub, room registry, provider wiring
web/                 vite + react + ts client
```
