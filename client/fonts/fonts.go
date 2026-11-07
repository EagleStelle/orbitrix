// Package fonts embeds the OFL fonts shared by every client target.
package fonts

import _ "embed"

var (
	//go:embed Unbounded.ttf
	Unbounded []byte
	//go:embed Archivo.ttf
	Archivo []byte
)
