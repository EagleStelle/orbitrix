package core

import (
	"reflect"
	"testing"
)

func TestRandomMap(t *testing.T) {
	pieces, makers := 0, 0
	for _, size := range [][2]int{{3, 3}, {5, 7}, {9, 9}, {13, 18}, {40, 40}, {3, 40}} {
		for seed := range uint64(40) {
			if size[0] == 40 && seed > 3 {
				break
			}
			m := RandomMap(size[0], size[1], seed)
			if issues := m.Validate(); len(issues) != 0 {
				t.Fatalf("%v seed %d: %v", size, seed, issues[0].Message)
			}
			if _, _, pace := m.Estimate(); pace != Fast && pace != Normal {
				t.Fatalf("%v seed %d: pace %d, want Fast or Normal", size, seed, pace)
			}
			if m.FloorCount() < min(len(m.Cells), MaxPlayers) {
				t.Fatalf("%v seed %d: room for %d seats only", size, seed, m.FloorCount())
			}
			if !reflect.DeepEqual(m, RandomMap(size[0], size[1], seed)) {
				t.Fatalf("%v seed %d: two rolls differ", size, seed)
			}
			data, err := EncodeMap(m)
			if err != nil {
				t.Fatalf("%v seed %d: %v", size, seed, err)
			}
			if back, err := DecodeMap(data); err != nil || len(back.Validate()) != 0 {
				t.Fatalf("%v seed %d: does not survive saving: %v", size, seed, err)
			}
			if _, err := NewGameWithMap(m, 2); err != nil {
				t.Fatalf("%v seed %d: %v", size, seed, err)
			}
			for _, c := range m.Cells {
				if c != (MapCell{}) {
					pieces++
				}
				if c.Feature == Furnace || c.Feature == Reservoir || c.Feature == Turret {
					makers++
				}
			}
			pieces += len(m.Edges)
		}
	}
	if pieces == 0 || makers*4 < pieces {
		t.Fatalf("%d pieces, %d of them make orbs: want at least a quarter", pieces, makers)
	}
	t.Logf("%d pieces, %d make orbs", pieces, makers)
}

func BenchmarkRandomMap40(b *testing.B) {
	for i := range b.N {
		RandomMap(40, 40, uint64(i))
	}
}

func BenchmarkRandomMap30(b *testing.B) {
	for i := range b.N {
		RandomMap(30, 30, uint64(i))
	}
}
