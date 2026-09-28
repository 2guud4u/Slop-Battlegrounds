package game

// Word pools mined from the Cards Against Humanity corpus (CC BY-NC-SA),
// filtered to short phrases and hand-merged with the original list.

var adjectivePool = []string{
	"Slimy", "Turbo", "Emo", "Disco",
	"Crusty", "Quantum", "Radioactive", "Soggy",
	"Vegan", "Explosive", "Depressed", "Cyber",
	"Haunted", "Ferocious", "Bureaucratic", "Inflatable",
	"Ancient", "Sentient", "Microwaved", "Absolute",
	"Crypto", "Nuclear", "Feral", "Weird",
	"Strange", "Sweaty", "Iron", "Ugly",
	"Crazy", "Cunning", "Gold", "Badass",
	"Rainbow", "Disappointing", "Gravitational",
	"Galactic", "Friendly", "Pointy", "Wild",
	"Flaming", "Bloody", "Spontaneous",
	"Suspicious", "Moist", "Illegal", "Premium",
	"Discount", "Unhinged", "Melancholy", "Frostbitten",
	"Anxious", "Deluxe", "Suspiciously Quiet", "Regal",
	"Gaslit", "Bioengineered", "Peasant", "Mythic",
	"Disgruntled", "Gluten-Free", "Possessed", "Unlicensed",
	"Abandoned", "Vegan-Friendly", "Levitating", "Cowardly",
	"Steampunk", "Cardboard", "Left-Handed", "Underwater",
	"Two-Faced", "Vindictive", "Moisturized", "Baroque",
}

var nounPool = []string{
	"Grandma", "Wizard", "Hamster", "Tax Auditor",
	"Dumpster", "Dragon", "Intern", "Clown",
	"Skateboard", "Toaster", "Warlord", "Pigeon",
	"Roomba", "Chupacabra", "Librarian", "CEO",
	"Mushroom", "Submarine", "Scarecrow", "Cryptobro",
	"Lawn Gnome", "Bouncer", "Kebab", "God",
	"Monster", "Sword", "Monkey", "Gag Reflex",
	"Bomb", "Massage", "Unicorn", "Disaster",
	"Banana", "Potion", "Llama", "Handshake",
	"Union", "High Five", "Boss", "Council",
	"Illuminati", "Internet", "Biscuit",
	"Podcast", "Complex", "Godmother", "Bunny",
	"Fairy", "Machine", "Ninja", "Dutchman",
	"Universe", "Ticking Noise", "Squid", "Hooker",
	"Teacher", "Engineer", "Sandwich", "Strangler",
	"Killer", "Black Hole", "Wendigo", "Pineapple",
	"Bubble", "Raid", "Convention",
	"Mistake", "Toilet", "Boyfriend", "Cow",
	"Beard", "Epidemic",
	"Ambulance", "Chiropractor", "Vending Machine", "Ghost Writer",
	"Side Quest", "Landlord", "Cult Leader", "Fanny Pack",
	"Steam Engine", "Rotisserie Chicken", "Taxidermist", "Food Coma",
	"Perpetual Motion", "Gym Teacher", "Final Boss", "Karaoke Machine",
	"Second Cousin", "Meatball", "Charity Gala", "Time Machine",
	"Bad Wi-Fi", "Alley Cat", "Day Spa", "Haunted Fridge",
	"Librarian Ghost", "Parking Ticket", "Mall Santa", "Job Interview",
	"Existential Dread", "Submarine Sandwich", "Old Gods", "Mini Fridge",
	"Thunderdome", "Farmers Market", "Retirement Plan", "Lawsuit",
	"Space Elevator", "Leftovers", "Cryptid", "Gym Membership",
}

var verbPool = []string{
	// combat — the lethal kind
	"summons a lightning storm", "unleashes a bass drop", "hacks the mainframe", "calls in an airstrike",
	"deploys a smoke screen", "casts a silence spell", "awakens an ancient curse", "goes full berserker mode",
	"transforms into a monster truck", "dodges like the Matrix", "charges a laser cannon", "detonates a glitter bomb",
	"banishes the opponent to the shadow realm", "summons a pack of wolves", "shapeshifts into a vending machine", "hurls a bowl of hot soup",
	"throws a suplex into the center of the earth", "grows fifty feet tall", "hiccups fireballs", "swings a ceremonial halibut",
	"activates a pocket-sized black hole", "summons a legal team", "dives through a plate-glass window", "enrages into super saiyan mode",
	// absurd CAH energy, still actions
	"dabs menacingly", "cries uncontrollably", "offers a firm handshake", "starts a pyramid scheme",
	"files a restraining order", "files for bankruptcy", "chugs a gallon of milk", "breaks the fourth wall",
	"turns it off and on again", "runs away with the circus", "screams at inanimate objects", "punches a mime",
	"weaponizes a pink fedora", "invents new curse words", "puts on a serious voice", "puts on pants mid-fight",
	"eats anxiety sandwiches", "hunts unicorns mid-brawl", "chugs three energy drinks", "deploys a feral Roomba",
	// wildcard
	"duplicates into a hundred copies", "throws a folding chair", "bites through the armor", "spawns a portal directly underneath",
	"dropkicks the opponent's pelvis", "shouts the forbidden word", "launches a rocket-powered fist", "hurls a thunderbolt",
	"folds the opponent into a paper airplane", "whips out nunchucks made of sausages", "farts hard enough to launch upward",
	"weaponizes a hug", "time-travels five seconds into the future", "inhales the opponent's attack", "roundhouse-kicks the planet off its axis",
}

// Single-word pools for forge cards — the arena stays on verbPool above.
var verbCardPool = []string{
	"Yeets", "Suplexes", "Summons", "Hacks", "Devours",
	"Banishes", "Wields", "Obliterates", "Snacks On", "Judges",
	"Landslides", "Duplicates", "Teriyaki-Grills", "Repossesses", "Sunbathes",
	"Therapy-Hugs", "DJ-Spins", "Dabs On", "Berserks", "Files",
}

var adverbPool = []string{
	"Ruthlessly", "Suspiciously", "Violently", "Gently", "Horizontally",
	"Forever", "Barely", "Aggressively", "Yesterday", "Underhandedly",
	"Seductively", "Somewhat", "Metaphysically", "Recklessly", "Politely",
}

var pronounPool = []string{
	"He", "She", "They", "It", "One",
	"Someone", "Everything", "Whoever", "Ye", "Nobody",
}

var prepositionPool = []string{
	"in", "on", "under", "inside", "between",
	"behind", "beyond", "through", "without", "upon",
	"despite", "among", "beneath", "within", "outside",
}

var conjunctionPool = []string{
	"and", "but", "or", "yet", "so",
	"while", "nor", "because", "tho", "versus",
}

// cardPools maps every drawable kind to its word pool.
var cardPools = map[string][]string{
	KindAdj:  adjectivePool,
	KindNoun: nounPool,
	KindVerb: verbCardPool,
	KindAdv:  adverbPool,
	KindPron: pronounPool,
	KindPrep: prepositionPool,
	KindConj: conjunctionPool,
}

// battlegroundPool: locations voted on during draft.
var battlegroundPool = []string{
	"an active volcano rim",
	"a flooded shopping mall",
	"the moon landing set",
	"inside a giant washing machine",
	"a medieval jousting arena",
	"a sinking pirate ship",
	"an abandoned amusement park",
	"a den of sleeping dragons",
	"the produce aisle at 3am",
	"a collapsing skyscraper rooftop",
	"inside a whirlpool",
	"a spaghetti western ghost town",
}
