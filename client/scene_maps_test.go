package client

import (
	"strings"
	"testing"
	"unicode/utf8"

	"orbitrix/client/internal/mapstore"
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
	store := mapstore.Load(
		func(string) ([]byte, error) { return []byte(`{"version":1,"maps":[]}`), nil },
		func(string, []byte) error { return nil },
	)
	s := &mapsScene{selected: -1, store: store}
	save := s.saveAt(-1)
	m := core.NewMap(3, 3)
	if err := save(m); err != nil {
		t.Fatal(err)
	}
	m.Name = "Renamed draft"
	if err := save(m); err != nil {
		t.Fatal(err)
	}
	if len(s.store.Maps) != 1 || s.store.Maps[0].Name != m.Name || s.selected != 0 {
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
