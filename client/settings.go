package client

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"orbitrix/client/internal/mapstore"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
)

const settingsFile = "settings.json"

// settings is what the game remembers between runs. Seats hold one option per player: seatHuman or 1+strategy,
// stored by name so reordering strategies never changes a saved choice.
type settings struct {
	Cols    int                 `json:"cols"`
	Rows    int                 `json:"rows"`
	Players int                 `json:"players"`
	Map     *core.MapDefinition `json:"map,omitempty"`
	Seats   []string            `json:"seats"`
	Theme   string              `json:"theme"` // "light" or "dark"
	Muted   bool                `json:"muted"`
	Volume  int                 `json:"volume"` // master volume, 0..100
	// Fullscreen is the desktop/browser window mode; Marks draws each player's shape mark on their orbs and dots.
	Fullscreen bool `json:"fullscreen"`
	Marks      bool `json:"marks"`
	// Name is what this player called themselves on Home, as typed; name() is what they go by.
	Name string `json:"name,omitempty"`
	// Online: the host password this device hosts with (cleared when the server refuses it), the Force relay debug
	// switch, and the seat a joiner can take back after the app was closed. Joiners save no password.
	Password string  `json:"password,omitempty"`
	Relay    bool    `json:"relay,omitempty"`
	Rejoin   *rejoin `json:"rejoin,omitempty"`
	// WifiPrivate and OnlinePublic: the access a new room starts with; Same Wi-Fi rooms are public and online rooms
	// private unless the player changes it under Settings, Network.
	WifiPrivate  bool `json:"wifiPrivate,omitempty"`
	OnlinePublic bool `json:"onlinePublic,omitempty"`
	// Firewall: this Windows device was warned that Windows would ask to let Orbitrix on the network.
	Firewall bool `json:"firewall,omitempty"`
	// Where is where Setup last played: whereDevice, whereWifi or whereOnline; where() is the one this build offers.
	Where int `json:"where,omitempty"`
	// Teams is the Teams add-on, and Team each seat's team, below core.MaxTeams; teams may be any size.
	Teams bool    `json:"teamsOn,omitempty"`
	Team  []uint8 `json:"team,omitempty"`

	// Rolled: Map is a roll of Setup's die, which keeps the size picker. rolled is that map, so choosing any other
	// map ends it without a reset.
	Rolled bool `json:"rolled,omitempty"`

	seats      []int // parsed Seats
	rolled     *core.MapDefinition
	mapChecked *core.MapDefinition
	mapOK      bool
}

// rejoin is a joiner's seat in a room: its code, the host's address on the LAN ("" online) and the hub's rejoin
// token, hex.
type rejoin struct {
	Code  string `json:"code"`
	Addr  string `json:"addr,omitempty"`
	Token string `json:"token"`
}

// maxName caps a player's name in bytes: within the wire's proto.MaxName, and short enough for a seat pill.
const maxName = 16

// isRolled reports whether the map in use is one Setup's die rolled, not one chosen from the library.
func (s *settings) isRolled() bool { return s.Map != nil && s.Map == s.rolled }

// name is what this player goes by: the name they gave on Home, or Human.
func (s *settings) name() string { return cmp.Or(strings.TrimSpace(s.Name), seatLabels[seatHuman]) }

// label is what seat i of seats is called on this device: this player's name on the first human seat (the one they
// take in a room they host), and every other seat's type.
func (s *settings) label(seats []int, i int) string {
	if i == firstHuman(seats) {
		return s.name()
	}
	return seatLabels[seats[i]]
}

var themeNames = [2]string{ui.Light: "light", ui.Dark: "dark"}

// themeIndex returns the theme saved as name; light unless it says dark.
func themeIndex(name string) int {
	if name == themeNames[ui.Dark] {
		return ui.Dark
	}
	return ui.Light
}

// Seat types: index 0 is Human, 1+s is AI strategy s.
const seatHuman = 0

var seatLabels = func() []string {
	labels := []string{"Human"}
	for strategy := range ai.NumStrategies {
		name := strategy.String()
		labels = append(labels, string(name[0]-'a'+'A')+name[1:])
	}
	return labels
}()

func seatName(opt int) string {
	if opt == seatHuman {
		return "human"
	}
	return ai.Strategy(opt - 1).String()
}

func parseSeat(name string) (int, bool) {
	if name == "human" {
		return seatHuman, true
	}
	s, ok := ai.Parse(name)
	return 1 + int(s), ok
}

// loadSettings reads saved settings, falling back to defaults for anything missing or out of range.
func loadSettings(rng *core.RNG) *settings {
	// Defaults first: Unmarshal only overwrites the fields a saved file has.
	s := &settings{Cols: 9, Rows: 9, Players: 2, Volume: 80}
	if data, err := platform.Load(settingsFile); err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			log.Printf("settings: %v (using defaults)", err)
		}
	}
	s.Volume = max(0, min(100, s.Volume))
	s.Cols = clampInt(s.Cols, core.MinSize, core.MaxSize, 9)
	s.Rows = clampInt(s.Rows, core.MinSize, core.MaxSize, 9)
	s.Players = clampInt(s.Players, core.MinPlayers, core.MaxPlayers, 2)
	if s.Map != nil {
		if data, err := platform.Load("maps.json"); err == nil && !s.Rolled { // a roll is no library map, whatever its name
			library := mapstore.Load(func(string) ([]byte, error) { return data, nil }, platform.Save)
			if i := s.selectedMapIndex(library.Maps); library.LoadErr == nil && i >= 0 && len(library.Maps[i].Validate()) == 0 {
				m := library.Maps[i].Clone()
				s.Map = &m
			}
		}
		if s.Map.ValidateStructure() != nil || len(s.Map.Validate()) != 0 {
			s.Map = nil
		} else {
			s.Cols, s.Rows = s.Map.Cols, s.Map.Rows
			if s.Rolled {
				s.rolled = s.Map
			}
		}
	}
	if len(s.Name) > maxName {
		s.Name = ""
	}
	for _, name := range s.Seats {
		opt, ok := parseSeat(name)
		if !ok {
			s.seats = nil
			break
		}
		s.seats = append(s.seats, opt)
	}
	if s.seats == nil {
		s.shuffleSeats(rng)
	} else {
		s.fitSeats()
	}
	s.fitTeams()
	return s
}

// Selections store a map copy; only a unique library name identifies its source safely.
func (s *settings) selectedMapIndex(maps []core.MapDefinition) int {
	if s.Map == nil || s.Map.Name == "" {
		return -1
	}
	index := -1
	for i, m := range maps {
		if m.Name == s.Map.Name {
			if index >= 0 {
				return -1
			}
			index = i
		}
	}
	return index
}

func clampInt(v, lo, hi, def int) int {
	if v == 0 {
		return def
	}
	return max(lo, min(hi, v))
}

// shuffleSeats makes player 1 Human and fills the other seats with the AIs in random order, repeating the order
// when there are more seats than AIs (prototype set_player_count with randomize, but player 1 is always the
// human, as the brief specifies).
func (s *settings) shuffleSeats(rng *core.RNG) {
	s.seats = append(s.seats[:0], seatHuman)
	for len(s.seats) < s.Players {
		s.seats = append(s.seats, 1)
	}
	s.rollBots(rng)
}

// rollBots gives every bot seat an AI in random order, none twice until every AI has a seat.
func (s *settings) rollBots(rng *core.RNG) {
	order := make([]int, ai.NumStrategies)
	for i := range order {
		order[i] = 1 + i
	}
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	k := 0
	for i, opt := range s.seats {
		if opt != seatHuman {
			s.seats[i] = order[k%len(order)]
			k++
		}
	}
}

// dealTeams deals a team game's seats at random into the teams that have players, evenly: equal teams, or one apart
// where the players don't divide.
func (s *settings) dealTeams(rng *core.RNG) {
	if !s.Teams {
		return
	}
	s.fitTeams()
	var in [core.MaxTeams]bool
	for _, t := range s.Team {
		in[t] = true
	}
	var used []uint8
	for t, ok := range in {
		if ok {
			used = append(used, uint8(t))
		}
	}
	if len(used) < 2 { // every player on one team: A joins it, or B joins A
		used = append(used, 1-min(used[0], 1))
	}
	order := make([]int, s.Players)
	for i := range order {
		order[i] = i
	}
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	for i, seat := range order {
		s.Team[seat] = used[i%len(used)]
	}
}

// fitSeats trims or extends the seats to the player count, extending with the AIs in dropdown order (prototype
// ensure_player_selections_length).
func (s *settings) fitSeats() {
	for len(s.seats) < s.Players {
		s.seats = append(s.seats, 1+(len(s.seats)-1+int(ai.NumStrategies))%int(ai.NumStrategies))
	}
	s.seats = s.seats[:s.Players]
}

// fitTeams gives every seat of a team game a team: seats keep theirs, whatever the teams' sizes, and a new seat joins
// the emptiest team that has players, A and B always counting.
func (s *settings) fitTeams() {
	if !s.Teams {
		return
	}
	s.Team = s.Team[:min(len(s.Team), s.Players)]
	var size [core.MaxTeams]int
	for i, t := range s.Team {
		if t >= core.MaxTeams { // a saved line-up this build can't hold
			s.Team[i], t = 0, 0
		}
		size[t]++
	}
	for len(s.Team) < s.Players {
		t := 0
		for k := 1; k < core.MaxTeams; k++ {
			if (k < 2 || size[k] > 0) && size[k] < size[t] {
				t = k
			}
		}
		s.Team = append(s.Team, uint8(t))
		size[t]++
	}
}

// rivals reports whether the seats can play each other: always with Teams off, else once two teams have players.
func (s *settings) rivals() bool {
	for _, t := range s.Team {
		if s.Teams && t != s.Team[0] {
			return true
		}
	}
	return !s.Teams
}

func (s *settings) save() {
	s.Rolled = s.isRolled()
	s.Seats = s.Seats[:0]
	for _, opt := range s.seats {
		s.Seats = append(s.Seats, seatName(opt))
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		err = platform.Save(settingsFile, data)
	}
	if err != nil {
		log.Print(fmt.Errorf("settings: save: %w", err))
	}
}

// where is where Setup plays: the saved choice, or This Device when this build can no longer offer it.
func (s *settings) where() int {
	if s.Where < whereDevice || s.Where > whereOnline || whereOff()&(1<<s.Where) != 0 {
		return whereDevice
	}
	return s.Where
}
