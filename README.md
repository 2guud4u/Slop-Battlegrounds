# Slop-Battlegrounds

tech stack is react and node.js typescript with Go backend

The game is like cards against humanity where players get cards with nouns, adjectives, verbs

 then they can combine the nouns and adjectives to create a monster/champion to fight for them

 two players are pitted against each other, they play the cards to create their champions, then we have ai generate images of each champions and we have llms decide who wins, during the fight, players can play verbs to influence the llm decisions, at the end te llm will generate series of events about the fight with associated images

 before the fight, the non fighting players can place bets, where they win coins, coins can be used in shop phase to buy different effects.

 set up a bare minium scaffold of the game for now and search up free apis. can use for the llm and image generation.

## Running

```bash
make dev     # backend :8080 + Vite :5173 with hot reload
make run     # build frontend, single Go binary serves everything on :8080
make test    # internal/... tests
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
| Image gen | Pollinations gen API (`openai/gpt-image-1-mini`; `?image=` passes the champion portrait for likeness) | `POLLINATION_API_KEY` |

Server-level env: `GROQ_API_KEY`, `POLLINATION_API_KEY`, `GEMINI_API_KEY`, `CF_ACCOUNT_ID`, `CF_API_TOKEN`, `ADDR` (default `:8080`), `GEN_DIR` (default `$TMPDIR/slop-gen`).

`auto` image provider = Pollinations (keyed). The old keyless `image.pollinations.ai` URL API is replaced by `gen.pollinations.ai`.

**Host keys:** whoever creates a room can paste their own Groq / Gemini /
Cloudflare keys on the join screen — they apply to that room only,
are held in memory, and are never sent to other clients.

## Game loop

`lobby → draft → betting → combat → verdict → (shop, once everyone's fought) → draft …`

- **draft**: pick a **forge template**, then fill its part-of-speech slots with cards (order and grammar enforced by the template). Everyone starts with `__noun__`, `__adj__`, `__adj__ __noun__`; champion image generated async. Noun cards sometimes arrive with an adjective attached; verb cards with an adverb. Spectators vote on the battleground — 4 LLM-generated locations per round.
- **betting**: non-fighters stake coins (min 10, winners get 2× back); skipped with only 2 players. The judge's scene narration + establishing art land here.
- **combat**: fighters pick one of 3 LLM-authored actions per round (or let fate pick) — up to 3 rounds, earlier rounds are a slideshow with art per beat.
- **verdict**: LLM picks a winner + narrates (fallback: mock judge); winner +25🪙, everyone +20🪙.
- **shop**: opens once every connected player has fought. Buy forge templates (e.g. `__noun__ with __adj__ and __adj__ __noun__` — "Dragon with Fiery and Sharp Claws") with coins; new templates unlock their parts of speech (verb/adv/pron/prep/conj) in your hand.
- Host picks the narration **story level** in the lobby: Middle School / High School (default) / College — it shapes both narration and action options.
Phase timeouts auto-advance; host can also skip the verdict screen.

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
cmd/server/           entrypoint — loads config, mounts the hub
internal/config/      .env → typed Defaults (provider keys, GEN_DIR, ADDR)
internal/api/         handlers.go (mux + ws + room registry),
                      providers.go (judge/imager resolution), ws.go (send pump)
internal/game/        room state machine (lobby→draft→betting→combat→verdict)
internal/llm/         groq.go, mock.go   (game.Judge)
internal/image/       gemini, cloudflare, pollinations, picsum-mock (game.Imager)
web/src/components/   screens (Lobby, Draft, Fight, Shop) + shared widgets
web/src/hooks/        useNow (tick), …
web/src/services/     socket.ts — ws connect + join + send helpers
web/src/styles/       style.css
```
need llm to draft scenario
need image generation
need different card prompts
need structure
need a way not have users wait forever
