package game

import (
	"context"
	"log"
	"time"
)

// genOptionsLocked asks the judge for fresh action options for the current
// round; applied only if the round hasn't moved on. Call with r.mu held.
func (r *Room) genOptionsLocked() {
	f := r.fight
	if f == nil {
		return
	}
	f.OptRound = f.Round
	f.Options = map[string][]string{} // clear last round's options until new ones land
	a, b := r.findLocked(f.A), r.findLocked(f.B)
	if a == nil || b == nil || a.Champion == nil || b.Champion == nil {
		return
	}
	req := JudgeRequest{A: *a.Champion, B: *b.Champion, Location: f.Location, Round: f.Round, History: eventHistory(f), Level: r.Complexity}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		opts, err := r.judge.Options(ctx, req)
		cancel()
		if err != nil || len(opts.A) == 0 || len(opts.B) == 0 {
			log.Printf("room %s: options gen failed (%v), using fallback moves", r.Code, err)
			opts = RoundOptions{A: sampleVerbs(verbsPerFighter), B: sampleVerbs(verbsPerFighter)}
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.fight != f || f.OptRound != req.Round {
			return // stale
		}
		f.Options[f.A] = opts.A
		f.Options[f.B] = opts.B
		r.broadcastLocked()
	}()
}

// ---------- phase machine ----------

// genLocationsLocked asks the judge for this round's battlegrounds; applied
// only while still in the same draft phase. Call with r.mu held.
func (r *Room) genLocationsLocked() {
	gen := r.gen
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		locs, err := r.judge.Locations(ctx, 4, r.Complexity)
		cancel()
		if err != nil || len(locs) == 0 {
			locs = PickBattlegrounds(4)
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.gen == gen && len(r.locOptions) == 0 {
			r.locOptions = locs
			r.broadcastLocked()
		}
	}()
}

// genSceneLocked narrates the arena setup (async) then paints a wide shot of
// the scene. Safe to call more than once — SceneGen guards the dispatch.
func (r *Room) genSceneLocked() {
	f := r.fight
	if f == nil || f.SceneGen {
		return
	}
	f.SceneGen = true
	a, b := r.findLocked(f.A), r.findLocked(f.B)
	if a == nil || b == nil || a.Champion == nil || b.Champion == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		s, err := r.judge.Scene(ctx, JudgeRequest{A: *a.Champion, B: *b.Champion, Location: f.Location, Level: r.Complexity})
		cancel()
		if err != nil || s == "" {
			log.Printf("room %s: scene gen failed: %v", r.Code, err)
			return
		}
		r.mu.Lock()
		if r.fight != f || f.Scene != "" {
			r.mu.Unlock()
			return
		}
		f.Scene = s
		r.broadcastLocked()
		r.mu.Unlock()
		// paint the narration — before the first bell if the net is kind
		prompt := sceneImagePrompt(f.Location, s, a.Champion.Name(), b.Champion.Name())
		go r.genImage(prompt, "", func(url string) {
			if r.fight == f && f.SceneImage == "" {
				f.SceneImage = url
				r.broadcastLocked()
			}
		})
	}()
}

// genImage calls the imager off-lock, then applies the result under r.mu.
func (r *Room) genImage(prompt, ref string, apply func(string)) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	url, err := r.imager.Generate(ctx, prompt, ref)
	cancel()
	if err != nil {
		log.Printf("room %s: image gen failed: %v", r.Code, err)
		return
	}
	r.mu.Lock()
	apply(url)
	r.mu.Unlock()
}
