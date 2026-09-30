package llm

import (
	"fmt"
	"strings"

	"slop-battlegrounds/internal/game"
)

// Prompt builders shared by every chat provider. Each returns the full user
// message; the JSON-shaped ones pair with a parser in judge.go.

const announcerStyle = `IMPORTANT WRITING STYLE:
- Write like a funny human game announcer, not a fantasy novelist.
- Use simple, conversational language.
- Describe what happens first; add humor second.
- Keep sentences short and easy to scan.
- Use concrete verbs and nouns.
- Avoid adjective chains such as "glittering, neon-lit, sugar-crazed..."
- Use at most 1-2 descriptive adjectives per sentence.
- Avoid excessive metaphors, similes, and poetic descriptions.
- Do not make every action sound epic or catastrophic.
- Do not use phrases like "tearing through", "roar-like chorus",
  "shrouded in", "under a sky of", "makes the crowd howl", or similar
  trailer/fantasy-novel language unless the joke specifically calls for it.
- Let the ridiculous character names and actions provide the humor.
- Vary sentence structure naturally.
- Do not force an exclamation mark into every sentence.
- NEVER use flowery language just to make the scene sound exciting.
- The narration should feel spontaneous and readable in a game UI.`

const roundTmpl = `You are a funny, slightly unhinged ringside announcer who speaks like a real
person — a BATTLE TO THE DEATH. This is combat round %d of 3. Two champions
traded lethal actions.

Battleground: %q
Champion A: %q — actions this round: %s
Champion B: %q — actions this round: %s

Earlier rounds:
%s

Write a punchy 2-3 sentence play-by-play of this round only. Build on what
already happened (wounds, terrain destroyed, grudges). Reference the actual
moves and make the battleground matter when relevant.

%s

Then decide: is this fight DECIDED — one champion definitively killed,
knocked out cold, or banished? Be stingy; a tie or close call is not decided.%s

Writing level: %s

Respond with strict JSON only:
{"text":"the narration","over":true|false,"winner":"a"|"b"|""}
- "over":true only when the fight is definitively over; then "winner" is the
  victor. Otherwise over=false and winner="".`

const lastRoundRule = `
This is the FINAL round. If both champions somehow still stand, the judges
score it a draw — over=false, winner="". A draw means both survive, barely.`

const optionsTmpl = `You run the battle menu for a ridiculous monster fight to the death.

Battleground: %q
Champion A: %q — Champion B: %q
This is round %d of 3.

Earlier rounds:
%s

Invent 3 absurd, funny, deadly actions each champion could perform NEXT.
Tailor them to their ridiculous names, the battleground, and what already
happened (wounds, broken terrain, grudges).

ACTION STYLE:
- 3-9 words each.
- Start with a strong verb.
- Use simple, readable language.
- Make the action itself funny; don't rely on fancy adjectives.
- Specific and visual beats vague and dramatic.
- Avoid adjective chains.
- Avoid words like "devastating", "unstoppable", "legendary",
  "cataclysmic", "infernal", "mystical", and "ultimate" unless essential
  to the joke.
- No fantasy-novel prose.
- No narration or explanation.
- Each action should feel like something a player would actually choose
  from a game menu.
- PG-13.

Writing level: %s

Respond with strict JSON only:
{"a":["action","action","action"],"b":["action","action","action"]}`

const locationsTmpl = `Invent %d absurd, funny battlegrounds for a ridiculous monster fighting game.

Short noun phrases, 3-7 words each. Places where a fight would be funny
and dangerous.

STYLE:
- Use concrete, recognizable objects and places.
- Keep them easy to visualize.
- Prefer one funny idea over several stacked ideas.
- Avoid elaborate fantasy descriptions.
- Avoid adjective chains.
- No poetic language.
- No numbers, no quotes.

Writing level: %s

Respond with strict JSON only: {"locs":["place","place","place","place"]}`

const sceneTmpl = `You are a funny, slightly unhinged ringside announcer who speaks like a real
person. The fight takes place in %q — a battle to the death. Champion A is %q;
champion B is %q.
Write exactly 2 punchy sentences setting the scene: the arena, the crowd, the
champions' entrances. Simple conversational language, concrete images, short
sentences — the absurd names carry the humor. No adjective chains, no
fantasy-novel prose, PG-13. Writing level: %s
Plain text only.`

// RoundPrompt asks for one round's narration + decided/winner JSON.
func RoundPrompt(req game.JudgeRequest) string {
	tail := ""
	if req.LastRound {
		tail = lastRoundRule
	}
	return fmt.Sprintf(roundTmpl,
		req.Round, orLoc(req.Location), req.A.Name(), orNone(req.MovesA),
		req.B.Name(), orNone(req.MovesB), historyOf(req.History),
		announcerStyle, tail, levelGuide(req.Level))
}

// OptionsPrompt asks for 3 next actions per fighter as JSON.
func OptionsPrompt(req game.JudgeRequest) string {
	return fmt.Sprintf(optionsTmpl, orLoc(req.Location), req.A.Name(), req.B.Name(),
		req.Round, historyOf(req.History), levelGuide(req.Level))
}

// LocationsPrompt asks for n battlegrounds as JSON.
func LocationsPrompt(n int, level string) string {
	return fmt.Sprintf(locationsTmpl, n, levelGuide(level))
}

// ScenePrompt asks for a plain-text 2-sentence arena intro.
func ScenePrompt(req game.JudgeRequest) string {
	return fmt.Sprintf(sceneTmpl, orLoc(req.Location), req.A.Name(), req.B.Name(), levelGuide(req.Level))
}

func orLoc(l string) string {
	if l == "" {
		return "a mysterious void"
	}
	return l
}

func orNone(m []string) string {
	if len(m) == 0 {
		return "(none — they just stood there menacingly)"
	}
	return strings.Join(m, "; ")
}

func historyOf(h []string) string {
	if len(h) == 0 {
		return "(none yet — this is the opening exchange)"
	}
	return strings.Join(h, "\n")
}

// levelGuide translates the room's complexity setting into prompt wording.
func levelGuide(level string) string {
	switch level {
	case game.LevelMiddle:
		return "1 = very simple, conversational game writing. Shortest sentences."
	case game.LevelCollege:
		return "4 = highly expressive but still concise and readable — dry wit, no purple prose."
	default: // high
		return "3 = punchy announcer style with occasional colorful language."
	}
}
