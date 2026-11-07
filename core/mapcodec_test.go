package core

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyMapImport(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[6] = MapCell{Feature: Switch, Group: 1}
	m.Cells[18] = MapCell{Feature: Switch, Group: 1}
	m.Edges = []MapEdge{{A: 12, B: 13, Kind: Gate, Group: 1}}
	b, err := EncodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	b[4] = 1
	read, err := DecodeMap(b)
	if err != nil || !reflect.DeepEqual(read, m) {
		t.Fatalf("legacy draft lost: %v", err)
	}
	if len(read.Validate()) == 0 {
		t.Fatal("legacy multiple controllers must remain an editable draft")
	}
	code := "orbitrix-map1." + base64.RawURLEncoding.EncodeToString(b)
	if _, err := ParseMapCode(code); err != nil {
		t.Fatal(err)
	}
}

func TestMapCodecRoundTrip(t *testing.T) {
	draft := NewMap(5, 7)
	draft.Name = "Ana's map — draft"
	draft.Cells[0] = MapCell{Terrain: Floor, Feature: Link, Group: 3} // unpaired draft Link
	draft.Cells[4] = MapCell{Terrain: Floor, Feature: Switch, Group: 2}
	draft.Cells[10] = MapCell{Terrain: Floor, Feature: Reservoir}
	draft.Cells[11].Terrain = BreakableWall
	draft.Cells[12].Terrain = Void
	draft.Cells[13].Terrain = Wall
	draft.Edges = []MapEdge{{A: 1, B: 2, Kind: Barrier}, {A: 2, B: 3, Kind: Gate, Group: 2}}
	large := NewMap(MaxSize, MaxSize)
	legacyGroups := NewMap(5, 5)
	legacyGroups.Name = "Extra group draft"
	legacyGroups.Cells[6], legacyGroups.Cells[18] = MapCell{Feature: Link, Group: 13}, MapCell{Feature: Link, Group: 13}
	large.Name = strings.Repeat("m", 64)
	for i := range large.Cells {
		if i%3 == 0 {
			large.Cells[i].Feature = Reservoir
		}
		if i%MaxSize != MaxSize-1 {
			large.Edges = append(large.Edges, MapEdge{A: i, B: i + 1, Kind: Barrier})
		}
		if i+MaxSize < len(large.Cells) {
			large.Edges = append(large.Edges, MapEdge{A: i, B: i + MaxSize, Kind: Gate, Group: 1})
		}
	}
	for _, m := range []MapDefinition{NewMap(3, 3), draft, large, legacyGroups} {
		t.Run(m.Name, func(t *testing.T) {
			before := m.Clone()
			b, err := EncodeMap(m)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeMap(b)
			if err != nil || !reflect.DeepEqual(m, got) {
				t.Fatalf("round trip mismatch: %v", err)
			}
			if !reflect.DeepEqual(before, m) {
				t.Fatal("encoding mutated input")
			}
			code, err := MapCode(m)
			if err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{code, " \n" + code + "\n", "https://orbitrix.example/play#map=" + code, "https://example/play#other=1&map=" + code + "&after=2"} {
				got, err := ParseMapCode(text)
				if err != nil || !reflect.DeepEqual(m, got) {
					t.Fatalf("code round trip mismatch: %v", err)
				}
			}
		})
	}
}

func TestMapCodecRejectsInvalidFiles(t *testing.T) {
	valid, err := EncodeMap(NewMap(3, 3))
	if err != nil {
		t.Fatal(err)
	}
	mutated := func(change func([]byte)) []byte {
		b := append([]byte(nil), valid...)
		change(b)
		return b
	}
	for name, b := range map[string][]byte{
		"empty":                nil,
		"header":               valid[:mapHeaderBytes-1],
		"truncated stream":     valid[:len(valid)-1],
		"wrong magic":          mutated(func(b []byte) { b[0] = 'X' }),
		"future version":       mutated(func(b []byte) { b[4]++ }),
		"unknown compression":  mutated(func(b []byte) { b[5]++ }),
		"checksum":             mutated(func(b []byte) { b[10] ^= 1 }),
		"incorrect raw length": mutated(func(b []byte) { binary.BigEndian.PutUint32(b[6:10], 1) }),
		"oversized raw length": mutated(func(b []byte) { binary.BigEndian.PutUint32(b[6:10], MaxMapBytes+1) }),
		"trailing stream data": append(append([]byte(nil), valid...), 0),
		"oversized file":       make([]byte, MaxMapBytes+1),
		"decompression bomb":   mapTestEnvelope(bytes.Repeat([]byte{0}, MaxMapBytes+1), MaxMapBytes),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeMap(b); err == nil {
				t.Fatal("accepted invalid map")
			}
		})
	}
}

func TestMapCodecRejectsInvalidContents(t *testing.T) {
	// An otherwise valid minimal payload: dimensions, name length, cells, edge count.
	payload := make([]byte, 3+2*9+2)
	payload[0], payload[1] = 3, 3
	for i := range 9 {
		payload[3+2*i] = byte(Floor)
	}
	mutated := func(change func([]byte)) []byte {
		b := append([]byte(nil), payload...)
		change(b)
		return mapTestEnvelope(b, len(b))
	}
	for name, b := range map[string][]byte{
		"short dimensions":     mapTestEnvelope([]byte{3, 3}, 2),
		"invalid dimensions":   mutated(func(b []byte) { b[0] = 255 }),
		"unknown terrain":      mutated(func(b []byte) { b[3] = 15 }),
		"unknown feature":      mutated(func(b []byte) { b[3] = 15<<4 | byte(Floor) }),
		"incorrect edge count": mutated(func(b []byte) { b[len(b)-1] = 1 }),
		"trailing payload":     mapTestEnvelope(append(append([]byte(nil), payload...), 0), len(payload)+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeMap(b); err == nil {
				t.Fatal("accepted invalid contents")
			}
		})
	}
	if _, err := DecodeMap(mapTestEnvelope(payload, len(payload))); err != nil {
		t.Fatalf("valid contents rejected: %v", err)
	}
}

func TestMapCodeRejectsInvalidText(t *testing.T) {
	for _, code := range []string{"", "orbitrix-map2.a", "orbitrix-map1.%%", "orbitrix-map1.A", "https://example/play#map=bad", "https://example/play#other=abc", strings.Repeat("a", 2*MaxMapBytes+1)} {
		if _, err := ParseMapCode(code); err == nil {
			t.Fatalf("accepted invalid share text %.40q", code)
		}
	}
	m := NewMap(3, 3)
	m.Cells = nil
	if _, err := EncodeMap(m); err == nil {
		t.Fatal("encoded structurally invalid map")
	}
	if _, err := MapCode(m); err == nil {
		t.Fatal("made code for structurally invalid map")
	}
}

func mapTestEnvelope(payload []byte, size int) []byte {
	var b bytes.Buffer
	b.WriteString("ORBM")
	b.Write([]byte{MapFormatVersion, 1})
	b.Write(binary.BigEndian.AppendUint32(nil, uint32(size)))
	b.Write(binary.BigEndian.AppendUint32(nil, crc32.ChecksumIEEE(payload)))
	w := zlib.NewWriter(&b)
	w.Write(payload)
	w.Close()
	return b.Bytes()
}

func FuzzMapDecode(f *testing.F) {
	valid, err := EncodeMap(NewMap(3, 3))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add([]byte("ORBM"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := DecodeMap(b)
		if err != nil {
			return
		}
		encoded, err := EncodeMap(m)
		if err != nil {
			t.Fatalf("decoded map cannot encode: %v", err)
		}
		got, err := DecodeMap(encoded)
		if err != nil || !reflect.DeepEqual(m, got) {
			t.Fatalf("decoded map cannot round trip: %v", err)
		}
	})
}
