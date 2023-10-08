// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"goki.dev/colors"
	"goki.dev/cursors"
	"goki.dev/gi/v2/gi"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/girl/units"
	"goki.dev/goosi"
	"goki.dev/goosi/events"
	"goki.dev/goosi/events/key"
	"goki.dev/goosi/mimedata"
	"goki.dev/gti"
	"goki.dev/ki/v2"
	"goki.dev/laser"
)

////////////////////////////////////////////////////////////////////////////////////////
//  KeyChordValueView

// KeyChordValueView presents an KeyChordEdit for key.Chord
type KeyChordValueView struct {
	ValueViewBase
}

func (vv *KeyChordValueView) WidgetType() *gti.Type {
	vv.WidgetTyp = KeyChordEditType
	return vv.WidgetTyp
}

func (vv *KeyChordValueView) UpdateWidget() {
	if vv.Widget == nil {
		return
	}
	kc := vv.Widget.(*KeyChordEdit)
	txt := laser.ToString(vv.Value.Interface())
	kc.SetText(txt)
}

func (vv *KeyChordValueView) ConfigWidget(widg gi.Widget) {
	vv.Widget = widg
	vv.StdConfigWidget(widg)
	kc := vv.Widget.(*KeyChordEdit)
	kc.KeyChordSig.ConnectOnly(vv.This(), func(recv, send ki.Ki, sig int64, data any) {
		vvv, _ := recv.Embed(TypeKeyChordValueView).(*KeyChordValueView)
		kcc := vvv.Widget.(*KeyChordEdit)
		if vvv.SetValue(key.Chord(kcc.Text)) {
			vvv.UpdateWidget()
		}
		vvv.ViewSig.Emit(vvv.This(), 0, nil)
	})
	vv.UpdateWidget()
}

func (vv *KeyChordValueView) HasAction() bool {
	return false
}

/////////////////////////////////////////////////////////////////////////////////
// KeyChordEdit

// KeyChordEdit is a label widget that shows a key chord string, and, when in
// focus (after being clicked) will update to whatever key chord is typed --
// used for representing and editing key chords.
type KeyChordEdit struct {
	gi.Label

	// true if the keyboard focus is active or not -- when we lose active focus we apply changes
	FocusActive bool `json:"-" xml:"-" desc:"true if the keyboard focus is active or not -- when we lose active focus we apply changes"`

	// [view: -] signal -- only one event, when chord is updated from key input
	// KeyChordSig ki.Signal `json:"-" xml:"-" view:"-" desc:"signal -- only one event, when chord is updated from key input"`
}

func (kc *KeyChordEdit) OnInit() {
	kc.AddStyles(func(s *styles.Style) {
		s.Cursor = cursors.Pointer
		s.AlignV = styles.AlignTop
		s.Border.Style.Set(styles.BorderNone)
		s.Border.Radius = styles.BorderRadiusFull
		s.Width.SetCh(20)
		s.Padding.Set(units.Dp(8 * gi.Prefs.DensityMul()))
		s.SetStretchMaxWidth()
		if w.StateIs(states.Selected) {
			s.BackgroundColor.SetSolid(colors.Scheme.TertiaryContainer)
			s.Color = colors.Scheme.OnTertiaryContainer
		} else {
			// STYTODO: get state styles working
			s.BackgroundColor.SetSolid(colors.Scheme.SecondaryContainer)
			s.Color = colors.Scheme.OnSecondaryContainer
		}
	})
}

// ChordUpdated emits KeyChordSig when a new chord has been entered
func (kc *KeyChordEdit) ChordUpdated() {
	kc.KeyChordSig.Emit(kc.This(), 0, kc.Text)
}

func (kc *KeyChordEdit) MakeContextMenu(m *gi.MenuActions) {
	m.AddAction(gi.ActOpts{Label: "Clear"},
		kc, func(recv, send ki.Ki, sig int64, data any) {
			kcc := recv.Embed(TypeKeyChordEdit).(*KeyChordEdit)
			kcc.SetText("")
			kcc.ChordUpdated()
		})
}

func (kc *KeyChordEdit) MouseEvent() {
	kcwe.AddFunc(events.MouseUp, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		me := d.(events.Event)
		kcc := recv.Embed(TypeKeyChordEdit).(*KeyChordEdit)
		if me.Action == events.Press && me.Button == events.Left {
			if kcc.Selectable {
				me.SetHandled()
				kcc.SetSelected(!kcc.StateIs(states.Selected))
				if kcc.StateIs(states.Selected) {
					kcc.GrabFocus()
				}
				kcc.EmitSelectedSignal()
				kcc.UpdateSig()
			}
		}
		if me.Action == events.Release && me.Button == events.Right {
			me.SetHandled()
			kcc.EmitContextMenuSignal()
			kcc.This().(gi.Widget).ContextMenu()
		}
	})
}

func (kc *KeyChordEdit) KeyChordEvent() {
	kcwe.AddFunc(events.KeyChord, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		kcc := recv.Embed(TypeKeyChordEdit).(*KeyChordEdit)
		if kcc.StateIs(states.Focused) && kcc.FocusActive {
			kt := d.(*events.Key)
			kt.SetHandled()
			kcc.SetText(string(kt.KeyChord())) // that's easy!
			goosi.TheApp.ClipBoard(kc.ParentRenderWin().RenderWin).Write(mimedata.NewText(string(kt.KeyChord())))
			kcc.ChordUpdated()
		}
	})
}

func (kc *KeyChordEdit) ApplyStyle(sc *gi.Scene) {
	kc.SetCanFocusIfActive()
	kc.Selectable = true
	kc.Redrawable = true
	kc.StyleLabel()
	kc.LayoutLabel()
}

func (kc *KeyChordEdit) SetTypeHandlers() {
	kc.HoverEvent()
	kc.MouseEvent()
	kc.KeyChordEvent()
}

// func (kc *KeyChordEdit) FocusChanged(change gi.FocusChanges) {
// 	switch change {
// 	case gi.FocusLost:
// 		kc.FocusActive = false
// 		kc.ClearSelected()
// 		kc.ChordUpdated()
// 		kc.UpdateSig()
// 	case gi.FocusGot:
// 		kc.FocusActive = true
// 		kc.SetSelected(true)
// 		kc.ScrollToMe()
// 		kc.EmitFocusedSignal()
// 		kc.UpdateSig()
// 	case gi.FocusInactive:
// 		kc.FocusActive = false
// 		kc.ClearSelected()
// 		kc.ChordUpdated()
// 		kc.UpdateSig()
// 	case gi.FocusActive:
// 		// we don't re-activate on keypress here, so that you don't end up stuck
// 		// on a given keychord
// 		// kc.SetSelected()
// 		// kc.FocusActive = true
// 		// kc.ScrollToMe()
// 	}
// }
