// Package mapstore validates and persists the local map library.
package mapstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"orbitrix/core"
)

const maxLibraryMaps, maxLibraryBytes = 128, 4 << 20

type mapLibrary struct {
	Version int                  `json:"version"`
	Maps    []core.MapDefinition `json:"maps"`
}

// Store only commits its in-memory list after the persistent write succeeds.
// A damaged library remains read-only instead of being replaced by an empty one.
type Store struct {
	Maps    []core.MapDefinition
	LoadErr error
	write   func(string, []byte) error
}

func decodeMapLibrary(b []byte) ([]core.MapDefinition, error) {
	if len(b) > maxLibraryBytes {
		return nil, fmt.Errorf("map library exceeds 4 MiB")
	}
	var saved mapLibrary
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&saved); err != nil {
		return nil, fmt.Errorf("read map library: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("read map library: trailing contents")
	}
	if saved.Version != 1 || len(saved.Maps) > maxLibraryMaps {
		return nil, fmt.Errorf("unsupported map library version or too many maps")
	}
	for i, m := range saved.Maps {
		if err := m.ValidateStructure(); err != nil {
			return nil, fmt.Errorf("read map %d: %w", i+1, err)
		}
	}
	return saved.Maps, nil
}

func encodeMapLibrary(maps []core.MapDefinition) ([]byte, error) {
	if len(maps) > maxLibraryMaps {
		return nil, fmt.Errorf("map library is full; export and remove a map first")
	}
	for _, m := range maps {
		if err := m.ValidateStructure(); err != nil {
			return nil, err
		}
	}
	b, err := json.Marshal(mapLibrary{Version: 1, Maps: maps})
	if err != nil {
		return nil, fmt.Errorf("save map library: %w", err)
	}
	if len(b) > maxLibraryBytes {
		return nil, fmt.Errorf("map library exceeds 4 MiB; export and remove a map first")
	}
	return b, nil
}

// Load reads the library, seeding starter maps only when the file is missing.
func Load(read func(string) ([]byte, error), write func(string, []byte) error) *Store {
	s := &Store{write: write}
	b, err := read("maps.json")
	if errors.Is(err, fs.ErrNotExist) {
		s.Maps = core.StarterMaps()
		return s
	}
	if err != nil {
		s.LoadErr = fmt.Errorf("could not open saved maps: %w", err)
		return s
	}
	s.Maps, s.LoadErr = decodeMapLibrary(b)
	return s
}

func (s *Store) commit(maps []core.MapDefinition) error {
	if s.LoadErr != nil {
		return fmt.Errorf("saved map library could not be read; it was left untouched: %w", s.LoadErr)
	}
	b, err := encodeMapLibrary(maps)
	if err != nil {
		return err
	}
	if err := s.write("maps.json", b); err != nil {
		return fmt.Errorf("save maps: %w", err)
	}
	s.Maps = maps
	return nil
}

// Put appends at -1, or replaces one existing entry without aliasing its draft.
func (s *Store) Put(index int, m core.MapDefinition) error {
	if index < -1 || index >= len(s.Maps) {
		return fmt.Errorf("saved map no longer exists")
	}
	maps := append([]core.MapDefinition(nil), s.Maps...)
	if index < 0 {
		maps = append(maps, m.Clone())
	} else {
		maps[index] = m.Clone()
	}
	return s.commit(maps)
}

// Remove commits a library without the entry at index.
func (s *Store) Remove(index int) error {
	if index < 0 || index >= len(s.Maps) {
		return fmt.Errorf("saved map no longer exists")
	}
	maps := make([]core.MapDefinition, 0, len(s.Maps)-1)
	maps = append(maps, s.Maps[:index]...)
	maps = append(maps, s.Maps[index+1:]...)
	return s.commit(maps)
}
