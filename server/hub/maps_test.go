package hub_test

import (
	"orbitrix/core"
	"orbitrix/proto"
	"orbitrix/server/hub"
	"testing"
)

func TestMapDistributionResyncRejoinRematch(t *testing.T) {
	m := core.StarterMaps()[2]
	room := newRoom(t, hub.Config{Map: &m, Seats: humans(2)})
	a, b := join(t, room, "Ana", proto.Token{}), join(t, room, "Bo", proto.Token{})
	l := waitFor[proto.Lobby](a)
	if l.Map == nil || l.Map.Name != m.Name {
		t.Fatal("map missing from lobby")
	}
	m.Cells[0].Terrain = core.Void
	start(t, room, a, b)
	initial := waitFor[proto.Snapshot](a)
	a.track(initial)
	b.track(waitFor[proto.Snapshot](b))
	if a.game.Map == nil || a.game.Map.Cells[0].Terrain != core.Floor {
		t.Fatal("room map aliases caller")
	}
	a.send(proto.Move{C: 0, R: 0})
	a.track(waitFor[proto.MoveApplied](a))
	b.track(waitFor[proto.MoveApplied](b))
	a.send(proto.Resync{})
	resync := waitFor[proto.Snapshot](a)
	restored, err := core.Restore(resync.Game)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Hash() != a.game.Hash() {
		t.Fatal("resync changed custom map")
	}
	_ = b.conn.Close()
	rejoined := join(t, room, "Bo", b.token)
	rejoined.track(waitFor[proto.Snapshot](rejoined))
	if rejoined.game.Hash() != a.game.Hash() {
		t.Fatal("rejoin lost custom map")
	}
	if err := room.Kick(rejoined.seat); err != nil {
		t.Fatal(err)
	}
	if err := room.Skip(rejoined.seat); err != nil {
		t.Fatal(err)
	}
	a.track(waitFor[proto.MoveApplied](a))
	if !a.game.Over() {
		t.Fatal("pass should finish first round with one owner")
	}
	if err := room.Rematch(); err != nil {
		t.Fatal(err)
	}
	l = a.lobbyWhere(func(l proto.Lobby) bool { return !l.Playing })
	if l.Map == nil || l.Map.Name != initial.Game.Map.Name {
		t.Fatal("rematch lost map")
	}
	bad := core.NewMap(3, 3)
	bad.Cells[1].Terrain = core.Wall
	if err := room.SetMap(&bad); err == nil {
		t.Fatal("accepted low-degree map")
	}
	if err := room.SetMap(nil); err != nil {
		t.Fatal(err)
	}
	l = a.lobbyWhere(func(l proto.Lobby) bool { return l.Map == nil })
	if l.Cols != 9 || l.Rows != 9 {
		t.Fatal("classic reset changed dimensions")
	}
}
