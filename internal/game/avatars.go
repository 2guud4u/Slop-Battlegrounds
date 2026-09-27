package game

// Avatar is a selectable character portrait, served from /avatars/.
type Avatar struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Avatars is the lobby picker catalog — italian brainrot cast.
// Images live in web/public/avatars/ (scraped from brainrothub.com) so both
// the vite dev server and the go static server can serve them.
var Avatars = []Avatar{
	{Key: "tralalero", Name: "Tralalero Tralala", URL: "/avatars/tralalero.webp"},
	{Key: "bombardiro", Name: "Bombardiro Crocodilo", URL: "/avatars/bombardiro.webp"},
	{Key: "tung", Name: "Tung Tung Tung Sahur", URL: "/avatars/tung.webp"},
	{Key: "cappuccino", Name: "Ballerina Cappuccina", URL: "/avatars/cappuccino.webp"},
	{Key: "lirili", Name: "Lirili Larila", URL: "/avatars/lirili.webp"},
	{Key: "brr", Name: "Brr Brr Patapim", URL: "/avatars/brr.webp"},
	{Key: "chimpanzini", Name: "Chimpanzini Bananini", URL: "/avatars/chimpanzini.webp"},
	{Key: "bombombini", Name: "Bombombini Gusini", URL: "/avatars/bombombini.webp"},
	{Key: "saturno", Name: "La Vaca Saturno Saturnita", URL: "/avatars/saturno.webp"},
	{Key: "trippi", Name: "Trippi Troppi", URL: "/avatars/trippi.webp"},
	{Key: "bobrini", Name: "Bobrini Cocosini", URL: "/avatars/bobrini.webp"},
	{Key: "frulli", Name: "Frulli Frulla", URL: "/avatars/frulli.webp"},
}

func AvatarURL(key string) string {
	for _, a := range Avatars {
		if a.Key == key {
			return a.URL
		}
	}
	return ""
}
