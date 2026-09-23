// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/EldinBegano/mask-decryption/internal/keystore"
	"github.com/EldinBegano/mask-decryption/internal/ops"
)

type logKind int

const (
	logOK logKind = iota
	logErr
	logWarn
)

type logLine struct {
	kind logKind
	text string
}

// ui holds the window state. Every field is touched only on the Fyne
// main goroutine; background work reports back through fyne.Do.
type ui struct {
	win fyne.Window

	target string
	isDir  bool

	pathLabel *widget.Label
	hintLabel *widget.Label
	encBtn    *widget.Button
	decBtn    *widget.Button
	busy      *widget.ProgressBarInfinite
	list      *widget.List
	summary   *widget.Label

	lines   []logLine
	running bool

	// finished, if set, is called on the main goroutine when a run ends.
	finished func()
}

func newUI(win fyne.Window) *ui {
	u := &ui{win: win}

	title := widget.NewLabelWithStyle("mlp", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Encrypt and decrypt files with AES-256-GCM")
	keyBtn := widget.NewButtonWithIcon("Keyfile", theme.SettingsIcon(), u.showKeyDialog)
	header := container.NewBorder(nil, nil, container.NewVBox(title, subtitle), keyBtn)

	dropText := widget.NewLabelWithStyle("Drop a file or folder here", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	u.pathLabel = widget.NewLabelWithStyle("Nothing selected", fyne.TextAlignCenter, fyne.TextStyle{})
	u.pathLabel.Wrapping = fyne.TextWrapBreak
	u.hintLabel = widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	u.hintLabel.Wrapping = fyne.TextWrapWord
	chooseFile := widget.NewButtonWithIcon("Choose file…", theme.FileIcon(), u.pickFile)
	chooseFolder := widget.NewButtonWithIcon("Choose folder…", theme.FolderOpenIcon(), u.pickFolder)
	dropCard := widget.NewCard("", "", container.NewVBox(
		dropText,
		u.pathLabel,
		u.hintLabel,
		container.NewGridWithColumns(2, chooseFile, chooseFolder),
	))

	u.encBtn = widget.NewButton("Encrypt", func() { u.run(true) })
	u.encBtn.Importance = widget.HighImportance
	u.decBtn = widget.NewButton("Decrypt", func() { u.run(false) })
	actions := container.NewGridWithColumns(2, u.encBtn, u.decBtn)

	u.busy = widget.NewProgressBarInfinite()
	u.busy.Hide()

	u.summary = widget.NewLabel("")
	u.list = widget.NewList(
		func() int { return len(u.lines) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.ConfirmIcon()), widget.NewLabel("template"))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container)
			icon := row.Objects[0].(*widget.Icon)
			label := row.Objects[1].(*widget.Label)
			l := u.lines[i]
			switch l.kind {
			case logOK:
				icon.SetResource(theme.ConfirmIcon())
			case logErr:
				icon.SetResource(theme.ErrorIcon())
			default:
				icon.SetResource(theme.WarningIcon())
			}
			label.SetText(l.text)
		},
	)
	results := widget.NewCard("Results", "", container.NewBorder(nil, u.summary, nil, nil, u.list))

	top := container.NewVBox(header, dropCard, actions, u.busy)
	win.SetContent(container.NewBorder(top, nil, nil, nil, results))
	win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			u.setTarget(uris[0].Path())
		}
	})

	u.refreshButtons()
	return u
}

func (u *ui) pickFile() {
	dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if r == nil {
			return
		}
		path := r.URI().Path()
		r.Close()
		u.setTarget(path)
	}, u.win).Show()
}

func (u *ui) pickFolder() {
	dialog.NewFolderOpen(func(l fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if l == nil {
			return
		}
		u.setTarget(l.Path())
	}, u.win).Show()
}

func (u *ui) setTarget(path string) {
	if u.running {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.target, u.isDir = path, info.IsDir()
	u.pathLabel.SetText(path)

	switch {
	case u.isDir:
		u.hintLabel.SetText("Every file in this folder and its subfolders will be processed.")
	case strings.HasSuffix(path, ".mlp"):
		u.hintLabel.SetText("Encrypted file: it can be decrypted.")
	default:
		u.hintLabel.SetText("Plain file: it can be encrypted.")
	}

	u.lines = nil
	u.summary.SetText("")
	u.list.Refresh()
	u.refreshButtons()
}

func (u *ui) refreshButtons() {
	canEnc, canDec := false, false
	if u.target != "" && !u.running {
		if u.isDir {
			canEnc, canDec = true, true
		} else if strings.HasSuffix(u.target, ".mlp") {
			canDec = true
		} else {
			canEnc = true
		}
	}
	setEnabled(u.encBtn, canEnc)
	setEnabled(u.decBtn, canDec)
}

func setEnabled(b *widget.Button, on bool) {
	if on {
		b.Enable()
	} else {
		b.Disable()
	}
}

// display shortens a path for the results list: relative to the chosen
// folder in batch mode, just the name for a single file.
func (u *ui) display(p string) string {
	if u.isDir {
		if rel, err := filepath.Rel(u.target, p); err == nil {
			return rel
		}
	}
	return filepath.Base(p)
}

func (u *ui) addLine(kind logKind, text string) {
	u.lines = append(u.lines, logLine{kind, text})
	u.list.Refresh()
	u.list.ScrollToBottom()
}

func (u *ui) addEvent(encrypt bool, e ops.Event) {
	switch e.Kind {
	case ops.EventDone:
		verb := "decrypted"
		if encrypt {
			verb = "encrypted"
		}
		note := ""
		if e.Result.Overwrote {
			note = "  (replaced existing)"
		}
		u.addLine(logOK, fmt.Sprintf("%s %s -> %s%s", verb, u.display(e.Result.Input), u.display(e.Result.Output), note))
		if e.Result.NonceFallback {
			u.addLine(logWarn, "Nonce counter was missing or corrupt; a random nonce was used.")
		}
		if e.Result.TimestampFailed {
			u.addLine(logWarn, "Could not restore the original timestamp (file itself is fine).")
		}
	case ops.EventFailed:
		u.addLine(logErr, e.Err.Error())
	}
}

func (u *ui) setRunning(on bool) {
	u.running = on
	if on {
		u.busy.Show()
		u.busy.Start()
	} else {
		u.busy.Stop()
		u.busy.Hide()
	}
	u.refreshButtons()
}

func summaryText(verb string, res ops.BatchResult) string {
	return fmt.Sprintf("%d %s, %d skipped, %d failed", len(res.Done), verb, len(res.Skipped), len(res.Failed))
}

// run encrypts or decrypts the current target on a background goroutine.
func (u *ui) run(encrypt bool) {
	if u.running || u.target == "" {
		return
	}
	u.lines = nil
	u.summary.SetText("")
	u.list.Refresh()
	u.setRunning(true)

	target, isDir := u.target, u.isDir
	verb := "decrypted"
	if encrypt {
		verb = "encrypted"
	}

	go func() {
		var keyCreatedDir string
		getKey := keystore.LoadKey
		if encrypt {
			getKey = func() ([]byte, error) {
				key, created, err := keystore.LoadOrCreateKey()
				if err == nil && created {
					keyCreatedDir, _ = keystore.ConfigDir()
				}
				return key, err
			}
		}
		on := func(e ops.Event) { fyne.Do(func() { u.addEvent(encrypt, e) }) }

		res, err := u.execute(encrypt, isDir, getKey, target, ops.Options{}, on)

		fyne.Do(func() {
			u.setRunning(false)
			if err != nil {
				u.summary.SetText("Nothing was processed.")
				u.reportStartError(err)
			} else {
				u.summary.SetText(summaryText(verb, res))
				if keyCreatedDir != "" {
					u.showNewKeyNotice(keyCreatedDir)
				}
				u.offerOverwrite(encrypt, getKey, res.Failed)
			}
			if u.finished != nil {
				u.finished()
			}
		})
	}()
}

// execute runs one file or one directory and always reports through on.
func (u *ui) execute(encrypt, isDir bool, getKey ops.KeyFunc, target string, opts ops.Options, on func(ops.Event)) (ops.BatchResult, error) {
	if isDir {
		if encrypt {
			return ops.EncryptDir(getKey, target, opts, on)
		}
		return ops.DecryptDir(getKey, target, opts, on)
	}

	var (
		r   ops.Result
		err error
	)
	if encrypt {
		r, err = ops.EncryptFile(getKey, target, "", opts)
	} else {
		r, err = ops.DecryptFile(getKey, target, "", opts)
	}

	var res ops.BatchResult
	if err != nil {
		// A missing key aborts the run rather than being one file's failure.
		if ops.KindOf(err) == ops.KindNoKey {
			return res, err
		}
		res.Failed = []ops.Failure{{Input: target, Output: r.Output, Err: err}}
		on(ops.Event{Kind: ops.EventFailed, Path: target, Output: r.Output, Err: err})
		return res, nil
	}
	res.Done = []ops.Result{r}
	on(ops.Event{Kind: ops.EventDone, Result: r})
	return res, nil
}

func (u *ui) reportStartError(err error) {
	if ops.KindOf(err) == ops.KindNoKey {
		dialog.ShowInformation("No keyfile found",
			"Decrypting needs the keyfile the files were encrypted with.\n\n"+
				"If you have a backup, restore it with Keyfile > Import backup.", u.win)
		return
	}
	dialog.ShowError(err, u.win)
}

func (u *ui) showNewKeyNotice(dir string) {
	dialog.ShowInformation("New keyfile created",
		"A new keyfile was created at:\n"+filepath.Join(dir, "keyfile")+
			"\n\nBack it up now (Keyfile > Export backup). If it is lost, every file "+
			"encrypted with it becomes permanently unreadable.", u.win)
}

// offerOverwrite asks whether to replace outputs that already existed, and
// if so redoes just those files with Force. The GUI's version of --force.
func (u *ui) offerOverwrite(encrypt bool, getKey ops.KeyFunc, failed []ops.Failure) {
	var existing []ops.Failure
	for _, f := range failed {
		if ops.KindOf(f.Err) == ops.KindExists {
			existing = append(existing, f)
		}
	}
	if len(existing) == 0 {
		return
	}

	var names []string
	for i, f := range existing {
		if i == 5 {
			names = append(names, fmt.Sprintf("… and %d more", len(existing)-5))
			break
		}
		names = append(names, u.display(f.Output))
	}
	msg := fmt.Sprintf("%d output file(s) already exist:\n\n%s\n\nReplace them?",
		len(existing), strings.Join(names, "\n"))

	dialog.NewConfirm("Replace existing files?", msg, func(yes bool) {
		if yes {
			u.retryForced(encrypt, getKey, existing)
		}
	}, u.win).Show()
}

func (u *ui) retryForced(encrypt bool, getKey ops.KeyFunc, existing []ops.Failure) {
	u.setRunning(true)
	verb := "decrypted"
	if encrypt {
		verb = "encrypted"
	}

	go func() {
		var res ops.BatchResult
		for _, f := range existing {
			var r ops.Result
			var err error
			if encrypt {
				r, err = ops.EncryptFile(getKey, f.Input, f.Output, ops.Options{Force: true})
			} else {
				r, err = ops.DecryptFile(getKey, f.Input, f.Output, ops.Options{Force: true})
			}
			if err != nil {
				res.Failed = append(res.Failed, ops.Failure{Input: f.Input, Output: f.Output, Err: err})
				fyne.Do(func() { u.addEvent(encrypt, ops.Event{Kind: ops.EventFailed, Err: err}) })
				continue
			}
			res.Done = append(res.Done, r)
			fyne.Do(func() { u.addEvent(encrypt, ops.Event{Kind: ops.EventDone, Result: r}) })
		}
		fyne.Do(func() {
			u.setRunning(false)
			u.summary.SetText("Replaced: " + summaryText(verb, res))
			if u.finished != nil {
				u.finished()
			}
		})
	}()
}

func (u *ui) showKeyDialog() {
	dir, _ := keystore.ConfigDir()
	info := widget.NewLabel("Your key is stored at:\n" + filepath.Join(dir, "keyfile") +
		"\n\nIf it is lost, every encrypted file becomes permanently unreadable. " +
		"Keep a backup somewhere safe, such as a USB stick.")
	info.Wrapping = fyne.TextWrapWord

	var d dialog.Dialog
	export := widget.NewButtonWithIcon("Export backup…", theme.UploadIcon(), func() {
		d.Hide()
		u.exportKey()
	})
	imp := widget.NewButtonWithIcon("Import backup…", theme.DownloadIcon(), func() {
		d.Hide()
		u.importKey()
	})
	d = dialog.NewCustom("Keyfile", "Close", container.NewVBox(info, export, imp), u.win)
	d.Resize(fyne.NewSize(460, 320))
	d.Show()
}

func (u *ui) exportKey() {
	dialog.NewFolderOpen(func(l fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if l == nil {
			return
		}
		if err := keystore.Export(l.Path()); err != nil {
			if errors.Is(err, keystore.ErrKeyfileMissing) {
				dialog.ShowInformation("No keyfile yet", "There is nothing to back up until you encrypt something.", u.win)
				return
			}
			dialog.ShowError(err, u.win)
			return
		}
		sum, _ := keystore.KeyfileSHA256()
		dialog.ShowInformation("Backup saved",
			"Keyfile and counter copied to:\n"+l.Path()+"\n\nSHA-256 of the keyfile:\n"+sum, u.win)
	}, u.win).Show()
}

func (u *ui) importKey() {
	dialog.NewFolderOpen(func(l fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if l == nil {
			return
		}
		hadOldKey, _ := keystore.Exists()
		doImport := func() {
			if err := keystore.Import(l.Path()); err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			msg := "The backup was restored."
			if hadOldKey {
				if oldPath, err := keystore.OldKeyPath(); err == nil {
					msg += "\n\nThe key that was active before is kept at:\n" + oldPath +
						"\n\nDelete it once you no longer need it."
				}
			}
			dialog.ShowInformation("Keyfile imported", msg, u.win)
		}
		if hadOldKey {
			dialog.NewConfirm("Replace current keyfile?",
				"A keyfile already exists. Importing replaces it and its nonce counter — files encrypted "+
					"with the current key can no longer be decrypted unless you restore it (it's kept as "+
					"keyfile.old, with counter.old, if you picked the wrong backup by mistake).",
				func(yes bool) {
					if yes {
						doImport()
					}
				}, u.win).Show()
			return
		}
		doImport()
	}, u.win).Show()
}
