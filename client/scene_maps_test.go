package client

import (
	"errors"
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
	save := s.saveAt(setupTestApp(390, 844, 1, 2), -1)
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

func TestSavedPortalEditsReachNormalPlayAndTest(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	m := core.NewMap(13, 10)
	m.Name = "Doritos"
	for _, cell := range []int{37, 92} {
		m.Cells[cell] = core.MapCell{Feature: core.Portal, Group: 3, PortalMode: core.PortalHorizontal}
	}
	store := mapstore.Load(
		func(string) ([]byte, error) { return []byte(`{"version":1,"maps":[]}`), nil },
		func(string, []byte) error { return nil },
	)
	store.Maps = []core.MapDefinition{m.Clone()}
	s := &mapsScene{back: newHome(), selected: 0, store: store}
	s.choose(a, &m)
	s.edit(a, m, 0)
	e := a.next.(*mapEditor)
	for _, cell := range []int{37, 92} {
		e.m.Cells[cell].Group, e.m.Cells[cell].PortalMode = 1, core.PortalBoth
	}
	if err := e.save(e.m); err != nil {
		t.Fatal(err)
	}
	normal, err := newPlay(a, 13, 10, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	testSetup := newMapTestSetup(a, e, e.m)
	if testSetup.settings.Map.Cells[37].PortalMode != core.PortalBoth {
		t.Fatal("Test did not use the current editor definition")
	}
	for _, cell := range []int{37, 92} {
		if normal.game.Map.Cells[cell] != testSetup.settings.Map.Cells[cell] || normal.game.Geo.PortalAxes[cell] != core.BothAxes {
			t.Fatalf("normal play retained a stale Portal: cell %d = %+v, axes %v", cell, normal.game.Map.Cells[cell], normal.game.Geo.PortalAxes[cell])
		}
	}
	if m.Cells[37].PortalMode != core.PortalHorizontal {
		t.Fatal("saving mutated the prior map definition")
	}
	before := normal.game.Hash()
	e.m.Cells[37].PortalMode = core.PortalVertical
	if normal.game.Hash() != before || a.set.Map.Cells[37].PortalMode != core.PortalBoth {
		t.Fatal("the editor still aliases a selected or ongoing map")
	}
}

func TestFailedMapSaveKeepsSelectedMap(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	m := core.NewMap(7, 7)
	m.Name = "Doritos"
	a.set.Map = &m
	store := mapstore.Load(
		func(string) ([]byte, error) { return []byte(`{"version":1,"maps":[]}`), nil },
		func(string, []byte) error { return errors.New("storage unavailable") },
	)
	store.Maps = []core.MapDefinition{m.Clone()}
	s := &mapsScene{store: store}
	edited := m.Clone()
	edited.Name = "Renamed Doritos"
	if err := s.saveAt(a, 0)(edited); err == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	if a.set.Map != &m || a.set.Map.Name != "Doritos" || store.Maps[0].Name != "Doritos" {
		t.Fatal("failed storage changed the selected or saved definition")
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
