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
	m.Cells[6] = MapCell{Circuit: Latch, Group: 1}
	m.Cells[18] = MapCell{Circuit: Latch, Group: 1}
	m.Edges = []MapEdge{{A: 12, B: 13, Kind: Gate, Group: 1}}
	// Build the actual old two-byte cells, rather than relabelling a new file.
	payload := []byte{5, 5, byte(len(m.Name))}
	payload = append(payload, m.Name...)
	for _, c := range m.Cells {
		feature := byte(c.Feature)
		if c.Circuit.IsSwitch() {
			feature = byte(c.Circuit) + 2
		}
		payload = append(payload, byte(c.Terrain)|feature<<4, c.Group)
	}
	payload = binary.BigEndian.AppendUint16(payload, 1)
	payload = append(payload, 0, 12, 0, 13, byte(Gate), 1)
	b := mapTestEnvelope(payload, len(payload))
	b[4] = 1
	read, err := DecodeMap(b)
	if err != nil || !reflect.DeepEqual(read, m) {
		t.Fatalf("legacy draft lost: %v", err)
	}
	if len(read.Validate()) != 0 {
		t.Fatal("legacy multiple controllers should now be playable")
	}
	code := "orbitrix-map1." + base64.RawURLEncoding.EncodeToString(b)
	if _, err := ParseMapCode(code); err != nil {
		t.Fatal(err)
	}
	b[4] = 2
	if got, err := ParseMapCode("orbitrix-map2." + base64.RawURLEncoding.EncodeToString(b)); err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("format 2 lost: %v", err)
	}
	if encoded, err := EncodeMap(m); err != nil || !bytes.Equal(encoded, b) {
		t.Fatalf("existing format-2 encoding changed: %v", err)
	}
	// Format 2 also carried Toggle and Hold as feature ordinals 4 and 5.
	payload[3+len(m.Name)+2*6] = 4 << 4
	payload[3+len(m.Name)+2*18] = 5 << 4
	b = mapTestEnvelope(payload, len(payload))
	b[4] = 2
	m.Cells[6].Circuit, m.Cells[18].Circuit = Toggle, Hold
	if got, err := DecodeMap(b); err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("format 2 switch types lost: %v", err)
	}
	b[4] = 1
	if _, err := DecodeMap(b); err == nil {
		t.Fatal("accepted new switch types in format 1")
	}
}

func TestMapCodecRoundTrip(t *testing.T) {
	draft := NewMap(5, 7)
	draft.Name = "Ana's map — draft"
	draft.Cells[0] = MapCell{Terrain: Floor, Feature: Link, Group: 3} // unpaired draft Link
	draft.Cells[4] = MapCell{Terrain: Floor, Circuit: Latch, Group: 2}
	draft.Cells[10] = MapCell{Terrain: Floor, Feature: Reservoir}
	draft.Cells[11].Terrain = BreakableWall
	draft.Cells[12].Terrain = Void
	draft.Cells[13].Terrain = Wall
	draft.Cells[15] = MapCell{Circuit: Door, Group: 2, InitiallyOpen: true}
	draft.Cells[19] = MapCell{Feature: Portal, Group: 1}
	draft.Cells[20] = MapCell{Feature: Portal, Group: 2, PortalMode: PortalHorizontal}
	draft.Cells[21] = MapCell{Feature: Portal, Group: 3, PortalMode: PortalVertical}
	draft.Edges = []MapEdge{{A: 1, B: 2, Kind: Barrier}, {A: 2, B: 3, Kind: Gate, Group: 2}, {A: 3, B: 4, Kind: BreakableBarrier}}
	draft.Edges[1].InitiallyOpen = true
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
			if b[4] != 2 {
				t.Fatal("map format was bumped beyond the existing development version")
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
			if !strings.HasPrefix(code, "orbitrix-map2.") {
				t.Fatal("share-code format was bumped")
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

func TestTemporaryCircuitMapImportExportsAsFormatTwo(t *testing.T) {
	m := NewMap(3, 3)
	m.Cells[0] = MapCell{Circuit: Latch, Group: 1}
	m.Cells[4] = MapCell{Circuit: Door, Group: 1, InitiallyOpen: true}
	m.Edges = []MapEdge{{A: 1, B: 2, Kind: Gate, Group: 1, InitiallyOpen: true}}
	payload := []byte{3, 3, byte(len(m.Name))}
	payload = append(payload, m.Name...)
	for _, c := range m.Cells {
		flags := byte(0)
		if c.InitiallyOpen {
			flags = 1
		}
		payload = append(payload, byte(c.Terrain)|byte(c.Feature)<<4, c.Group, byte(c.Circuit), flags)
	}
	payload = binary.BigEndian.AppendUint16(payload, 1)
	payload = append(payload, 0, 1, 0, 2, byte(Gate), 1, 1)
	b := mapTestEnvelope(payload, len(payload))
	b[4] = 3
	read, err := ParseMapCode("orbitrix-map3." + base64.RawURLEncoding.EncodeToString(b))
	if err != nil || !reflect.DeepEqual(read, m) {
		t.Fatalf("temporary export lost: %v", err)
	}
	encoded, err := EncodeMap(read)
	if err != nil || encoded[4] != 2 {
		t.Fatalf("temporary map did not export as format 2: %v", err)
	}
	if got, err := DecodeMap(encoded); err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("converted map lost state: %v", err)
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
		"future version":       mutated(func(b []byte) { b[4] = 4 }),
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
		"short dimensions":             mapTestEnvelope([]byte{3, 3}, 2),
		"invalid dimensions":           mutated(func(b []byte) { b[0] = 255 }),
		"unknown terrain":              mutated(func(b []byte) { b[3] = 15 }),
		"reserved terrain bit":         mutated(func(b []byte) { b[3] = 4 }),
		"unknown feature":              mutated(func(b []byte) { b[3] = 15<<4 | byte(Floor) }),
		"initial state without output": mutated(func(b []byte) { b[3] = 8 }),
		"incorrect edge count":         mutated(func(b []byte) { b[len(b)-1] = 1 }),
		"trailing payload":             mapTestEnvelope(append(append([]byte(nil), payload...), 0), len(payload)+1),
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
	m = NewMap(3, 3)
	m.Cells[4] = MapCell{Feature: Link, Circuit: Door, Group: 1}
	if _, err := EncodeMap(m); err == nil {
		t.Fatal("overlapping components must not be silently discarded")
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
