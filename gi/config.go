// Copyright (c) 2023, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"goki.dev/gicons"
	"goki.dev/ki/v2"
)

func (wb *WidgetBase) Config(vp *Viewport) {
	if wb.This() == nil {
		return
	}
	wi := wb.This().(Widget)
	updt := wi.UpdateStart()
	wb.Style.Defaults()    // reset
	wb.LayState.Defaults() // doesn't overwrite
	wi.ConfigWidget(vp)    // where everything actually happens
	wi.SetStyle(vp)
	wi.UpdateEnd(updt)
	wb.SetNeedsLayout(vp, updt)
}

// ConfigPartsIconLabel adds to config to create parts, of icon
// and label left-to right in a row, based on whether items are nil or empty
func (wb *WidgetBase) ConfigPartsIconLabel(config *ki.TypeAndNameList, icnm gicons.Icon, txt string) (icIdx, lbIdx int) {
	if wb.Style.Template != "" {
		wb.Parts.Style.Template = wb.Style.Template + ".Parts"
	}
	icIdx = -1
	lbIdx = -1
	if TheIconMgr.IsValid(icnm) {
		icIdx = len(*config)
		config.Add(TypeIcon, "icon")
		if txt != "" {
			config.Add(TypeSpace, "space")
		}
	}
	if txt != "" {
		lbIdx = len(*config)
		config.Add(TypeLabel, "label")
	}
	return
}

// ConfigPartsSetIconLabel sets the icon and text values in parts, and get
// part style props, using given props if not set in object props
func (wb *WidgetBase) ConfigPartsSetIconLabel(icnm gicons.Icon, txt string, icIdx, lbIdx int) {
	if icIdx >= 0 {
		ic := wb.Parts.Child(icIdx).(*Icon)
		if wb.Style.Template != "" {
			ic.Style.Template = wb.Style.Template + ".icon"
		}
		ic.SetIcon(icnm)
	}
	if lbIdx >= 0 {
		lbl := wb.Parts.Child(lbIdx).(*Label)
		if wb.Style.Template != "" {
			lbl.Style.Template = wb.Style.Template + ".icon"
		}
		if lbl.Text != txt {
			// avoiding SetText here makes it so label default
			// styles don't end up first, which is needed for
			// parent styles to override. However, there might have
			// been a reason for calling SetText, so we will see if
			// any bugs show up. TODO: figure out a good long-term solution for this.
			lbl.Text = txt
			// lbl.SetText(txt)
		}
	}
}

// PartsNeedUpdateIconLabel check if parts need to be updated -- for ConfigPartsIfNeeded
func (wb *WidgetBase) PartsNeedUpdateIconLabel(icnm gicons.Icon, txt string) bool {
	if TheIconMgr.IsValid(icnm) {
		ick := wb.Parts.ChildByName("icon", 0)
		if ick == nil {
			return true
		}
		ic := ick.(*Icon)
		if !ic.HasChildren() || ic.IconNm != icnm || wb.NeedsFullReRender() {
			return true
		}
	} else {
		cn := wb.Parts.ChildByName("icon", 0)
		if cn != nil { // need to remove it
			return true
		}
	}
	if txt != "" {
		lblk := wb.Parts.ChildByName("label", 2)
		if lblk == nil {
			return true
		}
		lbl := lblk.(*Label)
		lbl.Style.Color = wb.Style.Color
		if lbl.Text != txt {
			return true
		}
	} else {
		cn := wb.Parts.ChildByName("label", 2)
		if cn != nil {
			return true
		}
	}
	return false
}

// SetFullReRenderIconLabel sets the icon and label to be re-rendered, needed
// when styles change
func (wb *WidgetBase) SetFullReRenderIconLabel() {
	if ick := wb.Parts.ChildByName("icon", 0); ick != nil {
		ic := ick.(*Icon)
		ic.SetFullReRender()
	}
	if lblk := wb.Parts.ChildByName("label", 2); lblk != nil {
		lbl := lblk.(*Label)
		lbl.SetFullReRender()
	}
	wb.Parts.StyMu.Lock()
	wb.Parts.SetStyleWidget() // restyle parent so parts inherit
	wb.Parts.StyMu.Unlock()
}
