package core

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
)

const (
	MapFormatVersion = 2
	// MaxMapBytes bounds both the portable file and its decompressed contents.
	MaxMapBytes    = 64 * 1024
	mapCodePrefix  = "orbitrix-map2."
	mapHeaderBytes = 14
)

// EncodeMap makes a portable map file. Files, share codes and QR links all carry
// these same bytes. Only structural validity is required: drafts are shareable.
func EncodeMap(m MapDefinition) ([]byte, error) {
	if err := m.ValidateStructure(); err != nil {
		return nil, fmt.Errorf("encode map: %w", err)
	}
	if len(m.Edges) > 0xffff {
		return nil, fmt.Errorf("encode map: too many edges")
	}
	payload := make([]byte, 0, 5+len(m.Name)+2*len(m.Cells)+6*len(m.Edges))
	payload = append(payload, byte(m.Cols), byte(m.Rows), byte(len(m.Name)))
	payload = append(payload, m.Name...)
	for _, c := range m.Cells {
		// Keep legacy switch tags 3..5; Portal modes use 7..9, Spillway, Factory and Crusher 10..12, and bit 3
		// stores the initial state.
		component := byte(c.Feature)
		switch c.Feature {
		case Portal:
			component = 7 + byte(c.PortalMode)
		case Spillway, Factory, Crusher:
			component = 10 + byte(c.Feature-Spillway)
		}
		if c.Circuit != NoCircuit {
			component = byte(c.Circuit) + 2
		}
		cell := byte(c.Terrain) | component<<4
		if c.InitiallyOpen {
			cell |= 8
		}
		payload = append(payload, cell, c.Group)
	}
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(m.Edges)))
	for _, e := range m.Edges {
		payload = binary.BigEndian.AppendUint16(payload, uint16(e.A))
		payload = binary.BigEndian.AppendUint16(payload, uint16(e.B))
		kind := byte(e.Kind)
		if e.InitiallyOpen {
			kind |= 0x80
		}
		payload = append(payload, kind, e.Group)
	}
	if len(payload) > MaxMapBytes {
		return nil, fmt.Errorf("encode map: map exceeds %d bytes", MaxMapBytes)
	}
	var out bytes.Buffer
	out.WriteString("ORBM")
	out.WriteByte(MapFormatVersion)
	out.WriteByte(1) // compression: zlib
	out.Write(binary.BigEndian.AppendUint32(nil, uint32(len(payload))))
	out.Write(binary.BigEndian.AppendUint32(nil, crc32.ChecksumIEEE(payload)))
	w := zlib.NewWriter(&out)
	if _, err := w.Write(payload); err != nil {
		return nil, fmt.Errorf("encode map: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("encode map: %w", err)
	}
	if out.Len() > MaxMapBytes {
		return nil, fmt.Errorf("encode map: file exceeds %d bytes", MaxMapBytes)
	}
	return out.Bytes(), nil
}

// DecodeMap checks the envelope, decompression limit, checksum and every field
// before returning a map. Gameplay validation is a separate step before play.
func DecodeMap(b []byte) (MapDefinition, error) {
	bad := func(reason string) (MapDefinition, error) {
		return MapDefinition{}, fmt.Errorf("decode map: %s", reason)
	}
	if len(b) < mapHeaderBytes || len(b) > MaxMapBytes {
		return bad("file size out of range")
	}
	if string(b[:4]) != "ORBM" {
		return bad("not an Orbitrix map")
	}
	// Import the temporary circuit-build format too, without exporting it again.
	if b[4] != 1 && b[4] != MapFormatVersion && b[4] != 3 {
		return bad("unsupported map version")
	}
	if b[5] != 1 {
		return bad("unsupported compression")
	}
	size := binary.BigEndian.Uint32(b[6:10])
	if size > MaxMapBytes {
		return bad("decompressed map is too large")
	}
	source := bytes.NewReader(b[mapHeaderBytes:])
	r, err := zlib.NewReader(source)
	if err != nil {
		return bad("invalid compressed map")
	}
	payload, readErr := io.ReadAll(io.LimitReader(r, int64(size)+1))
	closeErr := r.Close()
	if readErr != nil || closeErr != nil || len(payload) != int(size) || source.Len() != 0 {
		return bad("invalid map length or trailing data")
	}
	if crc32.ChecksumIEEE(payload) != binary.BigEndian.Uint32(b[10:14]) {
		return bad("map checksum mismatch")
	}
	return decodeMapPayload(payload, b[4])
}

func decodeMapPayload(b []byte, version byte) (MapDefinition, error) {
	bad := func() (MapDefinition, error) { return MapDefinition{}, fmt.Errorf("decode map: invalid map contents") }
	if len(b) < 5 {
		return bad()
	}
	m := MapDefinition{Cols: int(b[0]), Rows: int(b[1])}
	n, nameLen := m.Cols*m.Rows, int(b[2])
	cellBytes, edgeBytes := 2, 6
	if version == 3 {
		cellBytes, edgeBytes = 4, 7
	}
	// Check dimensions before allocating using an untrusted cell count.
	if m.Cols < MinSize || m.Cols > MaxSize || m.Rows < MinSize || m.Rows > MaxSize || nameLen > 64 || len(b) < 5+nameLen+cellBytes*n {
		return bad()
	}
	m.Name = string(b[3 : 3+nameLen])
	b = b[3+nameLen:]
	m.Cells = make([]MapCell, n)
	for i := range m.Cells {
		p := b[cellBytes*i:]
		if version == 3 {
			if p[3] > 1 {
				return bad()
			}
			m.Cells[i] = MapCell{Terrain: Terrain(p[0] & 15), Feature: Feature(p[0] >> 4), Group: p[1], Circuit: CircuitKind(p[2]), InitiallyOpen: p[3] != 0}
			continue
		}
		cell, component := p[0], p[0]>>4
		c := MapCell{Terrain: Terrain(cell & 7), Group: p[1], InitiallyOpen: cell&8 != 0}
		if component > 12 || version == 1 && (component > 3 || c.InitiallyOpen) {
			return bad()
		}
		if component >= 10 {
			c.Feature = Spillway + Feature(component-10)
		} else if component >= 7 {
			c.Feature = Portal
			c.PortalMode = PortalMode(component - 7)
		} else if component > byte(Reservoir) {
			c.Circuit = CircuitKind(component - 2)
		} else {
			c.Feature = Feature(component)
		}
		m.Cells[i] = c
	}
	b = b[cellBytes*n:]
	edges := int(binary.BigEndian.Uint16(b[:2]))
	b = b[2:]
	if len(b) != edgeBytes*edges {
		return bad()
	}
	if edges > 0 {
		m.Edges = make([]MapEdge, edges)
	}
	for i := range m.Edges {
		m.Edges[i] = MapEdge{
			A: int(binary.BigEndian.Uint16(b[:2])), B: int(binary.BigEndian.Uint16(b[2:4])),
			Kind: EdgeKind(b[4] & 0x7f), Group: b[5], InitiallyOpen: b[4]&0x80 != 0,
		}
		if version == 3 {
			if b[6] > 1 {
				return bad()
			}
			m.Edges[i].Kind, m.Edges[i].InitiallyOpen = EdgeKind(b[4]), b[6] != 0
		}
		if version == 1 && m.Edges[i].InitiallyOpen {
			return bad()
		}
		b = b[edgeBytes:]
	}
	if err := m.ValidateStructure(); err != nil {
		return MapDefinition{}, fmt.Errorf("decode map: %w", err)
	}
	return m, nil
}

// MapCode is the portable file encoded as pasteable URL-safe text.
func MapCode(m MapDefinition) (string, error) {
	b, err := EncodeMap(m)
	if err != nil {
		return "", err
	}
	return mapCodePrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// ParseMapCode accepts a share code or a map import link. The fragment is never
// sent to the website, so a link needs no map storage or server lookup.
func ParseMapCode(text string) (MapDefinition, error) {
	if len(text) > 2*MaxMapBytes {
		return MapDefinition{}, fmt.Errorf("import map: share code is too large")
	}
	code := strings.TrimSpace(text)
	if _, fragment, ok := strings.Cut(code, "#"); ok {
		code = ""
		for part := range strings.SplitSeq(fragment, "&") {
			if value, found := strings.CutPrefix(part, "map="); found {
				code = value
				break
			}
		}
	}
	encoded, ok := strings.CutPrefix(code, mapCodePrefix)
	if !ok {
		encoded, ok = strings.CutPrefix(code, "orbitrix-map1.")
	}
	if !ok {
		encoded, ok = strings.CutPrefix(code, "orbitrix-map3.")
	}
	if !ok {
		return MapDefinition{}, fmt.Errorf("import map: unsupported share code")
	}
	if len(encoded) > base64.RawURLEncoding.EncodedLen(MaxMapBytes) {
		return MapDefinition{}, fmt.Errorf("import map: share code is too large")
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return MapDefinition{}, fmt.Errorf("import map: invalid share code")
	}
	return DecodeMap(b)
}
