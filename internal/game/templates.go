package game

// Forge templates constrain what a champion name may look like: an ordered
// pattern of part-of-speech slots and literal words. Players start with the
// three one- or two-word starters and buy bigger patterns in the shop.
// Owning a template also unlocks drawing cards of the kinds it uses.
type ForgeTemplate struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`    // pattern shown to players, e.g. "__adj__ __noun__"
	Cost    int      `json:"cost"`    // coins; 0 = starter
	Example string   `json:"example"` // a sample forged name
	Slots   []string `json:"slots"`   // Kind* values or literal words
	Sold    bool     `json:"sold"`    // shop: someone already bought this pattern
}

// Literal slots are joined into the name verbatim — they aren't cards.
func slotIsLiteral(s string) bool { return cardPools[s] == nil }

// Kinds returns the set of card kinds this template's slots need.
func (t ForgeTemplate) Kinds() map[string]bool {
	out := map[string]bool{}
	for _, s := range t.Slots {
		if !slotIsLiteral(s) {
			out[s] = true
		}
	}
	return out
}

var forgeTemplates = []ForgeTemplate{
	// starters — free
	{ID: "solo_noun", Name: "__noun__", Example: "Dragon", Slots: []string{KindNoun}},
	{ID: "adj_noun", Name: "__adj__ __noun__", Example: "Fiery Dragon", Slots: []string{KindAdj, KindNoun}},
	{ID: "adj_and_adj_noun", Name: "__adj__ and __adj__ __noun__", Example: "Fiery and Sharp Claws",
		Slots: []string{KindAdj, "and", KindAdj, KindNoun}},

	// shop catalog — cost unlocks both the pattern and its card kinds
	{ID: "noun_of_noun", Name: "__noun__ of __noun__", Cost: 30, Example: "Godzilla of Pigeons",
		Slots: []string{KindNoun, "of", KindNoun}},
	{ID: "adj_adj_noun", Name: "__adj__ __adj__ __noun__", Cost: 35, Example: "Crusty Haunted Barn",
		Slots: []string{KindAdj, KindAdj, KindNoun}},
	{ID: "noun_prep_noun", Name: "__noun__ __prep__ __noun__", Cost: 40, Example: "Demons in Denim",
		Slots: []string{KindNoun, KindPrep, KindNoun}},
	{ID: "pron_the_noun", Name: "__pron__ the __noun__", Cost: 40, Example: "She the Wasp",
		Slots: []string{KindPron, "the", KindNoun}},
	{ID: "noun_with_adj_noun", Name: "__noun__ with __adj__ and __adj__ __noun__", Cost: 45, Example: "Dragon with Fiery and Sharp Claws",
		Slots: []string{KindNoun, "with", KindAdj, "and", KindAdj, KindNoun}},
	{ID: "noun_who_verb", Name: "__noun__ who __verb__", Cost: 50, Example: "Grandma Who Suplexes",
		Slots: []string{KindNoun, "who", KindVerb}},
	{ID: "adv_verb_noun", Name: "__adv__ __verb__ __noun__", Cost: 60, Example: "Ruthlessly Yeets Grandmas",
		Slots: []string{KindAdv, KindVerb, KindNoun}},
	{ID: "noun_conj_noun", Name: "__noun__ __conj__ __noun__", Cost: 45, Example: "Wolf but Lawyer",
		Slots: []string{KindNoun, KindConj, KindNoun}},
}

var starterTemplateIDs = []string{"solo_noun", "adj_noun", "adj_and_adj_noun"}

// TemplateByID returns the template or nil.
func TemplateByID(id string) *ForgeTemplate {
	for i := range forgeTemplates {
		if forgeTemplates[i].ID == id {
			return &forgeTemplates[i]
		}
	}
	return nil
}
