package client

import (
	"encoding/json"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"orbitrix/core"
)

func TestMapImportCancelPreservesPreview(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	s := newMapImport(a, newHome(), func(core.MapDefinition) error { return nil })
	m := core.NewMap(9, 9)
	s.accept(m)
	preview := s.mapData
	s.waiting = true
	s.fileDone <- mapFileResult{}
	if err := s.update(a); err != nil {
		t.Fatal(err)
	}
	if s.waiting || s.mapData != preview {
		t.Fatal("canceling the picker discarded the import preview")
	}
}

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
	missing := loadMapStore(func(string) ([]byte, error) { return nil, fs.ErrNotExist }, write)
	if missing.loadErr != nil || len(missing.maps) != 3 {
		t.Fatal("missing library did not provide starter maps")
	}
	empty := loadMapStore(func(string) ([]byte, error) { return []byte(`{"version":1,"maps":[]}`), nil }, write)
	if empty.loadErr != nil || len(empty.maps) != 0 {
		t.Fatal("valid empty library was replaced with starters")
	}
	corrupt := loadMapStore(func(string) ([]byte, error) { return []byte("damaged"), nil }, write)
	if corrupt.loadErr == nil || len(corrupt.maps) != 0 {
		t.Fatal("corrupt library was silently replaced")
	}
	if err := corrupt.put(-1, core.NewMap(3, 3)); err == nil {
		t.Fatal("corrupt library allowed overwriting")
	}
	denied := loadMapStore(func(string) ([]byte, error) { return nil, fs.ErrPermission }, write)
	if denied.loadErr == nil {
		t.Fatal("permission failure was treated as missing file")
	}
}

func TestMapStoreWritesAreTransactionalAndIsolated(t *testing.T) {
	var saved []byte
	fail := false
	s := &mapStore{maps: []core.MapDefinition{core.NewMap(3, 3)}, write: func(name string, data []byte) error {
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
	if err := s.put(-1, m); err != nil {
		t.Fatal(err)
	}
	m.Cells[0].Terrain = core.Void
	if s.maps[1].Cells[0].Terrain != core.Floor {
		t.Fatal("library aliases a mutable draft")
	}
	if got, err := decodeMapLibrary(saved); err != nil || len(got) != 2 {
		t.Fatalf("stored library invalid: %v", err)
	}
	fail = true
	if err := s.put(0, m); !errors.Is(err, fs.ErrPermission) || s.maps[0].Cols != 3 {
		t.Fatal("failed replace changed in-memory library")
	}
	if err := s.remove(0); !errors.Is(err, fs.ErrPermission) || len(s.maps) != 2 {
		t.Fatal("failed remove changed in-memory library")
	}
	fail = false
	if err := s.remove(0); err != nil || len(s.maps) != 1 || s.maps[0].Cols != 4 {
		t.Fatalf("remove failed: %v", err)
	}
	for _, i := range []int{-2, 2} {
		if s.put(i, m) == nil || s.remove(i) == nil {
			t.Fatalf("accepted invalid library index %d", i)
		}
	}
}

func TestMapCanStartUsesPlayableCellsAndValidation(t *testing.T) {
	s := &settings{Cols: 3, Rows: 3, Players: 12}
	if mapCanStart(s) {
		t.Fatal("classic board accepted more players than cells")
	}
	s.Players = 2
	if !mapCanStart(s) {
		t.Fatal("classic board rejected two players")
	}
	m := core.NewMap(5, 5)
	s.Map = &m
	if !mapCanStart(s) {
		t.Fatal("valid custom map rejected")
	}
	s.Players = 26
	if mapCanStart(s) {
		t.Fatal("custom map accepted too many players")
	}
	s.Players = 2
	draft := m.Clone()
	draft.Cells[0] = core.MapCell{Feature: core.Link, Group: 1}
	s.Map = &draft
	if mapCanStart(s) {
		t.Fatal("unfinished map was playable")
	}
}

func TestMapSaveCallbackUpdatesExistingEntryAfterFirstSave(t *testing.T) {
	s := &mapsScene{selected: -1, store: &mapStore{write: func(string, []byte) error { return nil }}}
	save := s.saveAt(-1)
	m := core.NewMap(3, 3)
	if err := save(m); err != nil {
		t.Fatal(err)
	}
	m.Name = "Renamed draft"
	if err := save(m); err != nil {
		t.Fatal(err)
	}
	if len(s.store.maps) != 1 || s.store.maps[0].Name != m.Name || s.selected != 0 {
		t.Fatal("repeated Save duplicated the editor's map")
	}
}

func TestMapImportPreviewDoesNotSave(t *testing.T) {
	m := core.NewMap(3, 3)
	code, err := core.MapCode(m)
	if err != nil {
		t.Fatal(err)
	}
	saved := false
	s := &mapImportScene{save: func(core.MapDefinition) error { saved = true; return nil }}
	s.previewCode("https://example.com/play#map=" + code)
	if saved || s.mapData == nil || s.mapData.Cols != 3 {
		t.Fatal("preview saved automatically or lost map data")
	}
	s.previewCode("damaged-code")
	if saved || s.mapData != nil || s.status.message == "" {
		t.Fatal("failed import kept a stale preview")
	}
}

func TestMapCopyNameKeepsUTF8Bound(t *testing.T) {
	name := mapCopyName(strings.Repeat("界", 21))
	if len(name) > 64 || !utf8.ValidString(name) || !strings.HasSuffix(name, " (copy)") {
		t.Fatalf("bad copy name %q", name)
	}
}
