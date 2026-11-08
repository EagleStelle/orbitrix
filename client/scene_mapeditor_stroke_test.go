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

func TestEditorEdgeStrokeFollowsDrag(t *testing.T) {
	// Register an edge brush outside the existing tool range to cover future tools.
	old := editorBrushes[brushWall]
	editorBrushes[brushWall] = boardview.Brush{Edge: int(core.Barrier)}
	t.Cleanup(func() { editorBrushes[brushWall] = old })
	for tool, brush := range editorBrushes {
		if brush.Edge < 0 {
			continue
		}
		for _, tt := range []struct {
			name   string
			points [][2]float64
			edges  [][2]int
		}{
			{"right", [][2]float64{{1.08, 3.08}, {6.92, 3.08}}, [][2]int{{10, 11}, {18, 19}, {26, 27}, {34, 35}, {42, 43}, {50, 51}}},
			{"left", [][2]float64{{6.92, 3.08}, {1.08, 3.08}}, [][2]int{{10, 11}, {18, 19}, {26, 27}, {34, 35}, {42, 43}, {50, 51}}},
			{"down", [][2]float64{{3.08, 1.08}, {3.08, 6.92}}, [][2]int{{17, 25}, {18, 26}, {19, 27}, {20, 28}, {21, 29}, {22, 30}}},
			{"up", [][2]float64{{3.08, 6.92}, {3.08, 1.08}}, [][2]int{{17, 25}, {18, 26}, {19, 27}, {20, 28}, {21, 29}, {22, 30}}},
			{"turn", [][2]float64{{1.08, 3.08}, {6.08, 3.08}, {6.08, 6.92}}, [][2]int{{10, 11}, {18, 19}, {26, 27}, {34, 35}, {42, 43}, {43, 51}, {44, 52}, {45, 53}, {46, 54}}},
			{"diagonal", [][2]float64{{1.08, 3.08}, {6.08, 6.08}}, [][2]int{{10, 11}, {11, 19}, {19, 20}, {20, 28}, {28, 29}, {36, 37}, {37, 45}, {45, 46}}},
			{"reverse", [][2]float64{{1.08, 3.08}, {6.92, 3.08}, {1.08, 3.08}}, [][2]int{{10, 11}, {18, 19}, {26, 27}, {34, 35}, {42, 43}, {50, 51}}},
			{"jitter", [][2]float64{{1.08, 3.08}, {2.02, 2.96}, {3.01, 3.04}, {4.03, 2.98}, {6.92, 3.08}}, [][2]int{{10, 11}, {18, 19}, {26, 27}, {34, 35}, {42, 43}, {50, 51}}},
		} {
			for _, finger := range []bool{false, true} {
				t.Run(editorTools[tool]+"/"+tt.name+"/touch="+strconv.FormatBool(finger), func(t *testing.T) {
					a := setupTestApp(1040, 640, 1, 2)
					e := newMapEditor(a, newHome(), core.NewMap(9, 8), nil)
					e.canvas = ui.Rect{W: 900, H: 800}
					e.selectTool(tool)
					e.group = 3
					before := e.m.Clone()
					k, ox, oy := e.grid()
					for n, p := range tt.points {
						a.in = input{x: ox + p[0]*k, y: oy + p[1]*k, down: true, pressed: n == 0, finger: finger}
						e.updateCanvas(a, a.in.pressed, a.in.down)
						preview, ok := e.hoverEdge(e.hover, a.in.x, a.in.y)
						if !ok || !slices.ContainsFunc(e.m.Edges, func(edge core.MapEdge) bool { return edge.A == preview.A && edge.B == preview.B }) {
							t.Fatal("held preview must highlight the actual stroke edge")
						}
					}
					e.updateCanvas(a, false, false)
					want := make([]core.MapEdge, len(tt.edges))
					for i, pair := range tt.edges {
						want[i] = core.MapEdge{A: pair[0], B: pair[1], Kind: core.EdgeKind(brush.Edge)}
						if brush.Edge == int(core.Gate) {
							want[i].Group = 3
						}
					}
					sortEdges := func(edges []core.MapEdge) {
						slices.SortFunc(edges, func(a, b core.MapEdge) int {
							if a.A != b.A {
								return a.A - b.A
							}
							return a.B - b.B
						})
					}
					got := slices.Clone(e.m.Edges)
					sortEdges(got)
					sortEdges(want)
					if !slices.Equal(got, want) {
						t.Fatalf("drag painted extra or missing edges:\n got %v\nwant %v", got, want)
					}
					if e.stroke || !e.dirty || len(e.undo) != 1 || !slices.Equal(e.m.Cells, before.Cells) {
						t.Fatal("edge drag must leave cells untouched and create one undoable edit")
					}
					after := e.m.Clone()
					e.history(false)
					if !reflect.DeepEqual(e.m, before) {
						t.Fatal("undo did not restore the original map")
					}
					e.history(true)
					if !reflect.DeepEqual(e.m, after) {
						t.Fatal("redo did not restore the stroke")
					}
				})
			}
		}
	}
}

func TestEditorEdgeStrokePreservesStartingEdge(t *testing.T) {
	for _, painted := range []bool{false, true} {
		t.Run("already painted="+strconv.FormatBool(painted), func(t *testing.T) {
			a := setupTestApp(1040, 640, 1, 2)
			m := core.NewMap(9, 8)
			m.Edges = []core.MapEdge{{A: 3, B: 11, Kind: core.Gate, Group: 2}}
			route := []core.MapEdge{{A: 10, B: 11}, {A: 18, B: 19}, {A: 26, B: 27}}
			if painted {
				m.Edges = append(m.Edges, route...)
			}
			e := newMapEditor(a, newHome(), m, nil)
			e.canvas = ui.Rect{W: 900, H: 800}
			e.selectTool(brushBarrier)
			k, ox, oy := e.grid()
			for n, p := range [][2]float64{{1.08, 3.08}, {3.92, 3.08}} {
				a.in.x, a.in.y = ox+p[0]*k, oy+p[1]*k
				e.updateCanvas(a, n == 0, true)
			}
			e.updateCanvas(a, false, false)
			want := append([]core.MapEdge{m.Edges[0]}, route...)
			if !slices.Equal(e.m.Edges, want) || e.m.Edges[0] != m.Edges[0] {
				t.Fatal("drag must preserve the original kind and circuit on the perpendicular starting edge")
			}
			if painted && (e.dirty || len(e.undo) != 0) {
				t.Fatal("an unchanged drag must not leave a dirty draft or an undo checkpoint")
			}
		})
	}
}

func TestEditorEdgeStrokeSkipsNonFloorAndBorders(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	e := newMapEditor(a, newHome(), core.NewMap(9, 8), nil)
	e.canvas = ui.Rect{W: 900, H: 800}
	e.selectTool(brushBarrier)
	e.m.Cells[26].Terrain = core.Wall
	e.m.Cells[34].Terrain = core.Void
	e.m.Cells[42].Terrain = core.BreakableWall
	k, ox, oy := e.grid()
	for n, p := range [][2]float64{{1.08, 3.08}, {8.98, 3.08}, {8.98, 7.98}, {.02, 7.98}, {.02, .02}} {
		a.in.x, a.in.y = ox+p[0]*k, oy+p[1]*k
		e.updateCanvas(a, n == 0, true)
	}
	e.updateCanvas(a, false, false)
	want := []core.MapEdge{{A: 10, B: 11}, {A: 18, B: 19}, {A: 50, B: 51}, {A: 58, B: 59}, {A: 66, B: 67}}
	if !slices.Equal(e.m.Edges, want) {
		t.Fatalf("blocked floor and outside borders must not receive edges: %v", e.m.Edges)
	}
	if err := e.m.ValidateStructure(); err != nil {
		t.Fatal(err)
	}
}
