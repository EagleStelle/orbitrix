package client

import (
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	brushFloor = iota
	brushWall
	brushVoid
	brushBreak
	brushLink
	brushReservoir
	brushLatch
	brushBarrier
	brushGate
	brushErase
	brushPan
	brushToggle
	brushHold
)

var editorTools = [...]string{"Floor", "Wall", "Void", "Breakable wall", "Link", "Reservoir", "Latch", "Barrier", "Gate", "Erase", "Pan", "Toggle", "Hold"}
var editorHelp = [...]string{
	"Playable surface. Open routes set its capacity.", "Blocks explosions permanently.", "Empty space outside the playable board.", "Two nearby blasts turn it into floor.",
	"Pair two distant cells to route explosions.", "Stores one extra orb before exploding.", "Stays pressed after the first orb arrives.", "Blocks a route between two floor cells.",
	"Opens while its circuit's switch is pressed.", "Remove an edge or restore a cell to floor.", "Drag the map to move around.", "Flips on placement into an empty cell or when orbs pop in or out.", "Stays pressed while this cell holds orbs.",
}
var editorGroups = [...]struct {
	name  string
	tools []int
}{
	{"Terrain", []int{brushFloor, brushWall, brushVoid, brushBarrier}},
	{"Structures", []int{brushBreak, brushGate}},
	{"Devices", []int{brushLink, brushReservoir, brushLatch, brushToggle, brushHold}},
}
var editorBrushes = [...]boardview.Brush{
	{Terrain: core.Floor, Edge: -1}, {Terrain: core.Wall, Edge: -1}, {Terrain: core.Void, Edge: -1}, {Terrain: core.BreakableWall, Edge: -1},
	{Feature: core.Link, Edge: -1}, {Feature: core.Reservoir, Edge: -1}, {Feature: core.Switch, Edge: -1}, {Edge: int(core.Barrier)}, {Edge: int(core.Gate)},
	{Edge: -1}, {Edge: -1}, {Feature: core.ToggleSwitch, Edge: -1}, {Feature: core.HoldSwitch, Edge: -1},
}

type mapEditor struct {
	mouseTool, previousTool        int
	panReturn                      int
	panning                        bool
	canvasOf, canvasClip           *ebiten.Image
	canvasBounds                   ui.Rect
	p                              page
	back                           scene
	m                              core.MapDefinition
	save                           func(core.MapDefinition) error
	name                           *field
	undo, redo                     []core.MapDefinition
	tool, category                 int
	group                          uint8
	pair, last, hover              int
	dirty, stroke, strokeChanged   bool
	edgeDrag                       bool
	prevX, prevY, zoom, panX, panY float64
	edgeX, edgeY, edgeA, edgeB     int
	nameEdits, autosave            int
	ask                            *askDialog
	issues                         []core.MapIssue
	bad                            [core.MaxSize * core.MaxSize]bool
	badEdges                       []bool
	capacity                       []uint8
	revision                       uint64
	note                           string
	snack                          snackbar

	canvas, field, saveB, testB, toolB, dock, info, capacityB, previewB ui.Rect
	control                                                             [7]ui.Rect
	dims                                                                [4]ui.Rect
	tabs                                                                [3]ui.Rect
	palette                                                             []ui.Rect
	paletteTools                                                        []int
	headings                                                            [3]ui.Rect
	dockScroll, toolbarScroll                                           ui.Scroll
	wide, portrait                                                      bool
	tools, more, channelsOpen, capacities, preview                      bool
	options                                                             ui.Rect
	option                                                              [1]ui.Rect
	channelB, channelPanel                                              ui.Rect
	channels                                                            [core.MaxMapGroups]ui.Rect
	channelDone                                                         ui.Rect
	helpTool                                                            int
	helpWidth                                                           float64
	helpLines                                                           []string
	revealTool                                                          bool

	mapImage                          *ebiten.Image
	mapRevision                       uint64
	mapTheme                          *ui.Palette
	mapMarks, mapCapacity, mapPreview bool
	mapCell                           float64
	previewState                      []uint8
	icons                             *ebiten.Image
	brushIcons                        [len(editorTools)]*ebiten.Image
	iconTheme                         *ui.Palette
	iconMarks, iconPreview            bool
	iconGroup                         uint8
}

func newMapEditor(a *app, back scene, m core.MapDefinition, save func(core.MapDefinition) error) *mapEditor {
	e := &mapEditor{back: back, m: m.Clone(), save: save, name: newField(64, false, printable), group: 1, pair: -1, last: -1, hover: -1, zoom: 1, helpTool: -1, revision: 1}
	e.name.set(m.Name)
	e.nameEdits = e.name.edits
	e.validate()
	return e
}
func (e *mapEditor) validate() {
	e.issues = e.m.Validate()
	e.note = ""
	e.snack = snackbar{}
	clear(e.bad[:])
	e.badEdges = slices.Grow(e.badEdges[:0], len(e.m.Edges))[:len(e.m.Edges)]
	clear(e.badEdges)
	if cap(e.capacity) < len(e.m.Cells) {
		e.capacity = make([]uint8, len(e.m.Cells))
	} else {
		e.capacity = e.capacity[:len(e.m.Cells)]
		clear(e.capacity)
	}
	for i, c := range e.m.Cells {
		if c.Terrain == core.Floor {
			e.capacity[i] = uint8(e.m.Capacity(i))
		}
	}
	for _, v := range e.issues {
		for _, i := range v.Cells {
			if i >= 0 && i < len(e.m.Cells) {
				e.bad[i] = true
			}
		}
		for _, i := range v.Edges {
			if i >= 0 && i < len(e.m.Edges) {
				e.badEdges[i] = true
			}
		}
	}
}
func (e *mapEditor) checkpoint() {
	e.undo = append(e.undo, e.m.Clone())
	if len(e.undo) > 64 {
		copy(e.undo, e.undo[len(e.undo)-64:])
		e.undo = e.undo[:64]
	}
	e.redo = nil
}
func (e *mapEditor) history(redo bool) {
	from, to := &e.undo, &e.redo
	if redo {
		from, to = &e.redo, &e.undo
	}
	if len(*from) == 0 {
		return
	}
	*to = append(*to, e.m.Clone())
	e.m = (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	e.name.set(e.m.Name)
	e.nameEdits = e.name.edits
	e.pair = -1
	e.changed()
}
func (e *mapEditor) changed() { e.dirty = true; e.autosave = 30; e.revision++; e.validate() }
func (e *mapEditor) recoverSave() {
	data, err := core.EncodeMap(e.m)
	if err == nil {
		err = platform.Save("map-draft.orbitrixmap", data)
	}
	if err != nil {
		e.notify("Draft could not be saved: " + err.Error())
	}
}

func (e *mapEditor) grid() (cell, ox, oy float64) {
	cell = math.Max(4, math.Min(e.canvas.W/float64(e.m.Cols), e.canvas.H/float64(e.m.Rows))) * e.zoom
	ox = e.canvas.Cx() - cell*float64(e.m.Cols)/2 + e.panX
	oy = e.canvas.Cy() - cell*float64(e.m.Rows)/2 + e.panY
	return
}
func (e *mapEditor) cellAt(x, y float64) int {
	if !e.canvas.Contains(x, y) {
		return -1
	}
	k, ox, oy := e.grid()
	c, r := int(math.Floor((x-ox)/k)), int(math.Floor((y-oy)/k))
	if c < 0 || r < 0 || c >= e.m.Cols || r >= e.m.Rows {
		return -1
	}
	return c*e.m.Rows + r
}
func (e *mapEditor) edgeAt(i int, x, y float64) (int, int) {
	if i < 0 {
		return -1, -1
	}
	k, ox, oy := e.grid()
	c, r := i/e.m.Rows, i%e.m.Rows
	fx, fy := (x-ox)/k-float64(c), (y-oy)/k-float64(r)
	d, dist := 0, fx
	for j, v := range [3]float64{1 - fx, fy, 1 - fy} {
		if v < dist {
			d, dist = j+1, v
		}
	}
	nc, nr := c, r
	switch d {
	case 0:
		nc--
	case 1:
		nc++
	case 2:
		nr--
	case 3:
		nr++
	}
	if nc < 0 || nr < 0 || nc >= e.m.Cols || nr >= e.m.Rows {
		return -1, -1
	}
	j := nc*e.m.Rows + nr
	return min(i, j), max(i, j)
}

func (e *mapEditor) edgeVertexAt(x, y float64) (int, int) {
	k, ox, oy := e.grid()
	return max(0, min(e.m.Cols, int(math.Round((x-ox)/k)))), max(0, min(e.m.Rows, int(math.Round((y-oy)/k))))
}

// Trace between snapped corners, rather than picking every tile side the pointer passes.
func (e *mapEditor) paintEdgeStroke(x, y float64) bool {
	c, r := e.edgeVertexAt(x, y)
	dx, dy := ui.AbsInt(c-e.edgeX), ui.AbsInt(r-e.edgeY)
	if dx == 0 && dy == 0 {
		return e.paintEdge(e.edgeA, e.edgeB)
	}
	if !e.edgeDrag {
		e.edgeDrag = true
		// A tap near a corner can pick the other side before the drag reveals its direction.
		if e.strokeChanged && (e.edgeB-e.edgeA == 1) != (dx > dy) {
			e.m.Edges = append(e.m.Edges[:0], e.undo[len(e.undo)-1].Edges...)
			e.strokeChanged = false
			e.revision++
		}
	}
	sx, sy := 1, 1
	if c < e.edgeX {
		sx = -1
	}
	if r < e.edgeY {
		sy = -1
	}
	changed := false
	for ix, iy := 0, 0; ix < dx || iy < dy; {
		a, b := -1, -1
		if (2*ix+1)*dy < (2*iy+1)*dx {
			nc := e.edgeX + sx
			if e.edgeY > 0 && e.edgeY < e.m.Rows {
				a = min(e.edgeX, nc)*e.m.Rows + e.edgeY - 1
				b = a + 1
			}
			e.edgeX = nc
			ix++
		} else {
			nr := e.edgeY + sy
			if e.edgeX > 0 && e.edgeX < e.m.Cols {
				a = (e.edgeX-1)*e.m.Rows + min(e.edgeY, nr)
				b = a + e.m.Rows
			}
			e.edgeY = nr
			iy++
		}
		changed = e.paintEdge(a, b) || changed
	}
	return changed
}

func (e *mapEditor) paintEdge(a, b int) bool {
	e.edgeA, e.edgeB = a, b
	if a < 0 {
		return false
	}
	next := core.MapEdge{A: a, B: b, Kind: core.EdgeKind(editorBrushes[e.tool].Edge)}
	if e.channelTool() {
		next.Group = e.group
	}
	for j, edge := range e.m.Edges {
		if min(edge.A, edge.B) != a || max(edge.A, edge.B) != b {
			continue
		}
		if edge == next {
			return false
		}
		e.m.Edges[j] = next
		e.revision++
		return true
	}
	if e.m.Cells[a].Terrain != core.Floor || e.m.Cells[b].Terrain != core.Floor {
		e.notify("Place edges between two floor cells.")
		return false
	}
	e.m.Edges = append(e.m.Edges, next)
	e.revision++
	return true
}

// eraseEdgeAt is shared by the hover preview and paint, so the highlight matches what a tap removes.
func (e *mapEditor) eraseEdgeAt(i int, x, y float64) int {
	if i < 0 {
		return -1
	}
	k, ox, oy := e.grid()
	fx, fy := (x-ox)/k-float64(i/e.m.Rows), (y-oy)/k-float64(i%e.m.Rows)
	if min(fx, 1-fx, fy, 1-fy) > .2 {
		return -1
	}
	a, b := e.edgeAt(i, x, y)
	if a < 0 {
		return -1
	}
	for j, edge := range e.m.Edges {
		if min(edge.A, edge.B) == a && max(edge.A, edge.B) == b {
			return j
		}
	}
	return -1
}

func (e *mapEditor) paint(i int, x, y float64) bool {
	if i < 0 {
		return false
	}
	if e.pair >= 0 && (e.m.Cells[e.pair].Feature != core.Link || e.m.Cells[e.pair].Terrain != core.Floor) {
		e.pair = -1
	}
	if e.pair == i && e.tool != brushLink {
		e.pair = -1
	}
	if editorBrushes[e.tool].Edge >= 0 {
		return e.paintEdge(e.edgeAt(i, x, y))
	}
	if e.tool == brushErase {
		if edge := e.eraseEdgeAt(i, x, y); edge >= 0 {
			e.m.Edges = append(e.m.Edges[:edge], e.m.Edges[edge+1:]...)
			e.revision++
			return true
		}
	}
	var next core.MapCell
	switch e.tool {
	case brushFloor, brushWall, brushVoid, brushBreak, brushErase:
		next.Terrain = core.Floor
		switch e.tool {
		case brushWall:
			next.Terrain = core.Wall
		case brushVoid:
			next.Terrain = core.Void
		case brushBreak:
			next.Terrain = core.BreakableWall
		}
	case brushLink:
		if i == e.last {
			return false
		}
		if e.m.Cells[i].Terrain != core.Floor {
			e.notify("Links need floor underneath.")
			return false
		}
		if e.pair >= 0 && e.pair != i {
			if ui.AbsInt(e.pair/e.m.Rows-i/e.m.Rows)+ui.AbsInt(e.pair%e.m.Rows-i%e.m.Rows) == 1 {
				e.notify("Choose a distant cell for the partner portal.")
				return false
			}
			next = core.MapCell{Feature: core.Link, Group: e.m.Cells[e.pair].Group}
			e.pair = -1
		} else {
			var used [256]bool
			for _, c := range e.m.Cells {
				if c.Feature == core.Link {
					used[c.Group] = true
				}
			}
			if e.group == 0 || used[e.group] {
				e.notify("Choose an unused pair color.")
				return false
			}
			next = core.MapCell{Feature: core.Link, Group: e.group}
			e.pair = i
		}
	case brushReservoir, brushLatch, brushToggle, brushHold:
		if e.m.Cells[i].Terrain != core.Floor {
			e.notify("Devices need floor underneath.")
			return false
		}
		next.Feature = editorBrushes[e.tool].Feature
		if next.Feature.IsSwitch() {
			next.Group = e.group
			for j, c := range e.m.Cells {
				if j != i && c.Feature.IsSwitch() && c.Group == e.group {
					e.notify("This circuit already has a switch. Choose another circuit color.")
					return false
				}
			}
		}
	default:
		return false
	}
	if e.m.Cells[i] == next {
		return false
	}
	e.m.Cells[i] = next
	e.revision++
	return true
}
func resizeMap(m core.MapDefinition, cols, rows int) core.MapDefinition {
	n := core.NewMap(cols, rows)
	n.Name = m.Name
	for c := 0; c < min(cols, m.Cols); c++ {
		for r := 0; r < min(rows, m.Rows); r++ {
			n.Cells[c*rows+r] = m.Cells[c*m.Rows+r]
		}
	}
	for _, edge := range m.Edges {
		ac, ar := edge.A/m.Rows, edge.A%m.Rows
		bc, br := edge.B/m.Rows, edge.B%m.Rows
		if ac < cols && bc < cols && ar < rows && br < rows {
			edge.A, edge.B = ac*rows+ar, bc*rows+br
			n.Edges = append(n.Edges, edge)
		}
	}
	return n
}
func (e *mapEditor) selectTool(tool int) {
	e.tool = tool
	e.revealTool = true
	if tool != brushLink {
		e.pair = -1
	}
	for g, c := range editorGroups {
		for _, t := range c.tools {
			if t == tool {
				e.category = g
			}
		}
	}
}
func (e *mapEditor) channelTool() bool {
	brush := editorBrushes[e.tool]
	return brush.Feature == core.Link || brush.Edge == int(core.Gate) || brush.Feature.IsSwitch()
}

func (e *mapEditor) finishStroke() {
	if e.stroke {
		e.stroke = false
		if e.strokeChanged {
			e.changed()
		} else if len(e.undo) > 0 {
			e.undo = e.undo[:len(e.undo)-1]
		}
	}
	e.panning = false
}

// Temporary hand/erase tools do not select a palette tool or discard a pending Link pair.
func (e *mapEditor) updateMouseTool(a *app) (pressed bool) {
	mode := 0
	if !a.in.finger && e.ask == nil && !e.more && !e.channelsOpen && !e.tools && (e.mouseTool != 0 || e.canvas.Contains(a.in.x, a.in.y)) {
		if a.in.middle {
			mode = 1
		} else if a.in.right {
			mode = 2
		}
	}
	if mode == e.mouseTool {
		return false
	}
	e.finishStroke()
	if e.mouseTool == 0 {
		e.previousTool = e.tool
	}
	e.mouseTool = mode
	switch mode {
	case 0:
		e.tool = e.previousTool
	case 1:
		e.tool = brushPan
	case 2:
		e.tool = brushErase
	}
	if mode != 0 {
		e.name.focus(a, false)
	}
	return mode != 0
}

func (e *mapEditor) update(a *app) error {
	e.layout(a)
	e.hover = -1
	mousePressed := e.updateMouseTool(a)
	if e.ask != nil {
		yes, back := e.ask.update(a)
		if yes {
			_ = platform.Save("map-draft.orbitrixmap", nil)
			a.next = e.back
		}
		if back {
			e.ask = nil
		}
		return nil
	}
	if e.channelsOpen {
		return e.updateChannels(a)
	}
	if e.more {
		return e.updateOptions(a)
	}
	if e.tools {
		if a.in.esc {
			e.tools = false
			return nil
		}
		for i, r := range e.tabs {
			if a.clicked(r) {
				e.category = i
				e.selectTool(editorGroups[i].tools[0])
				e.tools = false
				return nil
			}
		}
		if a.in.clicked {
			e.tools = false
		}
		return nil
	}
	if bar := e.snackbar(); bar != nil {
		_, over := bar.update(a)
		if bar.expired() {
			e.note = ""
			e.snack = snackbar{}
		}
		if over {
			a.pointer = true
			if a.in.clicked {
				e.note = ""
			}
			return nil
		}
	}
	if e.portrait {
		moved := e.toolbarScroll.Update(a.ui())
		moved = e.dockScroll.Update(a.ui()) || moved
		if moved {
			e.layout(a)
		}
		if moved || e.toolbarScroll.Dragging || e.dockScroll.Dragging {
			return nil
		}
	}
	if !e.name.focused && !e.stroke && e.mouseTool == 0 {
		mod := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
		if mod && inpututil.IsKeyJustPressed(ebiten.KeyZ) {
			e.history(ebiten.IsKeyPressed(ebiten.KeyShift))
			return nil
		}
		if mod && inpututil.IsKeyJustPressed(ebiten.KeyY) {
			e.history(true)
			return nil
		}
		if !ebiten.IsKeyPressed(ebiten.KeyAlt) {
			switch {
			case inpututil.IsKeyJustPressed(ebiten.KeyEqual), inpututil.IsKeyJustPressed(ebiten.KeyNumpadAdd):
				e.viewAction(6)
				return nil
			case inpututil.IsKeyJustPressed(ebiten.KeyMinus), inpututil.IsKeyJustPressed(ebiten.KeyNumpadSubtract):
				e.viewAction(4)
				return nil
			case inpututil.IsKeyJustPressed(ebiten.KeyDigit0), inpututil.IsKeyJustPressed(ebiten.KeyNumpad0):
				e.viewAction(5)
				return nil
			}
		}
		if !mod && !ebiten.IsKeyPressed(ebiten.KeyAlt) && inpututil.IsKeyJustPressed(ebiten.KeyM) {
			e.togglePan()
			return nil
		}
		if !mod && !ebiten.IsKeyPressed(ebiten.KeyAlt) && inpututil.IsKeyJustPressed(ebiten.KeyE) {
			e.selectTool(brushErase)
			return nil
		}
	}
	if e.name.focused && a.in.esc {
		e.name.focus(a, false)
		return nil
	}
	if a.in.pressed && !e.field.Contains(a.in.x, a.in.y) {
		e.name.focus(a, false)
	}
	if e.p.backed(a) {
		if e.dirty {
			e.recoverSave()
			e.ask = &askDialog{text: "Discard the changes to this map?", yes: "Discard changes"}
		} else {
			a.next = e.back
		}
		return nil
	}
	a.hover(e.field)
	e.name.tap(a, e.field)
	if e.name.update(a) {
		e.name.focus(a, false)
	}
	if e.name.edits != e.nameEdits {
		e.nameEdits = e.name.edits
		e.m.Name = string(e.name.shown)
		e.dirty = true
		e.autosave = 30
	}
	if e.autosave > 0 && !e.stroke {
		e.autosave--
		if e.autosave == 0 {
			e.recoverSave()
		}
	}
	if e.mouseTool != 0 {
		e.updateCanvas(a, mousePressed, true)
		return nil
	}
	switch {
	case (!e.portrait || a.over(e.toolbarScroll.Viewport)) && a.clicked(e.saveB):
		e.m.Name = e.name.value()
		e.nameEdits = e.name.edits
		if err := e.save(e.m.Clone()); err != nil {
			e.notify(err.Error())
		} else {
			e.dirty = false
			e.autosave = 0
			e.notify("Map saved")
			_ = platform.Save("map-draft.orbitrixmap", nil)
		}
		return nil
	case len(e.issues) == 0 && (!e.portrait || a.over(e.toolbarScroll.Viewport)) && a.clicked(e.testB):
		e.validate()
		if len(e.issues) == 0 {
			e.recoverSave()
			a.next = newMapTestSetup(a, e, e.m)
		} else {
			e.note = ""
		}
		return nil
	case (!e.portrait || a.over(e.toolbarScroll.Viewport)) && a.clicked(e.capacityB):
		e.capacities = !e.capacities
		return nil
	case (!e.portrait || a.over(e.toolbarScroll.Viewport)) && a.clicked(e.previewB):
		e.preview = !e.preview
		return nil
	case (!e.portrait || a.over(e.toolbarScroll.Viewport)) && a.clicked(e.toolB):
		e.name.focus(a, false)
		e.more = true
		return nil
	case e.channelTool() && a.clicked(e.channelB):
		e.channelsOpen = true
		return nil
	}
	for i, r := range e.control {
		if i == 0 && len(e.undo) == 0 || i == 1 && len(e.redo) == 0 {
			continue
		}
		if e.portrait && !a.over(e.toolbarScroll.Viewport) || !a.clicked(r) {
			continue
		}
		switch i {
		case 0:
			e.history(false)
		case 1:
			e.history(true)
		case 2:
			e.selectTool(brushErase)
		case 3:
			e.togglePan()
		case 4, 5, 6:
			e.viewAction(i)
		}
		return nil
	}
	if !e.portrait && e.dockScroll.Update(a.ui()) {
		e.layout(a)
	}
	if e.dock.Contains(a.in.x, a.in.y) {
		for i, r := range e.tabs {
			if r.W > 0 && a.clicked(r) {
				if e.portrait {
					e.category = i
					e.selectTool(editorGroups[i].tools[0])
				} else {
					e.tools = true
				}
				return nil
			}
		}
		if e.dockScroll.Viewport.Contains(a.in.x, a.in.y) {
			for i, r := range e.palette {
				if a.clicked(r) {
					e.selectTool(e.paletteTools[i])
					return nil
				}
			}
		}
		return nil
	}
	e.updateCanvas(a, a.in.pressed, a.in.down)
	return nil
}

func (e *mapEditor) updateCanvas(a *app, pressed, down bool) {
	revision := e.revision
	if e.canvas.Contains(a.in.x, a.in.y) && a.in.wheel != 0 {
		e.zoom = math.Max(.5, math.Min(16, e.zoom*math.Pow(1.15, a.in.wheel)))
	}
	i := e.cellAt(a.in.x, a.in.y)
	e.hover = i
	if i >= 0 {
		a.hover(e.canvas)
	}
	if pressed && e.canvas.Contains(a.in.x, a.in.y) {
		e.prevX, e.prevY = a.in.x, a.in.y
		e.panning = e.tool == brushPan
		if e.tool != brushPan {
			e.checkpoint()
			e.stroke = true
			e.strokeChanged = false
			e.last = -1
			e.edgeX, e.edgeY = e.edgeVertexAt(a.in.x, a.in.y)
			e.edgeA, e.edgeB = -1, -1
			e.edgeDrag = false
			e.strokeChanged = e.paint(i, a.in.x, a.in.y)
			e.last = i
		}
	}
	if down && (e.canvas.Contains(a.in.x, a.in.y) || e.panning) {
		if e.tool == brushPan {
			if e.panning && !pressed {
				e.panX += a.in.x - e.prevX
				e.panY += a.in.y - e.prevY
			}
		} else if e.stroke && (i != e.last || (editorBrushes[e.tool].Edge >= 0 || e.tool == brushErase) && (a.in.x != e.prevX || a.in.y != e.prevY)) {
			if editorBrushes[e.tool].Edge >= 0 {
				e.strokeChanged = e.paintEdgeStroke(a.in.x, a.in.y) || e.strokeChanged
			} else if e.tool == brushLink || editorBrushes[e.tool].Feature.IsSwitch() {
				e.strokeChanged = e.paint(i, a.in.x, a.in.y) || e.strokeChanged
			} else {
				k, _, _ := e.grid()
				steps := max(1, int(math.Ceil(math.Hypot(a.in.x-e.prevX, a.in.y-e.prevY)/(k*.3))))
				for j := 1; j <= steps; j++ {
					f := float64(j) / float64(steps)
					x, y := e.prevX+(a.in.x-e.prevX)*f, e.prevY+(a.in.y-e.prevY)*f
					e.strokeChanged = e.paint(e.cellAt(x, y), x, y) || e.strokeChanged
				}
			}
			e.last = i
		}
		e.prevX, e.prevY = a.in.x, a.in.y
	}
	// Refresh once per changed input frame, before a held stroke can be drawn.
	if e.revision != revision {
		e.validate()
	}
	if !down {
		e.finishStroke()
	}
	a.moveCursor = !a.in.finger && e.tool == brushPan && (e.canvas.Contains(a.in.x, a.in.y) || e.panning)
}

func (e *mapEditor) updateOptions(a *app) error {
	if a.in.esc || a.clicked(e.option[0]) {
		e.more = false
		return nil
	}
	for i, r := range e.dims {
		if !a.clicked(r) {
			continue
		}
		c, rows := e.m.Cols, e.m.Rows
		switch i {
		case 0:
			c--
		case 1:
			c++
		case 2:
			rows--
		case 3:
			rows++
		}
		if c >= 3 && c <= 40 && rows >= 3 && rows <= 40 {
			e.checkpoint()
			e.m = resizeMap(e.m, c, rows)
			e.pair = -1
			e.panX, e.panY = 0, 0
			e.changed()
		}
		return nil
	}
	return nil
}
func (e *mapEditor) updateChannels(a *app) error {
	if a.in.esc || a.clicked(e.channelDone) {
		e.channelsOpen = false
		return nil
	}
	for i, r := range e.channels {
		if a.clicked(r) {
			e.group = uint8(i + 1)
			e.channelsOpen = false
			return nil
		}
	}
	return nil
}

type mapTestSetup struct {
	setup    *setupScene
	settings settings
	back     scene
}

func newMapTestSetup(a *app, back scene, m core.MapDefinition) *mapTestSetup {
	s := *a.set
	s.seats = append([]int(nil), s.seats...)
	s.Seats = append([]string(nil), s.Seats...)
	m = m.Clone()
	s.Map = &m
	s.Cols, s.Rows = m.Cols, m.Rows
	setup := newSetup()
	setup.test = true
	return &mapTestSetup{setup: setup, settings: s, back: back}
}
func (s *mapTestSetup) update(a *app) error {
	if a.in.esc {
		a.next = s.back
		return nil
	}
	old := a.set
	a.set = &s.settings
	defer func() { a.set = old }()
	err := s.setup.update(a)
	if p, ok := a.next.(*playScene); ok {
		p.returnTo = s.back
	}
	if _, ok := a.next.(*homeScene); ok {
		a.next = s.back
	}
	return err
}
func (s *mapTestSetup) draw(dst *ebiten.Image, a *app) {
	old := a.set
	a.set = &s.settings
	defer func() { a.set = old }()
	s.setup.draw(dst, a)
}
func (e *mapEditor) setCaret() {
	e.name.caret = image.Rect(int(e.field.X), int(e.field.Y), int(e.field.Right()), int(e.field.Bottom()))
}

func (e *mapEditor) notify(message string) {
	e.note = message
	e.snack = snackbar{message: message, timeout: snackDuration}
}

func (e *mapEditor) snackbar() *snackbar {
	message, timeout := e.note, snackDuration
	if message == "" {
		if len(e.issues) == 0 {
			return nil
		}
		message, timeout = e.issues[0].Message, 0
	}
	if e.snack.message != message || e.snack.timeout != timeout {
		e.snack = snackbar{message: message, timeout: timeout}
	}
	if e.snack.expired() {
		return nil
	}
	return &e.snack
}

func (e *mapEditor) togglePan() {
	if e.tool == brushPan {
		e.selectTool(e.panReturn)
	} else {
		e.panReturn = e.tool
		e.selectTool(brushPan)
	}
}

func (e *mapEditor) viewAction(action int) {
	switch action {
	case 4:
		e.zoom = math.Max(.5, e.zoom/1.25)
	case 5:
		e.zoom = 1
		e.panX, e.panY = 0, 0
	case 6:
		e.zoom = math.Min(16, e.zoom*1.25)
	}
}
