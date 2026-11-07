package client

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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

	seats      []int // parsed Seats
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

var themeNames = [2]string{themeLight: "light", themeDark: "dark"}

// themeIndex returns the theme saved as name; light unless it says dark.
func themeIndex(name string) int {
	if name == themeNames[themeDark] {
		return themeDark
	}
	return themeLight
}

func aiNames() []string {
	n := make([]string, ai.NumStrategies)
	for s := range ai.NumStrategies {
		n[s] = s.String()
	}
	return n
}

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
		if s.Map.ValidateStructure() != nil || len(s.Map.Validate()) != 0 {
			s.Map = nil
		} else {
			s.Cols, s.Rows = s.Map.Cols, s.Map.Rows
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
	return s
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
	order := make([]int, ai.NumStrategies)
	for i := range order {
		order[i] = 1 + i
	}
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	s.seats = s.seats[:0]
	s.seats = append(s.seats, seatHuman)
	for i := 1; i < s.Players; i++ {
		s.seats = append(s.seats, order[(i-1)%len(order)])
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

func (s *settings) save() {
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
