// Package core holds the Orbitrix game rules: board, moves, wave-by-wave explosion
// resolution, alive/win/turn order, seeded RNG and state hashing.
//
// It must not import Ebitengine, net or OS packages: the same code runs on every
// client target and on the server.
package core
