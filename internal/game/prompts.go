package game

import "fmt"

// Image prompt recipes shared by every game.Imager provider. Structure follows
// [subject], [action], [environment], [style], [lighting], [composition],
// [quality] — the ordering diffusion models weight most.
const artStyle = "vibrant stylized digital painting, bold saturated colors, dramatic cinematic lighting, sharp focus, highly detailed"

// sceneImagePrompt: wide establishing shot of the arena narration, shown while
// spectators bet — both champions named so the model paints the matchup.
func sceneImagePrompt(location, scene, a, b string) string {
	return fmt.Sprintf("%s versus %s in %s — %s, epic establishing shot, %s", a, b, location, scene, artStyle)
}

// actionPrompt: mid-fight scene for one played verb.
func actionPrompt(champ, verb, location string) string {
	if location == "" {
		location = "a strange arena"
	}
	return fmt.Sprintf("%s is %s, locked in mortal combat, %s, dynamic action shot, motion blur on the background, %s", champ, verb, "in "+location, artStyle)
}

// eventPrompt: verdict narration scene; the judge supplies the beat.
func eventPrompt(scene string) string {
	return fmt.Sprintf("%s, brutal deathmatch scene, wide dramatic shot, %s", scene, artStyle)
}
