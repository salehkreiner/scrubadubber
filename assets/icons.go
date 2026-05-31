// Package assets embeds the generated tray/menubar status icons.
//
// The icon files are produced by `go run ./assets/gen`. Both PNG (macOS) and
// ICO (Windows) variants are embedded; Icon returns whichever the systray
// library expects on the current platform.
package assets

import (
	_ "embed"
	"runtime"
)

// Status identifies which colored status icon to display.
type Status int

const (
	// Grey: the app itself is starting up.
	Grey Status = iota
	// Green: Hub healthy and bridge configured (protected).
	Green
	// Yellow: Hub running but health check degraded.
	Yellow
	// Red: Hub down or unreachable.
	Red
)

//go:embed icon_grey.png
var greyPNG []byte

//go:embed icon_green.png
var greenPNG []byte

//go:embed icon_yellow.png
var yellowPNG []byte

//go:embed icon_red.png
var redPNG []byte

//go:embed icon_grey.ico
var greyICO []byte

//go:embed icon_green.ico
var greenICO []byte

//go:embed icon_yellow.ico
var yellowICO []byte

//go:embed icon_red.ico
var redICO []byte

// Icon returns the icon bytes for the given status in the format the systray
// library expects on the current platform: ICO on Windows, PNG elsewhere.
func Icon(s Status) []byte {
	if runtime.GOOS == "windows" {
		switch s {
		case Green:
			return greenICO
		case Yellow:
			return yellowICO
		case Red:
			return redICO
		default:
			return greyICO
		}
	}
	switch s {
	case Green:
		return greenPNG
	case Yellow:
		return yellowPNG
	case Red:
		return redPNG
	default:
		return greyPNG
	}
}
