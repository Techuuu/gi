// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"goki.dev/gi/v2/gi"
	"goki.dev/gi/v2/gimain"
	"goki.dev/gi/v2/giv"
	"goki.dev/girl/styles"
)

func main() { gimain.Run(app) }

func app() {
	width := 1024
	height := 768

	win := gi.NewMainRenderWin("gogi-tabview-test", "GoGi TabView Test", width, height)

	vp := win.WinScene()
	updt := vp.UpdateStart()

	mfr := win.SetMainFrame()

	tv := gi.NewTabView(mfr, "tv")
	tv.NewTabButton = true

	lbl1 := tv.NewTab(gi.LabelType, "This is Label1").(*gi.Label)
	lbl1.SetText("this is the contents of the first tab")
	lbl1.SetProp("white-space", styles.WhiteSpaceNormal) // wrap

	lbl2 := tv.NewTab(gi.LabelType, "And this Label2").(*gi.Label)
	lbl2.SetText("this is the contents of the second tab")
	lbl2.SetProp("white-space", styles.WhiteSpaceNormal) // wrap

	tv1i, tv1ly := tv.NewTabLayout(giv.TypeTextView, "TextView1")
	tv1ly.SetStretchMax()
	tv1 := tv1i.(*giv.TextView)
	tb1 := &giv.TextBuf{}
	tb1.InitName(tb1, "tb1")
	tv1.SetBuf(tb1)
	tb1.SetText([]byte("TextView1 text"))

	tv.SelectTabIndex(0)

	// main menu
	appnm := gi.AppName()
	mmen := win.MainMenu
	mmen.ConfigMenus([]string{appnm, "Edit", "RenderWin"})

	amen := win.MainMenu.ChildByName(appnm, 0).(*gi.Action)
	amen.Menu = make(gi.MenuStage, 0, 10)
	amen.Menu.AddAppMenu(win)

	emen := win.MainMenu.ChildByName("Edit", 1).(*gi.Action)
	emen.Menu = make(gi.MenuStage, 0, 10)
	emen.Menu.AddCopyCutPaste(win)

	win.SetCloseCleanFunc(func(w *gi.RenderWin) {
		go gi.Quit() // once main window is closed, quit
	})

	win.MainMenuUpdated()

	vp.UpdateEndNoSig(updt)

	win.StartEventLoop()
}
