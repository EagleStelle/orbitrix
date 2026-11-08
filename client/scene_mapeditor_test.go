package client

import (
	"reflect"
	"slices"
	"strconv"
	"testing"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

// Compare a live stroke with validation rebuilt from the current draft.
func checkEditorValidation(t *testing.T, a *app, e *mapEditor) {
	t.Helper()
	e.renderMap(a)
	want := newMapEditor(nil, nil, e.m, nil)
	if !reflect.DeepEqual(e.issues, want.issues) || e.bad != want.bad || !slices.Equal(e.badEdges, want.badEdges) || !slices.Equal(e.capacity, want.capacity) {
		t.Fatal("rendered map has stale issues, highlights or capacities")
	}
}

func TestEditorEraseStrokeKeepsValidationCurrent(t *testing.T) {
	edges := []core.MapEdge{{A: 30, B: 31, Kind: core.Barrier}, {A: 40, B: 41, Kind: core.Gate, Group: 1}, {A: 50, B: 59, Kind: core.Gate, Group: 2}}
	for _, mode := range []string{"mouse", "right mouse", "touch"} {
		for _, tt := range []struct {
			name  string
			edges []core.MapEdge
			erase int
		}{
			{"only invalid edge", edges[1:2], 0},
			{"only valid edge", edges[:1], 0},
			{"first edge", edges, 0},
			{"middle edge", edges, 1},
			{"last edge", edges, 2},
		} {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				a := setupTestApp(1040, 640, 1, 2)
				m := core.NewMap(9, 9)
				m.Edges = slices.Clone(tt.edges)
				e := newMapEditor(a, newHome(), m, nil)
				e.capacities, e.preview = true, true
				e.layout(a)
				t.Cleanup(func() { e.mapImage.Deallocate() })
				checkEditorValidation(t, a, e)
				e.selectTool(brushErase)
				if mode == "right mouse" {
					e.selectTool(brushWall)
				}
				pointAt := func(edge core.MapEdge) {
					k, ox, oy := e.grid()
					x0, y0, x1, y1, _ := boardview.MapArtEdge(&e.m, edge)
					a.in.x, a.in.y = ox+(x0+x1)*k/2, oy+(y0+y1)*k/2
				}
				step := func() {
					t.Helper()
					if err := e.update(a); err != nil {
						t.Fatal(err)
					}
					checkEditorValidation(t, a, e)
				}
				a.in = input{pressed: true, down: true, finger: mode == "touch", right: mode == "right mouse"}
				pointAt(e.m.Edges[tt.erase])
				step()
				if !e.stroke || len(e.m.Edges) != len(m.Edges)-1 {
					t.Fatal("edge must be erased and drawn before releasing the stroke")
				}
				a.in.pressed = false
				for len(e.m.Edges) > 0 {
					before := len(e.m.Edges)
					pointAt(e.m.Edges[0])
					step()
					if len(e.m.Edges) >= before {
						t.Fatal("drag did not erase the next edge")
					}
				}
				a.in = input{x: offScreen, y: offScreen}
				step()
				if e.stroke || !e.dirty || len(e.undo) != 1 || !slices.Equal(m.Edges, tt.edges) {
					t.Fatal("drag must remain one undoable edit and leave the source map untouched")
				}
				e.history(false)
				checkEditorValidation(t, a, e)
				if !slices.Equal(e.m.Edges, m.Edges) {
					t.Fatal("undo did not restore all edges")
				}
				e.history(true)
				checkEditorValidation(t, a, e)
				if len(e.m.Edges) != 0 {
					t.Fatal("redo did not remove all edges")
				}
			})
		}
	}
}

func TestEditorPaintStrokeKeepsValidationCurrent(t *testing.T) {
	for tool, name := range editorTools {
		if tool == brushPan {
			continue
		}
		t.Run(name, func(t *testing.T) {
			a := setupTestApp(1040, 640, 1, 2)
			m := core.NewMap(9, 9)
			if tool == brushFloor {
				m.Cells[40].Terrain = core.Wall
			} else if tool == brushErase {
				m.Cells[40].Feature = core.Reservoir
			}
			if editorBrushes[tool].Circuit.IsSwitch() {
				m.Edges = []core.MapEdge{{A: 30, B: 31, Kind: core.Gate, Group: 1}}
			}
			e := newMapEditor(a, newHome(), m, nil)
			e.capacities, e.preview = true, true
			e.layout(a)
			checkEditorValidation(t, a, e)
			defer func() { e.mapImage.Deallocate() }()
			e.selectTool(tool)
			k, ox, oy := e.grid()
			a.in = input{x: ox + 4.5*k, y: oy + 4.5*k, pressed: true, down: true}
			if editorBrushes[tool].Edge >= 0 {
				a.in.y = oy + 5*k
			}
			step := func() {
				t.Helper()
				if err := e.update(a); err != nil {
					t.Fatal(err)
				}
				checkEditorValidation(t, a, e)
			}
			step()
			if !e.strokeChanged || !e.stroke {
				t.Fatal("brush must change the map before mouse release")
			}
			revision := e.revision
			e.notify("Keep this notification")
			a.in.pressed = false
			step()
			if e.revision != revision || e.note != "Keep this notification" {
				t.Fatal("unchanged held input must not invalidate the map or clear notifications")
			}
			if editorBrushes[tool].Edge >= 0 {
				if tool == brushBarrier {
					e.selectTool(brushGate)
				} else {
					e.selectTool(brushBarrier)
				}
				a.in.x += .01 * k
			} else {
				a.in.x, a.in.y = ox+6.5*k, oy+6.5*k
			}
			step()
			if tool == brushLink && (e.pair != -1 || len(e.issues) != 0) {
				t.Fatal("completing a Link pair must clear its issue during the stroke")
			}
			if editorBrushes[tool].Edge >= 0 && e.m.Edges[0].Kind == core.EdgeKind(editorBrushes[tool].Edge) {
				t.Fatal("replacing an edge must update its validation even when the edge count is unchanged")
			}
			a.in = input{x: offScreen, y: offScreen}
			step()
			if e.stroke || !e.dirty || len(e.undo) != 1 {
				t.Fatal("painting must preserve a single undo checkpoint per stroke")
			}
		})
	}
}

func TestEditorResizeValidationAndHistoryAtSizeLimits(t *testing.T) {
	for _, size := range []int{core.MinSize, 9, core.MaxSize} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			a := setupTestApp(1040, 640, 1, 2)
			m := core.NewMap(size, size)
			m.Edges = []core.MapEdge{{A: size*size - 2, B: size*size - 1, Kind: core.Gate, Group: 1}}
			e := newMapEditor(a, newHome(), m, nil)
			e.capacities, e.preview, e.more = true, true, true
			e.layout(a)
			checkEditorValidation(t, a, e)
			defer func() { e.mapImage.Deallocate() }()
			for _, action := range []int{1, 3, 0, 2, 0, 2, 1, 3} {
				a.in = input{x: e.dims[action].Cx(), y: e.dims[action].Cy(), clicked: true}
				if err := e.update(a); err != nil {
					t.Fatal(err)
				}
				if err := e.m.ValidateStructure(); err != nil {
					t.Fatal(err)
				}
				checkEditorValidation(t, a, e)
			}
			final := e.m.Clone()
			for len(e.undo) > 0 {
				e.history(false)
				checkEditorValidation(t, a, e)
			}
			if !reflect.DeepEqual(e.m, m) {
				t.Fatal("undo after resizing did not restore the original map")
			}
			for len(e.redo) > 0 {
				e.history(true)
				checkEditorValidation(t, a, e)
			}
			if !reflect.DeepEqual(e.m, final) {
				t.Fatal("redo after resizing did not restore the final map")
			}
		})
	}
}

func TestCustomMapViewReplaysTopology(t *testing.T) {
	for _, m := range core.StarterMaps() {
		t.Run(m.Name, func(t *testing.T) {
			g, err := core.NewGameWithMap(m, 3)
			if err != nil {
				t.Fatal(err)
			}
			v := boardview.NewView(g)
			rng := core.NewRNG(789)
			var moves []int
			for turn := 0; turn < 300 && !g.Over(); turn++ {
				moves = g.LegalMoves(moves[:0])
				i, p := moves[rng.Intn(len(moves))], int8(g.Turn)
				waves, err := g.Apply(i)
				if err != nil {
					t.Fatal(err)
				}
				v.Events = g.TerrainEvents
				v.Place(i, p)
				for _, w := range waves {
					v.Launch(w)
					v.Land(w)
				}
				if !slices.Equal(v.Count, g.Count) || !slices.Equal(v.Owner, g.Owner) || !slices.Equal(v.Geo.Cap, g.Geo.Cap) || !slices.Equal(v.Replay.Durability, g.Durability) || !slices.Equal(v.Replay.Pressed, g.Pressed) || v.Replay.CircuitOn != g.CircuitOn {
					t.Fatalf("replay differs at turn %d", turn)
				}
			}
		})
	}
}

func TestResizeMapKeepsCoordinates(t *testing.T) {
	m := core.NewMap(5, 7)
	m.Cells[2*7+3].Feature = core.Reservoir
	m.Edges = []core.MapEdge{{A: 2*7 + 3, B: 2*7 + 4, Kind: core.Barrier}, {A: 4*7 + 5, B: 4*7 + 6, Kind: core.Barrier}}
	n := resizeMap(m, 4, 6)
	if n.Cells[2*6+3].Feature != core.Reservoir || len(n.Edges) != 1 || n.Edges[0].A != 2*6+3 || n.Edges[0].B != 2*6+4 {
		t.Fatalf("resize lost coordinates: %+v", n.Edges)
	}
	if m.Cells[2*7+3].Feature != core.Reservoir {
		t.Fatal("resize mutated original")
	}
}

func TestEditorStrokeUndoAndLinkPair(t *testing.T) {
	a := setupTestApp(390, 780, 1, 2)
	m := core.NewMap(9, 9)
	e := newMapEditor(a, newHome(), m, func(core.MapDefinition) error { return nil })
	e.layout(a)
	e.checkpoint()
	e.tool = 1
	for _, i := range []int{20, 21, 22} {
		e.paint(i, 0, 0)
	}
	e.changed()
	if len(e.undo) != 1 {
		t.Fatal("stroke should have one checkpoint")
	}
	e.history(false)
	if e.m.Cells[20].Terrain != core.Floor || e.m.Cells[22].Terrain != core.Floor {
		t.Fatal("undo did not restore stroke")
	}
	e.history(true)
	if e.m.Cells[21].Terrain != core.Wall {
		t.Fatal("redo failed")
	}
	e.tool = 4
	e.last = -1
	e.paint(10, 0, 0)
	e.last = 10
	e.paint(70, 0, 0)
	if e.pair != -1 || e.m.Cells[10].Group == 0 || e.m.Cells[10].Group != e.m.Cells[70].Group {
		t.Fatal("Link endpoints were not paired")
	}
	if m.Cells[20].Terrain != core.Floor {
		t.Fatal("editor changed source map")
	}
}

func TestEditorTestSetupIsolated(t *testing.T) {
	a := setupTestApp(390, 780, 1, 2)
	original := a.set
	m := core.StarterMaps()[1]
	e := newMapEditor(a, newHome(), m, func(core.MapDefinition) error { return nil })
	s := newMapTestSetup(a, e, m)
	s.settings.Map.Cells[0].Terrain = core.Wall
	s.settings.seats[0] = 1
	if a.set != original || e.m.Cells[0].Terrain != m.Cells[0].Terrain || a.set.seats[0] == 1 {
		t.Fatal("test setup changed original preferences or draft")
	}
}

func TestEditorToolsRemainReachable(t *testing.T) {
	for _, size := range [][2]float64{{320, 640}, {390, 844}, {568, 320}, {600, 320}, {640, 360}, {760, 640}, {1040, 640}} {
		a := setupTestApp(size[0], size[1], 1, 2)
		e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
		e.layout(a)
		for i, r := range e.palette {
			if !containsRect(e.p.view.Viewport, r) && e.dockScroll.MaxScroll() == 0 {
				t.Fatalf("%vx%v: tool %s is unreachable: %+v", size[0], size[1], editorTools[e.paletteTools[i]], r)
			}
		}
		if !containsRect(e.p.view.Viewport, e.canvas) || !containsRect(e.p.view.Viewport, e.dock) {
			t.Fatalf("workspace leaves the screen on %v", size)
		}
		if e.canvas.X < e.dock.Right() && e.canvas.Right() > e.dock.X && e.canvas.Y < e.dock.Bottom() && e.canvas.Bottom() > e.dock.Y {
			t.Fatalf("palette covers canvas on %v", size)
		}
		for _, r := range e.control {
			minTarget := 44 * a.u
			if editorCompact(a) {
				minTarget = 40 * a.u
			}
			if r.W < minTarget || r.H < minTarget || !containsRect(e.p.view.Viewport, r) && !e.portrait {
				t.Fatalf("tool target too small or clipped on %v: %+v", size, r)
			}
		}
		if size[0] == 1040 && e.canvas.H < 500 {
			t.Fatal("desktop map must lead the layout")
		}
	}
}

func TestEditorMixedSwitchesPerCircuit(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
	e.tool = brushLatch
	if !e.paint(10, 0, 0) {
		t.Fatal("could not place first switch")
	}
	e.tool = brushToggle
	if !e.paint(20, 0, 0) || e.m.Cells[20].Circuit != core.Toggle {
		t.Fatal("could not add another controller")
	}
	if !e.paint(10, 0, 0) || e.m.Cells[10].Circuit != core.Toggle {
		t.Fatal("could not replace controller type in its own cell")
	}
}

func TestEditorLimitsLinkPairsToTwelve(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	m := core.NewMap(9, 9)
	for i := 1; i <= core.MaxMapGroups; i++ {
		m.Cells[i] = core.MapCell{Feature: core.Link, Group: uint8(i)}
		m.Cells[i+40] = core.MapCell{Feature: core.Link, Group: uint8(i)}
	}
	e := newMapEditor(a, newHome(), m, func(core.MapDefinition) error { return nil })
	e.selectTool(brushLink)
	if e.paint(70, 0, 0) || e.m.Cells[70].Feature != core.NoFeature {
		t.Fatal("created a thirteenth pair")
	}
	if e.note != "Choose an unused pair color." {
		t.Fatalf("missing recovery message: %s", e.note)
	}
}

func TestEditorRevealsSelectedTool(t *testing.T) {
	for _, size := range [][2]float64{{568, 320}, {1040, 540}} {
		a := setupTestApp(size[0], size[1], 1, 2)
		e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
		for _, tool := range []int{brushHold, brushFloor, brushGate, brushLatch} {
			e.selectTool(tool)
			e.layout(a)
			for i, item := range e.paletteTools {
				if item == tool && !containsRect(e.dockScroll.Viewport, e.palette[i]) {
					t.Fatalf("%v: selected tool %s is clipped", size, editorTools[tool])
				}
			}
		}
	}
}

func TestEditorValidationSnackbar(t *testing.T) {
	a := setupTestApp(390, 780, 1, 2)
	e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
	e.m.Cells[10].Feature = core.Link
	e.m.Cells[10].Group = 1
	e.changed()
	bar := sceneSnackbar(e)
	if len(e.issues) == 0 || bar == nil || bar.message != e.issues[0].Message {
		t.Fatal("validation must automatically expose an issue through the shared snackbar")
	}
	bar.ticks = snackIn
	bounds, _, _ := bar.frame(a)
	if bounds.Bottom() != a.safe().Bottom()-snackGap*a.u {
		t.Fatal("editor snackbar must be bottom anchored")
	}
	e.selectTool(brushErase)
	if e.snackbar() == nil {
		t.Fatal("changing tools must not hide unresolved validation")
	}
	e.m.Cells[10].Feature = core.NoFeature
	e.m.Cells[10].Group = 0
	e.changed()
	if e.snackbar() != nil {
		t.Fatal("resolving the issue must clear the snackbar")
	}
}

func TestEditorIssueHighlightsTargetElements(t *testing.T) {
	for _, tt := range []struct {
		name  string
		edit  func(*core.MapDefinition)
		cells []int
		edges []int
	}{
		{"orphan gate", func(m *core.MapDefinition) { m.Cells[40] = core.MapCell{} }, nil, []int{0}},
		{"gate without floor", func(m *core.MapDefinition) { m.Cells[30].Terrain = core.Wall }, nil, []int{0}},
		{"duplicate gates", func(m *core.MapDefinition) {
			m.Edges = append(m.Edges, core.MapEdge{A: 31, B: 30, Kind: core.Gate, Group: 1})
		}, nil, []int{0, 1}},
		{"invalid circuit color", func(m *core.MapDefinition) {
			m.Cells[40].Group, m.Edges[0].Group = 13, 13
		}, []int{40}, []int{0}},
		{"multiple switches", func(m *core.MapDefinition) {
			m.Cells[50] = core.MapCell{Circuit: core.Toggle, Group: 1}
		}, nil, nil},
		{"barrier with group", func(m *core.MapDefinition) {
			m.Cells[40], m.Edges[0].Kind = core.MapCell{}, core.Barrier
		}, nil, []int{0}},
		{"floor issue beside valid gate", func(m *core.MapDefinition) {
			m.Edges[0].A, m.Edges[0].B = 0, 1
		}, []int{0}, nil},
		{"valid gate", func(*core.MapDefinition) {}, nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := core.NewMap(9, 9)
			m.Cells[40] = core.MapCell{Circuit: core.Latch, Group: 1}
			m.Edges = []core.MapEdge{{A: 30, B: 31, Kind: core.Gate, Group: 1}}
			valid := m.Clone()
			tt.edit(&m)
			e := newMapEditor(nil, nil, m, nil)
			var wantCells [core.MaxSize * core.MaxSize]bool
			for _, i := range tt.cells {
				wantCells[i] = true
			}
			if e.bad != wantCells {
				t.Fatalf("highlighted cells must be %v; edge issues must not mark adjacent floors", tt.cells)
			}
			wantEdges := make([]bool, len(m.Edges))
			for _, i := range tt.edges {
				wantEdges[i] = true
			}
			if !slices.Equal(e.badEdges, wantEdges) {
				t.Fatalf("highlighted edges = %v, want %v", e.badEdges, wantEdges)
			}
			e.m = valid
			e.changed()
			if e.bad != [core.MaxSize * core.MaxSize]bool{} || !slices.Equal(e.badEdges, []bool{false}) {
				t.Fatal("fixing issues must clear cell and edge highlights")
			}
		})
	}
}

func TestEditorAllowsBreakableEnclosureAndHighlightsUnreachableGate(t *testing.T) {
	m := core.NewMap(9, 9)
	for c := 2; c <= 6; c++ {
		for r := 2; r <= 6; r++ {
			if c == 2 || c == 6 || r == 2 || r == 6 {
				m.Cells[c*m.Rows+r].Terrain = core.BreakableWall
			}
		}
	}
	controller := 4*m.Rows + 4
	m.Cells[controller] = core.MapCell{Circuit: core.Latch, Group: 1}
	m.Edges = []core.MapEdge{{A: m.Rows + 4, B: m.Rows + 5, Kind: core.Gate, Group: 1}}
	e := newMapEditor(nil, nil, m, nil)
	s := settings{Map: &e.m, Players: 2}
	if len(e.issues) != 0 || !mapCanStart(&s) || e.capacity[3*m.Rows+3] != 2 {
		t.Fatal("breakable enclosure should allow Test and retain starting capacities")
	}
	for i, c := range e.m.Cells {
		if c.Terrain == core.BreakableWall {
			e.m.Cells[i].Terrain = core.Wall
		}
	}
	e.changed()
	s.mapChecked = nil
	if len(e.issues) == 0 || mapCanStart(&s) || !e.bad[controller] || !e.badEdges[0] {
		t.Fatal("permanent enclosure must block play and highlight the unreachable switch and Gate")
	}
}

func TestEditorReservoirRescuesDeadEnd(t *testing.T) {
	m := core.NewMap(5, 5)
	for _, neighbor := range []int{17, 11, 13} {
		m.Edges = append(m.Edges, core.MapEdge{A: 12, B: neighbor, Kind: core.Barrier})
	}
	e := newMapEditor(nil, nil, m, nil)
	if e.capacity[12] != 1 || !e.bad[12] || mapCanStart(&settings{Map: &e.m, Players: 2}) {
		t.Fatal("one-connection Floor must start invalid")
	}
	e.checkpoint()
	e.selectTool(brushReservoir)
	if !e.paint(12, 0, 0) {
		t.Fatal("could not place Reservoir on invalid Floor")
	}
	e.changed()
	if e.capacity[12] != 2 || e.bad[12] || len(e.issues) != 0 || !mapCanStart(&settings{Map: &e.m, Players: 2}) {
		t.Fatal("Reservoir must clear the Floor error and allow Test")
	}
	e.history(false)
	if e.capacity[12] != 1 || !e.bad[12] || mapCanStart(&settings{Map: &e.m, Players: 2}) {
		t.Fatal("undoing Reservoir placement must restore the Floor error")
	}
	e.history(true)
	if e.capacity[12] != 2 || e.bad[12] || !mapCanStart(&settings{Map: &e.m, Players: 2}) {
		t.Fatal("redoing Reservoir placement must allow Test again")
	}
	e.m.Edges = append(e.m.Edges, core.MapEdge{A: 12, B: 7, Kind: core.Barrier})
	e.changed()
	if e.capacity[12] != 1 || !e.bad[12] || mapCanStart(&settings{Map: &e.m, Players: 2}) {
		t.Fatal("Reservoir cannot rescue Floor with no open connections")
	}
}

func TestEditorToolbarAcrossPlatforms(t *testing.T) {
	for _, size := range [][2]float64{{320, 640}, {390, 844}, {568, 320}, {640, 360}, {760, 640}, {1040, 640}} {
		for _, scale := range []float64{1, 2.625} {
			a := withCutout(setupTestApp(size[0], size[1], scale, 2))
			e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
			e.layout(a)
			controls := []ui.Rect{e.p.back, e.field, e.capacityB, e.previewB, e.testB, e.saveB, e.toolB}
			for i, r := range controls {
				minTarget := 44 * a.u
				if editorCompact(a) {
					minTarget = 40 * a.u
				}
				if r.W < minTarget || r.H < minTarget || !containsRect(a.safe(), r) && !(e.portrait && r.Y == e.toolbarScroll.Viewport.Y) {
					t.Fatalf("%v at %v: clipped or small header control %d: %+v", size, scale, i, r)
				}
				for j := 0; j < i; j++ {
					if overlaps(r, controls[j]) {
						t.Fatalf("%v at %v: overlapping header controls", size, scale)
					}
				}
			}
			for _, r := range []ui.Rect{e.field, e.capacityB, e.previewB, e.testB, e.saveB, e.toolB, e.control[2]} {
				if e.portrait && r.Y == e.toolbarScroll.Viewport.Y {
					originalX := r.X
					r.X -= e.toolbarScroll.Offset
					e.toolbarScroll.Reveal(r)
					e.layout(a)
					r.X = originalX - e.toolbarScroll.Offset
				}
				a.in.x, a.in.y = r.Cx(), r.Cy()
				a.pointer = false
				if err := e.update(a); err != nil {
					t.Fatal(err)
				}
				if !a.pointer {
					t.Fatalf("%v: interactive control has no pointer: %+v", size, r)
				}
			}
			if e.portrait {
				e.toolbarScroll.Reveal(e.capacityB)
				e.layout(a)
			}
			a.in.x, a.in.y = e.capacityB.Cx(), e.capacityB.Cy()
			a.in.clicked = true
			e.update(a)
			if !e.capacities || e.preview {
				t.Fatal("capacity toggle must work independently")
			}
			a.in.x, a.in.y = e.previewB.Cx(), e.previewB.Cy()
			e.update(a)
			if !e.capacities || !e.preview {
				t.Fatal("preview toggle must work independently")
			}
		}
	}
}

func TestEditorEraseHighlightMatchesRemoval(t *testing.T) {
	for _, kind := range []core.EdgeKind{core.Barrier, core.Gate, core.BreakableBarrier} {
		for _, vertical := range []bool{false, true} {
			a := setupTestApp(1040, 640, 1, 2)
			m := core.NewMap(9, 9)
			b := 31
			if vertical {
				b = 39
			}
			m.Cells[30].Feature = core.Reservoir
			m.Edges = []core.MapEdge{{A: 30, B: b, Kind: kind, Group: 1}}
			e := newMapEditor(a, newHome(), m, func(core.MapDefinition) error { return nil })
			e.layout(a)
			e.selectTool(brushErase)
			k, ox, oy := e.grid()
			x0, y0, x1, y1, _ := boardview.MapArtEdge(&e.m, e.m.Edges[0])
			x, y := ox+(x0+x1)*k/2, oy+(y0+y1)*k/2
			i := e.cellAt(x, y)
			if e.eraseEdgeAt(i, x, y) != 0 {
				t.Fatal("erase must highlight the hovered edge")
			}
			if !e.paint(i, x, y) || len(e.m.Edges) != 0 || e.m.Cells[30].Feature != core.Reservoir {
				t.Fatal("edge erase must remove only the highlighted edge")
			}
			x, y = ox+3.5*k, oy+3.5*k
			if e.eraseEdgeAt(30, x, y) != -1 {
				t.Fatal("cell center must highlight the cell")
			}
			if !e.paint(30, x, y) || e.m.Cells[30].Feature != core.NoFeature {
				t.Fatal("cell erase must remove the hovered feature")
			}
		}
	}
}

func TestEditorHeldMouseTools(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	m := core.NewMap(9, 9)
	m.Cells[40].Terrain = core.Wall
	e := newMapEditor(a, newHome(), m, func(core.MapDefinition) error { return nil })
	e.selectTool(brushLink)
	e.pair = 10
	e.m.Cells[10].Feature, e.m.Cells[10].Group = core.Link, 1
	e.layout(a)
	k, ox, oy := e.grid()
	a.in.x, a.in.y = ox+4.5*k, oy+4.5*k
	a.in.middle = true
	e.update(a)
	if e.tool != brushPan || !e.panning || e.pair != 10 {
		t.Fatal("middle hold must pan without losing the Link pair")
	}
	a.in.x += 24
	a.in.y += 18
	e.update(a)
	if e.panX != 24 || e.panY != 18 {
		t.Fatal("middle drag did not pan")
	}
	a.in.middle = false
	e.update(a)
	if e.tool != brushLink || e.panning || e.pair != 10 || len(e.undo) != 0 {
		t.Fatal("middle release must restore the brush without creating history")
	}
	a.in.right = true
	e.update(a)
	if e.tool != brushErase || e.m.Cells[40].Terrain != core.Floor || !e.stroke {
		t.Fatal("right hold must erase immediately")
	}
	// Release over chrome: the stroke must still finish and the previous brush must return.
	a.in.x, a.in.y = e.saveB.Cx(), e.saveB.Cy()
	a.in.right = false
	e.update(a)
	if e.tool != brushLink || e.stroke || !e.dirty || len(e.undo) != 1 {
		t.Fatal("right release must finalize one undoable stroke and restore the brush")
	}
	e.history(false)
	if e.m.Cells[40].Terrain != core.Wall {
		t.Fatal("right-drag erase was not undoable")
	}
	// Overlapping holds always resolve to pan, then erase, then the original brush.
	a.in.x, a.in.y = e.canvas.Cx(), e.canvas.Cy()
	a.in.right = true
	e.update(a)
	a.in.middle = true
	e.update(a)
	if e.tool != brushPan {
		t.Fatal("middle hold must take priority")
	}
	a.in.middle = false
	e.update(a)
	if e.tool != brushErase {
		t.Fatal("releasing middle must resume held right erase")
	}
	a.in.right = false
	e.update(a)
	if e.tool != brushLink {
		t.Fatal("releasing both must restore the original brush")
	}
}

func TestEditorMouseOverridesIgnoreTouchAndDialogs(t *testing.T) {
	for _, state := range []string{"touch", "options", "channels", "tools", "outside"} {
		a := setupTestApp(1040, 640, 1, 2)
		e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
		e.layout(a)
		a.in.x, a.in.y = e.canvas.Cx(), e.canvas.Cy()
		a.in.middle, a.in.right = true, true
		switch state {
		case "touch":
			a.in.finger = true
		case "options":
			e.more = true
		case "channels":
			e.channelsOpen = true
		case "tools":
			e.tools = true
		case "outside":
			a.in.x, a.in.y = e.field.Cx(), e.field.Cy()
		}
		e.updateMouseTool(a)
		if e.mouseTool != 0 || e.tool != brushFloor {
			t.Fatalf("mouse override leaked into %s", state)
		}
	}
}

func TestEditorCompactSizingKeepsCanvasPrimary(t *testing.T) {
	for _, size := range [][2]float64{{320, 640}, {390, 844}, {568, 320}, {640, 360}} {
		for _, scale := range []float64{1, 2.625} {
			a := setupTestApp(size[0], size[1], scale, 2)
			e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
			e.layout(a)
			if e.control[0].H != 40*a.u {
				t.Fatal("mobile controls must use the compact 40px target")
			}
			if size[0] < size[1] {
				for _, r := range e.control {
					if r.Y != e.control[0].Y {
						t.Fatal("portrait tools must fit one row at the smallest supported phone width")
					}
				}
				if e.dock.H > 136*a.u || e.canvas.H < 300*a.u {
					t.Fatal("portrait chrome leaves too little map space")
				}
			} else {
				if e.field.Y != e.testB.Y {
					t.Fatal("landscape header must fit one row")
				}
				if e.canvas.H < 200*a.u || e.dock.W > 152*a.u {
					t.Fatal("landscape chrome crowds the map")
				}
			}
			for i, r := range e.control {
				for _, other := range e.control[:i] {
					if overlaps(r, other) {
						t.Fatal("compact editing tools overlap")
					}
				}
			}
		}
	}
}

func TestEditorPortraitHorizontalOverflow(t *testing.T) {
	a := setupTestApp(320, 640, 1, 2)
	e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
	e.selectTool(brushLatch)
	e.layout(a)
	if !e.toolbarScroll.Horizontal || e.toolbarScroll.MaxScroll() <= 0 || !e.dockScroll.Horizontal || e.dockScroll.MaxScroll() <= 0 {
		t.Fatal("portrait toolbar and elements must overflow horizontally")
	}
	for _, r := range e.palette {
		if r.W != 72*a.u {
			t.Fatal("portrait elements must keep their natural width")
		}
	}
	// A swipe may cross out of its starting strip, without triggering map paint or a control.
	a.in.finger, a.in.pressed, a.in.down = true, true, true
	a.in.x, a.in.y = e.toolbarScroll.Viewport.Cx(), e.toolbarScroll.Viewport.Cy()
	e.update(a)
	a.in.pressed = false
	a.in.dragX = -100
	a.in.x -= 100
	e.update(a)
	if e.toolbarScroll.Offset != 100 || e.stroke || e.dirty {
		t.Fatal("toolbar swipe must scroll without triggering editing")
	}
	a.in.down, a.in.dragX = false, 0
	e.update(a)
	// The last toolbar item remains reachable after scrolling to the end.
	e.toolbarScroll.Reveal(e.control[6])
	e.layout(a)
	if !containsRect(e.toolbarScroll.Viewport, e.control[6]) {
		t.Fatal("last toolbar action is unreachable")
	}
	// Auto-reveal and wheel work on the independent element strip.
	e.selectTool(brushDoor)
	e.layout(a)
	if !containsRect(e.dockScroll.Viewport, e.palette[len(e.palette)-1]) {
		t.Fatal("selected element is not revealed")
	}
	a.in.x, a.in.y = e.dockScroll.Viewport.Cx(), e.dockScroll.Viewport.Cy()
	a.in.wheel = 1
	e.update(a)
	if e.dockScroll.Offset >= e.dockScroll.MaxScroll() {
		t.Fatal("wheel did not scroll the element strip")
	}
	// A clipped action cannot be activated outside its strip.
	a.in.finger = false
	a.in.wheel = 0
	a.in.clicked = true
	a.in.x, a.in.y = e.capacityB.Cx(), e.capacityB.Cy()
	e.update(a)
	if e.capacities {
		t.Fatal("offscreen toolbar action took a click")
	}
}

func TestEditorPlacementAndEraseShareEdgeHighlight(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	for _, b := range []int{31, 39} {
		m := core.NewMap(9, 9)
		m.Edges = []core.MapEdge{{A: 30, B: b, Kind: core.Barrier}}
		e := newMapEditor(a, newHome(), m, func(core.MapDefinition) error { return nil })
		e.layout(a)
		k, ox, oy := e.grid()
		x0, y0, x1, y1, _ := boardview.MapArtEdge(&m, m.Edges[0])
		x, y := ox+(x0+x1)*k/2, oy+(y0+y1)*k/2
		i := e.cellAt(x, y)
		e.tool = brushErase
		want, ok := e.hoverEdge(i, x, y)
		if !ok {
			t.Fatal("erase highlight is missing")
		}
		for _, tool := range []int{brushBarrier, brushGate} {
			e.tool = tool
			got, ok := e.hoverEdge(i, x, y)
			if !ok || got != want {
				t.Fatal("edge placement must match erase outline exactly")
			}
		}
		e.tool = brushErase
		if _, ok := e.hoverEdge(30, ox+3.5*k, oy+3.5*k); ok {
			t.Fatal("plain cell must not receive an edge outline")
		}
	}
}

func TestEditorMoveToggle(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
	e.selectTool(brushWall)
	e.togglePan()
	e.update(a)
	if e.tool != brushPan {
		t.Fatal("Move must remain selected after the toggle")
	}
	e.togglePan()
	if e.tool != brushWall {
		t.Fatal("toggling Move off must restore the prior tool")
	}
	e.togglePan()
	e.selectTool(brushErase)
	e.togglePan()
	e.togglePan()
	if e.tool != brushErase {
		t.Fatal("Move must restore the most recently selected tool")
	}
}

func TestEditorViewActions(t *testing.T) {
	e := &mapEditor{zoom: 1, panX: 35, panY: -20}
	e.viewAction(6)
	if e.zoom != 1.25 {
		t.Fatal("zoom in must match the toolbar")
	}
	e.viewAction(4)
	if e.zoom != 1 {
		t.Fatal("zoom out must match the toolbar")
	}
	for range 100 {
		e.viewAction(6)
	}
	if e.zoom != 16 {
		t.Fatal("zoom in must respect its upper bound")
	}
	for range 100 {
		e.viewAction(4)
	}
	if e.zoom != .5 {
		t.Fatal("zoom out must respect its lower bound")
	}
	e.viewAction(5)
	if e.zoom != 1 || e.panX != 0 || e.panY != 0 {
		t.Fatal("reset must fit and recenter the map")
	}
}

func TestEditorNotificationTimeoutAndPersistentIssues(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
	e.notify("Map saved")
	for range snackDuration - 1 {
		e.update(a)
	}
	if e.snackbar() == nil {
		t.Fatal("notification disappeared before its timeout")
	}
	e.update(a)
	if e.snackbar() != nil {
		t.Fatal("notification did not disappear after five seconds")
	}
	e.notify("Map saved")
	if e.snackbar() == nil || e.snack.age != 0 {
		t.Fatal("repeated notifications must restart the timeout")
	}
	e.m.Cells[10].Feature, e.m.Cells[10].Group = core.Link, 1
	e.changed()
	e.autosave = 0
	for range snackDuration * 2 {
		e.update(a)
	}
	if e.snackbar() == nil || e.snack.timeout != 0 {
		t.Fatal("validation issues must remain visible")
	}
	e.notify("Another notification")
	for range snackDuration {
		e.update(a)
	}
	if e.snackbar() == nil || e.snack.message != e.issues[0].Message || e.snack.timeout != 0 {
		t.Fatal("unresolved issues must return after a notification expires")
	}
}

func TestEditorMoveCursor(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
	e.layout(a)
	e.togglePan()
	a.in.x, a.in.y = e.canvas.X+1, e.canvas.Y+1
	e.update(a)
	if !a.moveCursor {
		t.Fatal("Move must use the move cursor across the canvas, including empty space")
	}
	a.moveCursor = false
	a.in.x, a.in.y = e.saveB.Cx(), e.saveB.Cy()
	e.update(a)
	if a.moveCursor || !a.pointer {
		t.Fatal("toolbar controls must keep their pointer cursor")
	}
	a.pointer, a.moveCursor = false, false
	a.in.x, a.in.y = e.canvas.Cx(), e.canvas.Cy()
	e.selectTool(brushErase)
	e.update(a)
	if a.moveCursor {
		t.Fatal("leaving Move must restore the normal cursor")
	}
	e.togglePan()
	a.in.finger = true
	e.update(a)
	if a.moveCursor {
		t.Fatal("touch must not request a mouse cursor")
	}
}
