package types

// Percept is the world as the hero takes it in: where he is, what is around
// him and what he carries, in the names the game itself gives them — object
// captions from TEXT.DAT, item names from BAR.BAR. It is everything a driver
// outside the game learns about the world: no quest variables, no scripts, no
// cells or internal keys.
type Percept struct {
	Where string `json:"where"` // one of the Where* values
	// Busy is what is under way ("идёт сцена", "я иду"); "" when the hero
	// is free to act.
	Busy   string  `json:"busy,omitempty"`
	Around []Thing `json:"around,omitempty"` // what can be acted on
	Exits  []Thing `json:"exits,omitempty"`  // the ways out of the place
	Hands  string  `json:"hands,omitempty"`  // the item in hand; "Рука" is none
	// EmptyHands says the hand holds nothing: Hands is the bare hand.
	EmptyHands bool `json:"empty_hands,omitempty"`
	// Carry is the rest of what the hero has on him, in bar order.
	Carry  []string   `json:"carry,omitempty"`
	Friday *Companion `json:"friday,omitempty"` // Friday, while she is with him
	Map    bool       `json:"map"`              // the island map can be opened
	// Hearing is the line on screen right now.
	Hearing string `json:"hearing,omitempty"`
}

// Where the hero is, as he would put it.
const (
	WhereIsland = "остров"
	WhereMap    = "карта острова"
	WherePuzzle = "головоломка"
	WhereScene  = "заставка"
	WherePause  = "пауза"
)

// Which side of the hero something is on.
const (
	SideLeft  = "слева"
	SideRight = "справа"
	SideNear  = "рядом"
)

// Thing is something the hero sees and can go to: its name and which side of
// him it is on. Things sharing a name are numbered from the left ("Камни 2").
type Thing struct {
	Name string `json:"name"`
	Side string `json:"side,omitempty"`
}

// Companion is Friday as the hero sees her.
type Companion struct {
	Side  string   `json:"side,omitempty"`
	Hands string   `json:"hands,omitempty"`
	Carry []string `json:"carry,omitempty"`
}

// Outcome is what came of an action: the lines shown while it played, whether
// the world answered at all, whether the hero may act again, and the world
// afterwards.
type Outcome struct {
	Said []string `json:"said,omitempty"`
	// Reacted is false when nothing happened: the game ignores an action it
	// has no answer for, and the player sees just that.
	Reacted bool `json:"reacted"`
	// Ready is false when the action is still playing out when the wait ends.
	Ready   bool    `json:"ready"`
	Changes Changes `json:"changes"`
	Look    Percept `json:"look"`
}

// Changes is what the player would notice has changed since an action
// began: a new place, things gained and lost, things that came into sight or
// went, ways out that opened or closed, Friday joining or leaving, the map.
// Misses counts the actions in a row that came to nothing.
type Changes struct {
	NewPlace   bool     `json:"new_place,omitempty"`
	Gained     []string `json:"gained,omitempty"`
	Lost       []string `json:"lost,omitempty"`
	Appeared   []string `json:"appeared,omitempty"`
	Vanished   []string `json:"vanished,omitempty"`
	Opened     []string `json:"opened,omitempty"`
	Closed     []string `json:"closed,omitempty"`
	FridayCame bool     `json:"friday_came,omitempty"`
	FridayLeft bool     `json:"friday_left,omitempty"`
	MapGained  bool     `json:"map_gained,omitempty"`
	Misses     int      `json:"misses,omitempty"`
}
