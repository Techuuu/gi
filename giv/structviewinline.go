// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"reflect"
	"strings"

	"goki.dev/gi/v2/gi"
	"goki.dev/girl/styles"
	"goki.dev/goosi/events"
	"goki.dev/ki/v2"
	"goki.dev/laser"
)

// StructViewInline represents a struct as a single line widget, for smaller
// structs and those explicitly marked inline in the kit type registry type
// properties -- constructs widgets in Parts to show the field names and
// editor fields for each field
type StructViewInline struct {
	gi.WidgetBase

	// the struct that we are a view onto
	Struct any `desc:"the struct that we are a view onto"`

	// ValueView for the struct itself, if this was created within value view framework -- otherwise nil
	StructValView ValueView `desc:"ValueView for the struct itself, if this was created within value view framework -- otherwise nil"`

	// if true add an edit action button at the end -- other users of this widget can then configure that -- it is called 'edit-action'
	AddButton bool `desc:"if true add an edit action button at the end -- other users of this widget can then configure that -- it is called 'edit-action'"`

	// ValueView representations of the fields
	FieldViews []ValueView `json:"-" xml:"-" desc:"ValueView representations of the fields"`

	// value view that needs to have SaveTmp called on it whenever a change is made to one of the underlying values -- pass this down to any sub-views created from a parent
	TmpSave ValueView `json:"-" xml:"-" desc:"value view that needs to have SaveTmp called on it whenever a change is made to one of the underlying values -- pass this down to any sub-views created from a parent"`

	// a record of parent View names that have led up to this view -- displayed as extra contextual information in view dialog windows
	ViewPath string `desc:"a record of parent View names that have led up to this view -- displayed as extra contextual information in view dialog windows"`

	// [view: inactive] if true, some fields have default values -- update labels when values change
	HasDefs bool `json:"-" xml:"-" view:"inactive" desc:"if true, some fields have default values -- update labels when values change"`

	// if true, some fields have viewif conditional view tags -- update after..
	HasViewIfs bool `json:"-" xml:"-" inactive:"+" desc:"if true, some fields have viewif conditional view tags -- update after.."`
}

func (sv *StructViewInline) OnChildAdded(child ki.Ki) {
	w, _ := gi.AsWidget(child)
	if w.Parent().Name() == "Parts" && strings.HasPrefix(w.Name(), "label-") {
		w.AddStyles(func(s *styles.Style) {
			s.AlignH = styles.AlignLeft
		})
	}
}

// SetStruct sets the source struct that we are viewing -- rebuilds the
// children to represent this struct
func (sv *StructViewInline) SetStruct(st any) {
	updt := false
	if sv.Struct != st {
		updt = sv.UpdateStart()
		sv.Struct = st
		// if k, ok := st.(ki.Ki); ok {
		// 	k.NodeSignal().Connect(sv.This(), func(recv, send ki.Ki, sig int64, data any) {
		// 		svv, _ := recv.Embed(TypeStructViewInline).(*StructViewInline)
		// 		svv.UpdateFields() // this never gets called, per below!
		// 		// fmt.Printf("struct view inline ki update values\n")
		// 		svv.ViewSig.Emit(svv.This(), 0, k)
		// 	})
		// }
	}
	sv.ConfigParts(sv.Sc)
	sv.UpdateEnd(updt)
}

// ConfigParts configures Parts for the current struct
func (sv *StructViewInline) ConfigParts(sc *gi.Scene) {
	if laser.AnyIsNil(sv.Struct) {
		return
	}
	parts := sv.NewParts(gi.LayoutHoriz)
	config := ki.Config{}
	// always start fresh!
	sv.FieldViews = make([]ValueView, 0)
	laser.FlatFieldsValueFunc(sv.Struct, func(fval any, typ reflect.Type, field reflect.StructField, fieldVal reflect.Value) bool {
		// todo: check tags, skip various etc
		vwtag := field.Tag.Get("view")
		if vwtag == "-" {
			return true
		}
		viewif := field.Tag.Get("viewif")
		if viewif != "" {
			sv.HasViewIfs = true
			if !StructViewIf(viewif, field, sv.Struct) {
				return true
			}
		}
		vv := FieldToValueView(sv.Struct, field.Name, fval)
		if vv == nil { // shouldn't happen
			return true
		}
		vvp := fieldVal.Addr()
		vv.SetStructValue(vvp, sv.Struct, &field, sv.TmpSave, sv.ViewPath)
		vtyp := vv.WidgetType()
		// todo: other things with view tag..
		labnm := "label-" + field.Name
		valnm := "value-" + field.Name
		config.Add(gi.LabelType, labnm)
		config.Add(vtyp, valnm) // todo: extend to diff types using interface..
		sv.FieldViews = append(sv.FieldViews, vv)
		return true
	})
	if sv.AddButton {
		config.Add(gi.ButtonType, "edit-action")
	}
	_, updt := parts.ConfigChildren(config)
	sv.HasDefs = false
	for i, vv := range sv.FieldViews {
		lbl := parts.Child(i * 2).(*gi.Label)
		vvb := vv.AsValueViewBase()
		vvb.ViewPath = sv.ViewPath
		widg := parts.Child((i * 2) + 1).(gi.Widget)
		hasDef, inactTag := StructViewFieldTags(vv, lbl, widg, sv.IsDisabled()) // in structview.go
		if hasDef {
			sv.HasDefs = true
		}
		vv.ConfigWidget(widg)
		if !sv.IsDisabled() && !inactTag {
			vvb.OnChange(func(e events.Event) {
				sv.UpdateFieldAction()
				sv.SendChange()
			})
		}
	}
	parts.UpdateEnd(updt)
	sv.UpdateEndLayout(updt)
}

func (sv *StructViewInline) UpdateFields() {
	updt := sv.UpdateStart()
	for _, vv := range sv.FieldViews {
		vv.UpdateWidget()
	}
	sv.UpdateEndRender(updt)
}

func (sv *StructViewInline) UpdateFieldAction() {
	if sv.HasViewIfs {
		sv.ConfigParts(sv.Sc)
	} else if sv.HasDefs {
		updt := sv.UpdateStart()
		for i, vv := range sv.FieldViews {
			lbl := sv.Parts.Child(i * 2).(*gi.Label)
			StructViewFieldDefTag(vv, lbl)
		}
		sv.UpdateEndRender(updt)
	}
}
