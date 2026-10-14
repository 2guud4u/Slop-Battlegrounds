package game

import (
	"context"
	"log"
	"time"
)

// after is the room's async scheduler: work runs off-lock and returns
// (result, keep); if keep and the room's gen epoch hasn't moved since the
// call, apply runs under r.mu. The gen check lives here so no call site can
// forget it — per-touchpoint staleness (same fight, same phase) belongs in
// the apply closure. Call with r.mu held.
func after[T any](r *Room, work func() (T, bool), apply func(T)) {
	gen := r.gen
	go func() {
		v, keep := work()
		if !keep {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.gen != gen {
			return // stale — the room moved on while work ran
		}
		apply(v)
	}()
}

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
	after(r, func() (RoundOptions, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		opts, _ := r.judge.Options(ctx, req) // resilient: degrades internally
		return opts, true
	}, func(opts RoundOptions) {
		if r.fight != f || f.OptRound != req.Round {
			return // stale
		}
		f.Options[f.A] = opts.A
		f.Options[f.B] = opts.B
		r.broadcastLocked()
	})
}

// ---------- phase machine ----------

// genLocationsLocked asks the judge for this round's battlegrounds; applied
// only while still in the same draft phase. Call with r.mu held.
func (r *Room) genLocationsLocked() {
	after(r, func() ([]string, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		locs, _ := r.judge.Locations(ctx, 4, r.Complexity) // resilient: pool fallback inside
		return locs, true
	}, func(locs []string) {
		if len(r.locOptions) == 0 {
			r.locOptions = locs
			r.broadcastLocked()
		}
	})
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
	after(r, func() (string, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s, _ := r.judge.Scene(ctx, JudgeRequest{A: *a.Champion, B: *b.Champion, Location: f.Location, Level: r.Complexity})
		return s, s != ""

	}, func(s string) {
		if r.fight != f || f.Scene != "" {
			return
		}
		f.Scene = s
		r.broadcastLocked()
		// paint the narration — before the first bell if the net is kind
		prompt := sceneImagePrompt(f.Location, s, a.Champion.Name(), b.Champion.Name())
		r.genImage(prompt, "", func(url string) {
			if r.fight == f && f.SceneImage == "" {
				f.SceneImage = url
				r.broadcastLocked()
			}
		})
	})
}

// genImage calls the imager off-lock, then applies the result under r.mu.
// Call with r.mu held.
func (r *Room) genImage(prompt, ref string, apply func(string)) {
	after(r, func() (string, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		url, err := r.imager.Generate(ctx, prompt, ref)
		if err != nil {
			log.Printf("room %s: image gen failed: %v", r.Code, err)
			return "", false
		}
		return url, true
	}, apply)
}
