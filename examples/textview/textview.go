// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"goki.dev/gi/v2/gi"
	"goki.dev/gi/v2/gimain"
	"goki.dev/gi/v2/giv"
	"goki.dev/girl/styles"
	"goki.dev/girl/units"
)

var samplefile gi.FileName = "sample.go"

func main() { gimain.Run(app) }

func app() {
	width := 1024
	height := 768

	// gi.LayoutTrace = true

	gi.SetAppName("textview")
	gi.SetAppAbout(`This is a demo of the TextView in the <b>GoGi</b> graphical interface system, within the <b>GoKi</b> tree framework.  See <a href="https://github.com/goki">GoKi on GitHub</a>`)

	win := gi.NewMainRenderWin("gogi-textview-test", "GoGi TextView Test", width, height)

	vp := win.WinScene()
	updt := vp.UpdateStart()

	// // style sheet
	// var css = ki.Props{
	// 	"kbd": ki.Props{
	// 		"color": "blue",
	// 	},
	// }
	// vp.CSS = css

	mfr := win.SetMainFrame()

	trow := mfr.NewChild(gi.LayoutType, "trow").(*gi.Layout)
	trow.Lay = gi.LayoutHoriz
	trow.SetStretchMaxWidth()

	title := trow.NewChild(gi.LabelType, "title").(*gi.Label)
	hdrText := `This is a <b>test</b> of the TextView`
	title.Text = hdrText
	title.SetProp("text-align", styles.AlignCenter)
	title.SetProp("vertical-align", styles.AlignTop)
	title.SetProp("font-size", "x-large")

	splt := mfr.NewChild(gi.TypeSplitView, "split-view").(*gi.SplitView)
	splt.SetSplits(.5, .5)
	// these are all inherited so we can put them at the top "editor panel" level
	splt.SetProp("white-space", styles.WhiteSpacePreWrap)
	splt.SetProp("tab-size", 4)
	splt.SetProp("font-family", gi.Prefs.MonoFont)
	splt.SetProp("line-height", 1.1)

	// generally need to put text view within its own layout for scrolling
	txly1 := splt.NewChild(gi.LayoutType, "view-layout-1").(*gi.Layout)
	txly1.SetStretchMaxWidth()
	txly1.SetStretchMaxHeight()
	txly1.SetMinPrefWidth(units.Ch(20))
	txly1.SetMinPrefHeight(units.Ch(10))

	txed1 := txly1.NewChild(giv.TypeTextView, "textview-1").(*giv.TextView)
	txed1.Scene = vp

	// generally need to put text view within its own layout for scrolling
	txly2 := splt.NewChild(gi.LayoutType, "view-layout-2").(*gi.Layout)
	txly2.SetStretchMaxWidth()
	txly2.SetStretchMaxHeight()
	txly2.SetMinPrefWidth(units.Ch(20))
	txly2.SetMinPrefHeight(units.Ch(10))

	txed2 := txly2.NewChild(giv.TypeTextView, "textview-2").(*giv.TextView)
	txed2.Scene = vp

	txbuf := giv.NewTextBuf()
	txed1.SetBuf(txbuf)
	txed2.SetBuf(txbuf)

	txbuf.Hi.Lang = "Go"
	txbuf.Open(samplefile)

	// main menu
	appnm := gi.AppName()
	mmen := win.MainMenu
	mmen.ConfigMenus([]string{appnm, "Edit", "RenderWin"})

	amen := win.MainMenu.ChildByName(appnm, 0).(*gi.Button)
	amen.Menu = make(gi.MenuStage, 0, 10)
	amen.Menu.AddAppMenu(win)

	emen := win.MainMenu.ChildByName("Edit", 1).(*gi.Button)
	emen.Menu = make(gi.MenuStage, 0, 10)
	emen.Menu.AddCopyCutPaste(win)

	win.SetCloseCleanFunc(func(w *gi.RenderWin) {
		go gi.Quit() // once main window is closed, quit
	})

	win.MainMenuUpdated()
	vp.UpdateEndNoSig(updt)

	win.StartEventLoop()
}
