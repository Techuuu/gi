// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"fmt"
	"reflect"
	"strconv"

	"goki.dev/gi/v2/gi"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/goosi/events"
	"goki.dev/icons"
	"goki.dev/ki/v2"
	"goki.dev/laser"
)

// SliceViewInline represents a slice as a single line widget, for smaller
// slices and those explicitly marked inline -- constructs widgets in Parts to
// show the key names and editor vals for each value.
type SliceViewInline struct {
	gi.WidgetBase

	// the slice that we are a view onto
	Slice any

	// Value for the slice itself, if this was created within value view framework -- otherwise nil
	SliceValView Value

	// whether the slice is actually an array -- no modifications
	IsArray bool

	// whether the slice has a fixed-len flag on it
	IsFixedLen bool

	// has the slice been edited?
	Changed bool

	// Value representations of the fields
	Values []Value `json:"-" xml:"-"`

	// value view that needs to have SaveTmp called on it whenever a change is made to one of the underlying values -- pass this down to any sub-views created from a parent
	TmpSave Value `json:"-" xml:"-"`

	// a record of parent View names that have led up to this view -- displayed as extra contextual information in view dialog windows
	ViewPath string
}

func (sv *SliceViewInline) OnInit() {
	sv.Style(func(s *styles.Style) {
		s.MinWidth.SetCh(20)
	})
}

func (sv *SliceViewInline) OnChildAdded(child ki.Ki) {
	w, _ := gi.AsWidget(child)
	switch w.PathFrom(sv.This()) {
	case "parts":
		parts := w.(*gi.Layout)
		parts.Lay = gi.LayoutHoriz
		w.Style(func(s *styles.Style) {
			s.Overflow = styles.OverflowHidden // no scrollbars!
		})
	}
}

// SetSlice sets the source slice that we are viewing -- rebuilds the children to represent this slice
func (sv *SliceViewInline) SetSlice(sl any) {
	if laser.AnyIsNil(sl) {
		sv.Slice = nil
		return
	}
	updt := false
	newslc := false
	if reflect.TypeOf(sl).Kind() != reflect.Pointer { // prevent crash on non-comparable
		newslc = true
	} else {
		newslc = (sv.Slice != sl)
	}
	if newslc {
		updt = sv.UpdateStart()
		sv.Slice = sl
		sv.IsArray = laser.NonPtrType(reflect.TypeOf(sl)).Kind() == reflect.Array
		sv.IsFixedLen = false
		if sv.SliceValView != nil {
			_, sv.IsFixedLen = sv.SliceValView.Tag("fixed-len")
		}
	}
	sv.UpdateFromSlice()
	sv.UpdateEndLayout(updt)
}

// ConfigParts configures Parts for the current slice
func (sv *SliceViewInline) ConfigParts(sc *gi.Scene) {
	if laser.AnyIsNil(sv.Slice) {
		return
	}
	parts := sv.NewParts(gi.LayoutHoriz)
	config := ki.Config{}
	// always start fresh!
	sv.Values = make([]Value, 0)

	mv := reflect.ValueOf(sv.Slice)
	mvnp := laser.NonPtrValue(mv)

	sz := min(mvnp.Len(), SliceInlineLen)
	for i := 0; i < sz; i++ {
		val := laser.OnePtrUnderlyingValue(mvnp.Index(i)) // deal with pointer lists
		vv := ToValue(val.Interface(), "")
		if vv == nil { // shouldn't happen
			fmt.Printf("nil value view!\n")
			continue
		}
		vv.SetSliceValue(val, sv.Slice, i, sv.TmpSave, sv.ViewPath)
		vtyp := vv.WidgetType()
		idxtxt := strconv.Itoa(i)
		valnm := "value-" + idxtxt
		config.Add(vtyp, valnm)
		sv.Values = append(sv.Values, vv)
	}
	if !sv.IsArray && !sv.IsFixedLen {
		config.Add(gi.ButtonType, "add-action")
	}
	config.Add(gi.ButtonType, "edit-action")
	mods, updt := parts.ConfigChildren(config)
	if !mods {
		updt = parts.UpdateStart()
	}
	for i, vv := range sv.Values {
		vvb := vv.AsValueBase()
		vvb.OnChange(func(e events.Event) { sv.SetChanged() })
		widg := parts.Child(i).(gi.Widget)
		if sv.SliceValView != nil {
			vv.SetTags(sv.SliceValView.AllTags())
		}
		vv.ConfigWidget(widg, sc)
		if sv.IsDisabled() {
			widg.AsWidget().SetState(true, states.Disabled)
		}
	}
	if !sv.IsArray && !sv.IsFixedLen {
		adbti, err := parts.Children().ElemFromEndTry(1)
		if err == nil {
			adbt := adbti.(*gi.Button)
			adbt.SetType(gi.ButtonTonal)
			adbt.SetIcon(icons.Add)
			adbt.Tooltip = "add an element to the slice"
			adbt.OnChange(func(e events.Event) {
				sv.SliceNewAt(-1)
			})
		}
	}
	edbti, err := parts.Children().ElemFromEndTry(0)
	if err == nil {
		edbt := edbti.(*gi.Button)
		edbt.SetType(gi.ButtonTonal)
		edbt.SetIcon(icons.Edit)
		edbt.Tooltip = "edit slice in a dialog window"
		edbt.OnClick(func(e events.Event) {
			vpath := sv.ViewPath
			title := ""
			if sv.SliceValView != nil {
				newPath := ""
				isZero := false
				title, newPath, isZero = sv.SliceValView.AsValueBase().Label()
				if isZero {
					return
				}
				vpath = sv.ViewPath + "/" + newPath
			} else {
				elType := laser.NonPtrType(reflect.TypeOf(sv.Slice).Elem().Elem())
				title = "Slice of " + laser.NonPtrType(elType).Name()
			}
			SliceViewDialog(sv, DlgOpts{Title: title, TmpSave: sv.TmpSave, ViewPath: vpath}, sv.Slice, nil, nil)
			// todo: seems very bad:
			// svvvk := dlg.Stage.Scene.ChildByType(TypeSliceView, ki.Embeds, 2)
			// if svvvk != nil {
			// 	svvv := svvvk.(*SliceView)
			// 	svvv.SliceValView = sv.SliceValView
			// 	// svvv.ViewSig.ConnectOnly(svv.This(), func(recv, send ki.Ki, sig int64, data any) {
			// 	// 	svvvv, _ := recv.Embed(TypeSliceViewInline).(*SliceViewInline)
			// 	// 	svvvv.ViewSig.Emit(svvvv.This(), 0, nil)
			// 	// })
			// }
		})
	}
	parts.UpdateEndLayout(updt)
	sv.SetNeedsLayoutUpdate(sc, updt)
}

// SetChanged sets the Changed flag and emits the ViewSig signal for the
// SliceView, indicating that some kind of edit / change has taken place to
// the table data.  It isn't really practical to record all the different
// types of changes, so this is just generic.
func (sv *SliceViewInline) SetChanged() {
	sv.Changed = true
	sv.SendChange()
}

// SliceNewAt inserts a new blank element at given index in the slice -- -1
// means the end
func (sv *SliceViewInline) SliceNewAt(idx int) {
	if sv.IsArray || sv.IsFixedLen {
		return
	}

	updt := sv.UpdateStart()
	defer sv.UpdateEndLayout(updt)

	laser.SliceNewAt(sv.Slice, idx)

	if sv.TmpSave != nil {
		sv.TmpSave.SaveTmp()
	}
	sv.SetChanged()
	sv.UpdateFromSlice()
}

func (sv *SliceViewInline) UpdateFromSlice() {
	sv.ConfigParts(sv.Sc)
}

func (sv *SliceViewInline) UpdateValues() {
	updt := sv.UpdateStart()
	for _, vv := range sv.Values {
		vv.UpdateWidget()
	}
	sv.UpdateEndRender(updt)
}

// func (sv *SliceViewInline) ApplyStyle(sc *gi.Scene) {
// 	sv.ConfigParts(sc)
// 	sv.WidgetBase.ApplyStyle(sc)
// }

func (sv *SliceViewInline) Render(sc *gi.Scene) {
	if sv.PushBounds(sc) {
		sv.RenderParts(sc)
		sv.RenderChildren(sc)
		sv.PopBounds(sc)
	}
}
