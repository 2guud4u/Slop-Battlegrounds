package game

// Word pools for forge cards. Seeded from the Cards Against Humanity corpus
// (CC BY-NC-SA) and hand-curated. Every noun must be something that could
// step into the arena and fight — a creature, a person, a machine, an object,
// or food — never an abstract idea, event, or place.

var adjectivePool = []string{
	// texture & condition
	"Slimy", "Crusty", "Soggy", "Moist", "Sweaty", "Greasy", "Sticky", "Fuzzy",
	"Rusty", "Dusty", "Squishy", "Crispy", "Lumpy", "Hairy", "Frostbitten", "Moisturized",
	// power & menace
	"Turbo", "Quantum", "Radioactive", "Explosive", "Nuclear", "Ferocious", "Feral", "Mythic",
	"Ancient", "Sentient", "Possessed", "Haunted", "Cursed", "Galactic", "Gravitational", "Flaming",
	"Bloody", "Titanic", "Venomous", "Armored", "Unstoppable", "Legendary", "Undead", "Thunderous",
	// mood
	"Emo", "Depressed", "Anxious", "Unhinged", "Melancholy", "Disgruntled", "Vindictive", "Cowardly",
	"Grumpy", "Smug", "Sleepy", "Hangry", "Petty", "Dramatic", "Overconfident", "Friendly",
	"Suspicious", "Suspiciously Quiet", "Two-Faced", "Cunning", "Crazy", "Weird", "Strange", "Wild",
	// style & origin
	"Disco", "Cyber", "Steampunk", "Baroque", "Regal", "Peasant", "Rainbow", "Gold",
	"Iron", "Cardboard", "Inflatable", "Bioengineered", "Robotic", "Medieval", "Victorian", "Retro",
	"Underwater", "Levitating", "Microwaved", "Deep-Fried", "Glitter-Covered", "Bedazzled", "Pixelated", "Holographic",
	// commerce & bureaucracy
	"Bureaucratic", "Crypto", "Premium", "Discount", "Deluxe", "Unlicensed", "Illegal", "Gluten-Free",
	"Vegan", "Organic", "Knockoff", "Limited-Edition", "Refurbished", "Tax-Deductible", "Corporate", "Artisanal",
	// size & shape
	"Pointy", "Tiny", "Colossal", "Chonky", "Lanky", "Pocket-Sized", "Left-Handed", "Absolute",
	"Ugly", "Badass", "Spontaneous", "Disappointing", "Abandoned",
}

var nounPool = []string{
	// creatures
	"Dragon", "Hamster", "Pigeon", "Llama", "Squid", "Monkey", "Bunny", "Cow",
	"Unicorn", "Chupacabra", "Wendigo", "Cryptid", "Alley Cat", "Goose", "Raccoon", "Possum",
	"Shark", "Octopus", "Badger", "Honey Badger", "Walrus", "Platypus", "Capybara", "Moose",
	"Gorilla", "Kraken", "Yeti", "Sasquatch", "Griffin", "Hydra", "Mothman", "Loch Ness Monster",
	"Goblin", "Troll", "Ogre", "Vampire", "Werewolf", "Zombie", "Mummy", "Ghost",
	"Fairy", "Gremlin", "Demon", "Minotaur", "Kaiju", "T-Rex", "Velociraptor", "Mega Shrimp",
	// people & roles
	"Grandma", "Wizard", "Tax Auditor", "Intern", "Clown", "Warlord", "Librarian", "CEO",
	"Scarecrow", "Cryptobro", "Bouncer", "Ninja", "Pirate", "Samurai", "Viking", "Knight",
	"Gym Teacher", "Chiropractor", "Landlord", "Cult Leader", "Taxidermist", "Mall Santa", "Substitute Teacher", "Lunch Lady",
	"Mime", "Lifeguard", "Mailman", "Plumber", "Barista", "Influencer", "Life Coach", "Magician",
	"Cowboy", "Astronaut", "Pro Wrestler", "Hall Monitor", "HOA President", "Crossing Guard", "Second Cousin", "Godmother",
	// machines & objects
	"Toaster", "Roomba", "Submarine", "Skateboard", "Dumpster", "Vending Machine", "Karaoke Machine", "Time Machine",
	"Steam Engine", "Mini Fridge", "Haunted Fridge", "Ambulance", "Lawn Gnome", "Monster Truck", "Forklift", "Shopping Cart",
	"Microwave", "Blender", "Leaf Blower", "Lawnmower", "Chainsaw", "Printer", "Smart Speaker", "Traffic Cone",
	"Mech Suit", "Robot Vacuum", "Garden Hose", "Rubber Duck", "Teddy Bear", "Beanbag Chair", "Grandfather Clock", "Disco Ball",
	"Sword", "Bomb", "Potion", "Fanny Pack", "Folding Chair", "Toilet", "Bowling Ball", "Pogo Stick",
	// food
	"Kebab", "Banana", "Biscuit", "Sandwich", "Pineapple", "Meatball", "Rotisserie Chicken", "Mushroom",
	"Burrito", "Taco", "Hot Dog", "Pickle", "Waffle", "Pancake Stack", "Corn Dog", "Gummy Bear",
	"Potato", "Eggplant", "Baguette", "Cheese Wheel", "Donut", "Egg Roll", "Chicken Nugget", "Fortune Cookie",
	// beyond
	"God", "Old Gods", "Final Boss", "Black Hole", "Final Form", "Evil Twin", "Clone", "Doppelganger",
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
	"Body-Slams", "Roasts", "Headbutts", "Dropkicks", "Swallows",
	"Out-Dances", "Haunts", "Microwaves", "Audits", "Sues",
	"Gaslights", "Ghosts", "Unsubscribes", "Rizzes", "Photobombs",
	"Moonwalks", "Cancels", "Speedruns", "Pickpockets", "Serenades",
	"Nutmegs", "Tackles", "Uppercuts", "Deep-Fries", "Vaporizes",
}

var adverbPool = []string{
	"Ruthlessly", "Suspiciously", "Violently", "Gently", "Horizontally",
	"Forever", "Barely", "Aggressively", "Yesterday", "Underhandedly",
	"Seductively", "Somewhat", "Metaphysically", "Recklessly", "Politely",
	"Dramatically", "Accidentally", "Sarcastically", "Majestically", "Sneakily",
	"Loudly", "Sheepishly", "Furiously", "Lovingly", "Awkwardly",
	"Illegally", "Professionally", "Emotionally", "Quietly", "Unapologetically",
}

var pronounPool = []string{
	"He", "She", "They", "It", "One",
	"Someone", "Everything", "Whoever", "Ye", "Nobody",
	"Everyone", "Somebody", "Anyone", "We", "Thou",
}

var prepositionPool = []string{
	"in", "on", "under", "inside", "between",
	"behind", "beyond", "through", "without", "upon",
	"despite", "among", "beneath", "within", "outside",
	"above", "against", "across", "around", "atop",
}

var conjunctionPool = []string{
	"and", "but", "or", "yet", "so",
	"while", "nor", "because", "tho", "versus",
	"unless", "until", "whereas", "although", "plus",
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
