package mapstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"orbitrix/core"
)

func TestMapLibraryRoundTripAllowsDrafts(t *testing.T) {
	maps := core.StarterMaps()
	draft := core.NewMap(3, 3)
	draft.Name = "Unfinished"
	draft.Cells[4] = core.MapCell{Feature: core.Link, Group: 1}
	maps = append(maps, draft)
	b, err := encodeMapLibrary(maps)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeMapLibrary(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(maps) || got[3].Name != draft.Name || !slices.Equal(got[3].Cells, draft.Cells) {
		t.Fatalf("round trip changed maps: %v", got)
	}
	if len(got[3].Validate()) == 0 {
		t.Fatal("fixture must remain an unfinished draft")
	}
}

func TestMapLibraryRejectsMalformedStorage(t *testing.T) {
	valid, err := encodeMapLibrary(core.StarterMaps())
	if err != nil {
		t.Fatal(err)
	}
	badMap := core.NewMap(3, 3)
	badMap.Cells = nil
	badMapJSON, _ := json.Marshal(mapLibrary{Version: 1, Maps: []core.MapDefinition{badMap}})
	tooManyJSON, _ := json.Marshal(mapLibrary{Version: 1, Maps: make([]core.MapDefinition, maxLibraryMaps+1)})
	for _, input := range []string{
		"", "{}", "null", "[]", "{", `{"version":2,"maps":[]}`, `{"version":1,"unexpected":true}`,
		string(valid) + "null", string(badMapJSON), string(tooManyJSON), strings.Repeat(" ", maxLibraryBytes+1),
	} {
		if _, err := decodeMapLibrary([]byte(input)); err == nil {
			t.Fatalf("accepted malformed library (first 80 bytes): %.80q", input)
		}
	}
	if _, err := encodeMapLibrary(make([]core.MapDefinition, maxLibraryMaps+1)); err == nil {
		t.Fatal("encoded too many maps")
	}
}

func TestMapLibrarySeedsOnlyMissingFiles(t *testing.T) {
	write := func(string, []byte) error { t.Fatal("loading a library must not write"); return nil }
	missing := Load(func(string) ([]byte, error) { return nil, fs.ErrNotExist }, write)
	if missing.LoadErr != nil || len(missing.Maps) != 3 {
		t.Fatal("missing library did not provide starter maps")
	}
	empty := Load(func(string) ([]byte, error) { return []byte(`{"version":1,"maps":[]}`), nil }, write)
	if empty.LoadErr != nil || len(empty.Maps) != 0 {
		t.Fatal("valid empty library was replaced with starters")
	}
	corrupt := Load(func(string) ([]byte, error) { return []byte("damaged"), nil }, write)
	if corrupt.LoadErr == nil || len(corrupt.Maps) != 0 {
		t.Fatal("corrupt library was silently replaced")
	}
	if err := corrupt.Put(-1, core.NewMap(3, 3)); err == nil {
		t.Fatal("corrupt library allowed overwriting")
	}
	denied := Load(func(string) ([]byte, error) { return nil, fs.ErrPermission }, write)
	if denied.LoadErr == nil {
		t.Fatal("permission failure was treated as missing file")
	}
}

func TestMapStoreWritesAreTransactionalAndIsolated(t *testing.T) {
	var saved []byte
	fail := false
	s := &Store{Maps: []core.MapDefinition{core.NewMap(3, 3)}, write: func(name string, data []byte) error {
		if name != "maps.json" {
			t.Fatalf("unexpected file %q", name)
		}
		if fail {
			return fs.ErrPermission
		}
		saved = slices.Clone(data)
		return nil
	}}
	m := core.NewMap(4, 4)
	m.Name = "New map"
	if err := s.Put(-1, m); err != nil {
		t.Fatal(err)
	}
	m.Cells[0].Terrain = core.Void
	if s.Maps[1].Cells[0].Terrain != core.Floor {
		t.Fatal("library aliases a mutable draft")
	}
	if got, err := decodeMapLibrary(saved); err != nil || len(got) != 2 {
		t.Fatalf("stored library invalid: %v", err)
	}
	fail = true
	if err := s.Put(0, m); !errors.Is(err, fs.ErrPermission) || s.Maps[0].Cols != 3 {
		t.Fatal("failed replace changed in-memory library")
	}
	if err := s.Remove(0); !errors.Is(err, fs.ErrPermission) || len(s.Maps) != 2 {
		t.Fatal("failed remove changed in-memory library")
	}
	fail = false
	if err := s.Remove(0); err != nil || len(s.Maps) != 1 || s.Maps[0].Cols != 4 {
		t.Fatalf("remove failed: %v", err)
	}
	for _, i := range []int{-2, 2} {
		if s.Put(i, m) == nil || s.Remove(i) == nil {
			t.Fatalf("accepted invalid library index %d", i)
		}
	}
}
