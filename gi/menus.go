// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"image"

	"goki.dev/colors"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/girl/units"
	"goki.dev/goosi/events"
	"goki.dev/goosi/events/key"
	"goki.dev/icons"
	"goki.dev/ki/v2"
)

// Menu is a slice list of Buttons (or other Widgets)
// that are used for generating a Menu.
type Menu ki.Slice

/*
func (m Menu) MarshalJSON() ([]byte, error) {
	ks := (ki.Slice)(m)
	_ = ks
	// return ks.MarshalJSON()
	return nil, nil
}

func (m *Menu) UnmarshalJSON(b []byte) error {
	ks := (*ki.Slice)(m)
	_ = ks
	// return ks.UnmarshalJSON(b)
	return nil
}
*/

func (m *Menu) CopyFrom(men *Menu) {
	ks := (*ki.Slice)(m)
	ks.CopyFrom((ki.Slice)(*men))
}

// MakeMenuFunc is a callback for making a menu on demand, receives the object
// calling this function (typically a Button) and the menu
type MakeMenuFunc func(obj Widget, m *Menu)

// SetButton sets properties of given button
func (m *Menu) SetButton(bt *Button, opts ActOpts, fun func(bt *Button)) {
	nm := opts.Name
	if nm == "" {
		nm = opts.Label
	}
	if nm == "" {
		nm = string(opts.Icon)
	}
	bt.InitName(bt, nm)
	bt.Type = ButtonMenu
	bt.Text = opts.Label
	bt.Tooltip = opts.Tooltip
	bt.Icon = icons.Icon(opts.Icon)
	bt.Shortcut = key.Chord(opts.Shortcut).OSShortcut()
	if opts.ShortcutKey != KeyFunNil {
		bt.Shortcut = ShortcutForFun(opts.ShortcutKey)
		// todo: need a flag for menu-based?
	}
	bt.Data = opts.Data
	bt.UpdateFunc = opts.UpdateFunc
	if fun != nil {
		bt.OnClick(func(e events.Event) {
			fun(bt)
		})
	}
}

// AddButton adds an button to the menu using given options, and connects the
// button signal to given receiver object and function, along with given data
// which is stored on the button and then passed in the button signal.
// Optional updateFunc is a function called prior to showing the menu to
// update the buttons (enabled or not typically).
func (m *Menu) AddButton(opts ActOpts, fun func(bt *Button)) *Button {
	if m == nil {
		*m = make(Menu, 0, 10)
	}
	bt := &Button{}
	m.SetButton(bt, opts, fun)
	*m = append(*m, bt.This().(Widget))
	return bt
}

// InsertButtonBefore adds an button to the menu before existing item of given
// name, using given options, and connects the button signal to given receiver
// object and function, along with given data which is stored on the button
// and then passed in the button signal.  Optional updateFunc is a function
// called prior to showing the menu to update the buttons (enabled or not
// typically).  If name not found, adds to end of list..
func (m *Menu) InsertButtonBefore(before string, opts ActOpts, fun func(bt *Button)) *Button {
	sl := (*[]ki.Ki)(m)
	if idx, got := ki.SliceIndexByName(sl, before, 0); got {
		bt := &Button{}
		m.SetButton(bt, opts, fun)
		ki.SliceInsert(sl, bt.This(), idx)
		return bt
	} else {
		return m.AddButton(opts, fun)
	}
}

// InsertButtonAfter adds an button to the menu after existing item of given
// name, using given options, and connects the button signal to given receiver
// object and function, along with given data which is stored on the button
// and then passed in the button signal.  Optional updateFunc is a function
// called prior to showing the menu to update the buttons (enabled or not
// typically).  If name not found, adds to end of list..
func (m *Menu) InsertButtonAfter(after string, opts ActOpts, fun func(bt *Button)) *Button {
	sl := (*[]ki.Ki)(m)
	if idx, got := ki.SliceIndexByName(sl, after, 0); got {
		bt := &Button{}
		m.SetButton(bt, opts, fun)
		ki.SliceInsert(sl, bt.This(), idx+1)
		return bt
	} else {
		return m.AddButton(opts, fun)
	}
}

// AddSeparator adds a separator at the next point in the menu (name is just
// internal label of element, defaults to 'sep' if empty)
func (m *Menu) AddSeparator(name string) *Separator {
	if m == nil {
		*m = make(Menu, 0, 10)
	}
	sp := &Separator{}
	if name == "" {
		name = "sep"
	}
	sp.InitName(sp, name)
	sp.Horiz = true
	*m = append(*m, sp.This().(Widget))
	return sp
}

// AddLabel adds a label to the menu
func (m *Menu) AddLabel(lbl string) *Label {
	if m == nil {
		*m = make(Menu, 0, 10)
	}
	lb := &Label{}
	lb.InitName(lb, lbl)
	lb.SetText(lbl)
	*m = append(*m, lb.This().(Widget))
	return lb
}

// SetShortcuts sets the shortcuts to given window -- call when the menu has
// been attached to a window
func (m *Menu) SetShortcuts(em *EventMgr) {
	if em == nil {
		return
	}
	for _, mi := range *m {
		if bt := AsButton(mi); bt != nil {
			em.AddShortcut(bt.Shortcut, bt)
		}
	}
}

// DeleteShortcuts deletes the shortcuts in given window
func (m *Menu) DeleteShortcuts(em *EventMgr) {
	if em == nil {
		return
	}
	for _, mi := range *m {
		if bt := AsButton(mi); bt != nil {
			em.DeleteShortcut(bt.Shortcut, bt)
		}
	}
}

// UpdateButtons calls update function on all the buttons in the menu, and any
// of their sub-buttons
func (m *Menu) UpdateButtons() {
	for _, mi := range *m {
		if bt := AsButton(mi); bt != nil {
			bt.UpdateButtons()
		}
	}
}

// FindButtonByName finds an button on the menu, or any sub-menu, with given
// name (exact match) -- this is not the Text label but the Name of the
// element (for AddButton items, this is the same as Label or Icon (if Label
// is empty)) -- returns false if not found
func (m *Menu) FindButtonByName(name string) (*Button, bool) {
	for _, mi := range *m {
		if bt := AsButton(mi); bt != nil {
			if bt.Name() == name {
				return bt, true
			}
			if bt.Menu != nil {
				if sbt, ok := bt.Menu.FindButtonByName(name); ok {
					return sbt, ok
				}
			}
		}
	}
	return nil, false
}

///////////////////////////////////////////////////////////////
// Standard menu elements

/*

// AddCopyCutPaste adds a Copy, Cut, Paste buttons that just emit the
// corresponding keyboard shortcut.  Paste is automatically enabled by
// clipboard having something in it.
func (m *Menu) AddCopyCutPaste(win *RenderWin) {
	m.AddButton(ActOpts{Label: "Copy", ShortcutKey: KeyFunCopy},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			win.EventMgr.SendKeyFunEvent(KeyFunCopy, false) // false = ignore popups -- don't send to menu
		})
	m.AddButton(ActOpts{Label: "Cut", ShortcutKey: KeyFunCut},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			win.EventMgr.SendKeyFunEvent(KeyFunCut, false) // false = ignore popups -- don't send to menu
		})
	m.AddButton(ActOpts{Label: "Paste", ShortcutKey: KeyFunPaste,
		UpdateFunc: func(bt *Button) {
			bt.SetEnabledState(!goosi.TheApp.ClipBoard(win.RenderWin).IsEmpty())
		}}, nil, func(recv, send ki.Ki, sig int64, data any) {
		win.EventMgr.SendKeyFunEvent(KeyFunPaste, false) // false = ignore popups -- don't send to menu
	})
}

// AddCopyCutPasteDupe adds a Copy, Cut, Paste, and Duplicate buttons that
// just emit the corresponding keyboard shortcut.  Paste is automatically
// enabled by clipboard having something in it.
func (m *Menu) AddCopyCutPasteDupe(win *RenderWin) {
	m.AddCopyCutPaste(win)
	dpsc := ActiveKeyMap.ChordForFun(KeyFunDuplicate)
	m.AddButton(ActOpts{Label: "Duplicate", Shortcut: dpsc},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			win.EventMgr.SendKeyFunEvent(KeyFunDuplicate, false) // false = ignore popups -- don't send to menu
		})
}


// CustomAppMenuFunc is a function called by AddAppMenu after the
// AddStdAppMenu is called -- apps can set this function to add / modify / etc
// the menu
var CustomAppMenuFunc = (func(m *Menu, win *RenderWin))(nil)

// AddAppMenu adds an "app" menu to the menu -- calls AddStdAppMenu and then
// CustomAppMenuFunc if non-nil
func (m *Menu) AddAppMenu(win *RenderWin) {
	m.AddStdAppMenu(win)
	if CustomAppMenuFunc != nil {
		CustomAppMenuFunc(m, win)
	}
}
*/

/*
// AddStdAppMenu adds a standard set of menu items for application-level control.
func (m *Menu) AddStdAppMenu(win *RenderWin) {
	aboutitle := "About " + goosi.TheApp.Name()
	m.AddButton(ActOpts{Label: aboutitle},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			PromptDialog(win.Scene, DlgOpts{Title: aboutitle, Prompt: goosi.TheApp.About()}, AddOk, NoCancel, nil, nil)
		})
	m.AddButton(ActOpts{Label: "GoGi Preferences...", Shortcut: "Command+P"},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			TheViewIFace.PrefsView(&Prefs)
		})
	m.AddSeparator("sepq")
	m.AddButton(ActOpts{Label: "Quit", Shortcut: "Command+Q"},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			goosi.TheApp.QuitReq()
		})
}
*/

// AddRenderWinsMenu adds menu items for current main and dialog windows.
// must be called under RenderWinGlobalMu mutex lock!
func (m *Menu) AddRenderWinsMenu(win *RenderWin) {
	/*  todo
	m.AddButton(ActOpts{Label: "Minimize"},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			win.GoosiWin.Minimize()
		})
	m.AddButton(ActOpts{Label: "Focus Next", ShortcutKey: KeyFunWinFocusNext},
		nil, func(recv, send ki.Ki, sig int64, data any) {
			AllRenderWins.FocusNext()
		})
	m.AddSeparator("sepa")
	for _, w := range MainRenderWins {
		if w != nil {
			m.AddButton(ActOpts{Label: w.Title},
				nil, func(recv, send ki.Ki, sig int64, data any) {
					w.GoosiWin.Raise()
				})
		}
	}
	if len(DialogRenderWins) > 0 {
		m.AddSeparator("sepw")
		for _, w := range DialogRenderWins {
			if w != nil {
				m.AddButton(ActOpts{Label: w.Title},
					nil, func(recv, send ki.Ki, sig int64, data any) {
						w.GoosiWin.Raise()
					})
			}
		}
	}
	*/
}

///////////////////////////////////////////////////////////////////
// PopupMenu function

// MenuSceneConfigStyles configures the default styles
// for the given pop-up menu frame with the given parent.
// It should be called on menu frames when they are created.
func MenuSceneConfigStyles(msc *Scene) {
	msc.AddStyles(func(s *styles.Style) {
		s.Border.Style.Set(styles.BorderNone)
		s.Border.Radius = styles.BorderRadiusExtraSmall
		s.BackgroundColor.SetSolid(colors.Scheme.SurfaceContainer)
		s.BoxShadow = styles.BoxShadow2()
	})
}

// MenuMaxHeight is the maximum height of any menu popup panel in units of font height
// scroll bars are enforced beyond that size.
var MenuMaxHeight = 30

// NewMenuScene constructs a Scene for displaying the given Menu.
func NewMenuScene(menu Menu, name string) *Scene {
	msc := StageScene(name + "-menu")
	MenuSceneConfigStyles(msc)
	hasSelected := false
	for _, ac := range menu {
		wi, wb := AsWidget(ac)
		if wi == nil {
			continue
		}
		cl := wi.Clone().This().(Widget)
		cb := cl.AsWidget()
		if bt, ok := cl.(*Button); ok {
			bt.Type = ButtonMenu
			if bt.Menu == nil {
				cb.Listeners[events.Click] = wb.Listeners[events.Click]
				bt.HandleClickDismissMenu()
			}
		}
		cb.Sc = msc
		msc.AddChild(cl)
		if !hasSelected && cb.StateIs(states.Selected) {
			msc.EventMgr.SetStartFocus(cl)
			hasSelected = true
		}
	}
	if !hasSelected && msc.HasChildren() {
		msc.EventMgr.SetStartFocus(msc.Child(0).(Widget))
	}
	return msc
}

// NewMenuFromScene returns a new Menu stage with given scene contents,
// in connection with given widget, which provides key context
// for constructing the menu, at given RenderWin position
// (e.g., use ContextMenuPos or WinPos method on ctx Widget).
// Typically use NewMenu which takes a standard [Menu].
// Make further configuration choices using Set* methods, which
// can be chained directly after the New call.
// Use Run call at the end to start the Stage running.
func NewMenuFromScene(sc *Scene, ctx Widget, pos image.Point) *PopupStage {
	sc.Geom.Pos = pos
	return NewPopupStage(MenuStage, sc, ctx)
}

// NewMenu returns a new Menu stage with given scene contents,
// in connection with given widget, which provides key context
// for constructing the menu at given RenderWin position
// (e.g., use ContextMenuPos or WinPos method on ctx Widget).
// The menu is specified in terms of a [Menu].
// Make further configuration choices using Set* methods, which
// can be chained directly after the New call.
// Use Run call at the end to start the Stage running.
func NewMenu(menu Menu, ctx Widget, pos image.Point) *PopupStage {
	return NewMenuFromScene(NewMenuScene(menu, ctx.Name()), ctx, pos)
}

///////////////////////////////////////////////////////////////
// 	Context Menu

// CtxtMenuFunc is a function for creating a context menu for given node
type CtxtMenuFunc func(g Widget, m *Menu)

func (wb *WidgetBase) MakeContextMenu(m *Menu) {
	// derived types put native menu code here
	if wb.CtxtMenuFunc != nil {
		wb.CtxtMenuFunc(wb.This().(Widget), m)
	}
	mvp := wb.Sc
	TheViewIFace.CtxtMenuView(wb.This(), wb.IsDisabled(), mvp, m)
}

func (wb *WidgetBase) ContextMenuPos() image.Point {
	return wb.WinPos(.5, .5) // center
}

func (wb *WidgetBase) ContextMenu() {
	var menu Menu
	wi := wb.This().(Widget)
	wi.MakeContextMenu(&menu)
	if len(menu) == 0 {
		return
	}
	NewMenu(menu, wi, wi.ContextMenuPos()).Run()
}

///////////////////////////////////////////////////////////////
// 	Choosers

// StringsChooserPopup creates a menu of the strings in the given string
// slice, and calls the given function on receiver when the user selects.
// This is an event coming from the Button for the given menu
// item; the name of the Button is the string value, and the data will be
// the index in the slice.  A string equal to curSel will be marked as
// selected. ctx Widget provides position etc for the menu.
func StringsChooserPopup(strs []string, curSel string, ctx Widget, fun func(bt *Button)) {
	var menu Menu
	for i, it := range strs {
		bt := menu.AddButton(ActOpts{Label: it, Data: i}, fun)
		bt.SetSelected(it == curSel)
	}
	NewMenu(menu, ctx, ctx.ContextMenuPos()).Run()
}

// SubStringsChooserPopup creates a menu of the sub-strings in the given
// slice of string slices, and calls the given function on receiver when
// the user selects.  This is an event coming from the Button for the given
// menu item. The sub-menu name is the first element of each sub-slice.
// The name of the Button is the string value, and the data is an
// []int{s,i} slice of submenu and item indexes.
// A string of subMenu: item equal to curSel will be marked as selected.
// Location is from the ContextMenuPos of recv node.
func SubStringsChooserPopup(strs [][]string, curSel string, ctx Widget, fun func(bt *Button)) *Scene {
	var menu Menu
	for si, ss := range strs {
		sz := len(ss)
		if sz < 2 {
			continue
		}
		s1 := ss[0]
		sm := menu.AddButton(ActOpts{Label: s1}, nil)
		sm.Type = ButtonMenu
		for i := 1; i < sz; i++ {
			it := ss[i]
			cnm := s1 + ": " + it
			bt := sm.Menu.AddButton(ActOpts{Label: it, Data: []int{si, i}}, fun)
			bt.SetSelected(cnm == curSel)
		}
	}
	// wb := ctx.AsWidget()
	// pos := recv.ContextMenuPos()
	// sc := wb.Sc
	// return PopupMenu(menu, pos.X, pos.Y, sc, recv.Name())
	return nil
}

// StringsInsertFirst inserts the given string at start of a string slice,
// while keeping overall length to given max value
// useful for a "recents" kind of string list
func StringsInsertFirst(strs *[]string, str string, max int) {
	if strs == nil {
		*strs = make([]string, 0, max)
	}
	sz := len(*strs)
	if sz > max {
		*strs = (*strs)[:max]
	}
	if sz >= max {
		copy((*strs)[1:max], (*strs)[0:max-1])
		(*strs)[0] = str
	} else {
		*strs = append(*strs, "")
		if sz > 0 {
			copy((*strs)[1:], (*strs)[0:sz])
		}
		(*strs)[0] = str
	}
}

// StringsInsertFirstUnique inserts the given string at start of a string slice,
// while keeping overall length to given max value.
// if item is already on the list, then it is moved to the top and not re-added (unique items only)
// useful for a "recents" kind of string list
func StringsInsertFirstUnique(strs *[]string, str string, max int) {
	if strs == nil {
		*strs = make([]string, 0, max)
	}
	sz := len(*strs)
	if sz > max {
		*strs = (*strs)[:max]
	}
	for i, s := range *strs {
		if s == str {
			if i == 0 {
				return
			}
			copy((*strs)[1:i+1], (*strs)[0:i])
			(*strs)[0] = str
			return
		}
	}
	if sz >= max {
		copy((*strs)[1:max], (*strs)[0:max-1])
		(*strs)[0] = str
	} else {
		*strs = append(*strs, "")
		if sz > 0 {
			copy((*strs)[1:], (*strs)[0:sz])
		}
		(*strs)[0] = str
	}
}

// StringsDelete deletes item from strings list
func StringsDelete(strs *[]string, str string) {
	for i, s := range *strs {
		if s == str {
			*strs = append((*strs)[:i], (*strs)[i+1:]...)
			return
		}
	}
}

// StringsAppendIfUnique append str to strs if not already in slice
func StringsAppendIfUnique(strs *[]string, str string, max int) {
	if strs == nil {
		*strs = make([]string, 0, max)
	}
	for _, s := range *strs {
		if s == str {
			return
		}
	}
	*strs = append(*strs, str)
}

// StringsAddExtras is a generic function for appending a slice to a slice used to add items to menus
func StringsAddExtras(items *[]string, extras []string) {
	*items = append(*items, extras...)
}

// StringsRemoveExtras is a generic function for removing items of a slice from another slice
func StringsRemoveExtras(items *[]string, extras []string) {
	for _, extra := range extras {
		i := 0
		for _, item := range *items {
			if item != extra {
				(*items)[i] = item
				i++
			}
		}
		*items = (*items)[:i]
	}
}

////////////////////////////////////////////////////////////////////////////////////////
// Separator

// Separator defines a string to indicate a menu separator item
var MenuTextSeparator = "-------------"

// Separator draws a vertical or horizontal line
type Separator struct {
	WidgetBase

	// is this a horizontal separator -- otherwise vertical
	Horiz bool `xml:"horiz" desc:"is this a horizontal separator -- otherwise vertical"`
}

func (sp *Separator) OnInit() {
	// TODO: fix disappearing separator in menu
	sp.AddStyles(func(s *styles.Style) {
		s.Margin.Set()
		s.Padding.Set(units.Dp(4), units.Dp(0))
		s.AlignV = styles.AlignCenter
		s.AlignH = styles.AlignCenter
		s.Border.Style.Top = styles.BorderSolid
		s.Border.Color.Top = colors.Scheme.OutlineVariant
		s.Border.Width.Top.SetDp(1)
		if sp.Horiz {
			s.MaxWidth.SetDp(-1)
			s.MinHeight.SetDp(1)
		} else {
			s.MaxHeight.SetDp(-1)
			s.MinWidth.SetDp(1)
		}
	})
}

func (sp *Separator) CopyFieldsFrom(frm any) {
	fr := frm.(*Separator)
	sp.WidgetBase.CopyFieldsFrom(&fr.WidgetBase)
	sp.Horiz = fr.Horiz
}

func (sp *Separator) RenderSeparator(sc *Scene) {
	rs, pc, st := sp.RenderLock(sc)
	defer sp.RenderUnlock(rs)

	pos := sp.LayState.Alloc.Pos.Add(st.TotalMargin().Pos())
	sz := sp.LayState.Alloc.Size.Sub(st.TotalMargin().Size())

	if !st.BackgroundColor.IsNil() {
		pc.FillBox(rs, pos, sz, &st.BackgroundColor)
	}
	// border-top is standard property for separators in CSS (see https://www.w3schools.com/howto/howto_css_dividers.asp)
	pc.StrokeStyle.Width = st.Border.Width.Top
	pc.StrokeStyle.SetColor(&st.Border.Color.Top)
	if sp.Horiz {
		pc.DrawLine(rs, pos.X, pos.Y+0.5*sz.Y, pos.X+sz.X, pos.Y+0.5*sz.Y)
	} else {
		pc.DrawLine(rs, pos.X+0.5*sz.X, pos.Y, pos.X+0.5*sz.X, pos.Y+sz.Y)
	}
	pc.FillStrokeClear(rs)
}

func (sp *Separator) Render(sc *Scene) {
	if sp.PushBounds(sc) {
		sp.RenderSeparator(sc)
		sp.RenderChildren(sc)
		sp.PopBounds(sc)
	}
}
