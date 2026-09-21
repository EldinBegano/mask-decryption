// SPDX-License-Identifier: GPL-3.0-or-later

// Command mlp-gui is the desktop front end for mlp: pick or drop a file or
// folder, encrypt or decrypt it, and back up the keyfile. It uses the same
// keyfile and .mlp format as the mlp command.
package main

import (
	_ "embed"
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

//go:embed assets/mlp-gui.svg
var iconSVG []byte

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println("mlp-gui", version)
		return
	}

	a := app.NewWithID("io.github.eldinbegano.mlp-gui")
	a.SetIcon(fyne.NewStaticResource("mlp-gui.svg", iconSVG))

	w := a.NewWindow("mlp")
	newUI(w)
	w.Resize(fyne.NewSize(580, 640))
	w.ShowAndRun()
}
