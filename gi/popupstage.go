// Copyright (c) 2023, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"log/slog"

	"goki.dev/goosi/events"
	"goki.dev/ki/v2"
)

// PopupStage supports Popup types (Menu, Tooltip, Snackbar, Chooser),
// which are transitory and simple, without additional decor,
// and are associated with and managed by a MainStage element (Window, etc).
type PopupStage struct {
	StageBase

	// Main is the MainStage that owns this Popup (via its PopupMgr)
	Main *MainStage
}

// AsPopup returns this stage as a PopupStage (for Popup types)
// returns nil for MainStage types.
func (st *PopupStage) AsPopup() *PopupStage {
	return st
}

func (st *PopupStage) String() string {
	return "PopupStage: " + st.StageBase.String()
}

func (st *PopupStage) MainMgr() *MainStageMgr {
	if st.Main == nil {
		return nil
	}
	return st.Main.StageMgr
}

func (st *PopupStage) RenderCtx() *RenderContext {
	if st.Main == nil {
		return nil
	}
	return st.Main.RenderCtx()
}

func (st *PopupStage) Delete() {
	if st.Scene != nil {
		st.Scene.Delete(ki.DestroyKids)
	}
	st.Scene = nil
	st.Main = nil
}

func (st *PopupStage) StageAdded(smi StageMgr) {
	pm := smi.AsPopupMgr()
	st.Main = pm.Main
}

func (st *PopupStage) HandleEvent(evi events.Event) {
	if st.Scene == nil {
		return
	}
	if evi.IsHandled() {
		return
	}
	evi.SetLocalOff(st.Scene.Geom.Pos)
	// fmt.Println("pos:", evi.Pos(), "local:", evi.LocalPos())
	st.Scene.EventMgr.HandleEvent(evi)
}

// NewPopupStage returns a new PopupStage with given type and scene contents.
// Make further configuration choices using Set* methods, which
// can be chained directly after the NewPopupStage call.
// Use Run call at the end to start the Stage running.
func NewPopupStage(typ StageTypes, sc *Scene, ctx Widget) *PopupStage {
	if ctx == nil {
		slog.Error("NewPopupStage needs a context Widget")
		return nil
	}
	cwb := ctx.AsWidget()
	if cwb.Sc == nil || cwb.Sc.Stage == nil {
		slog.Error("NewPopupStage context doesn't have a Stage")
		return nil
	}
	st := &PopupStage{}
	st.This = st
	st.SetType(typ)
	st.SetScene(sc)
	st.CtxWidget = ctx
	cst := cwb.Sc.Stage
	mst := cst.AsMain()
	if mst != nil {
		st.Main = mst
	} else {
		pst := cst.AsPopup()
		st.Main = pst.Main
	}

	switch typ {
	case Menu:
		st.Modal = true
		st.ClickOff = true
		MenuSceneConfigStyles(sc)
	}

	return st
}

// NewSnackbar returns a new Snackbar stage with given scene contents,
// in connection with given widget (which provides key context).
// Make further configuration choices using Set* methods, which
// can be chained directly after the New call.
// Use an appropriate Run call at the end to start the Stage running.
func NewSnackbar(sc *Scene, ctx Widget) *PopupStage {
	return NewPopupStage(Snackbar, sc, ctx)
}

// NewChooser returns a new Chooser stage with given scene contents,
// in connection with given widget (which provides key context).
// Make further configuration choices using Set* methods, which
// can be chained directly after the New call.
// Use an appropriate Run call at the end to start the Stage running.
func NewChooser(sc *Scene, ctx Widget) *PopupStage {
	return NewPopupStage(Chooser, sc, ctx)
}

// RunPopup runs a popup-style Stage in context widget's popups.
func (st *PopupStage) RunPopup() *PopupStage {
	mm := st.MainMgr()
	if mm == nil {
		slog.Error("popupstage has no MainMgr")
		return st
	}
	mm.RenderCtx.Mu.RLock()
	defer mm.RenderCtx.Mu.RUnlock()

	ms := st.Main.Scene

	cmgr := &st.Main.PopupMgr
	cmgr.Push(st)

	sc := st.Scene
	sz := sc.PrefSize(ms.Geom.Size)
	scrollWd := int(sc.Style.ScrollBarWidth.Dots)
	fontHt := 16
	if sc.Style.Font.Face != nil {
		fontHt = int(sc.Style.Font.Face.Metrics.Height)
	}

	switch st.Type {
	case Menu:
		sz.X += scrollWd * 2
		maxht := int(MenuMaxHeight * fontHt)
		sz.Y = min(maxht, sz.Y)

	}
	sc.Geom.Size = sz
	sc.FitInWindow(ms.Geom) // does resize

	sc.EventMgr.InitialFocus()

	return st
}

// Close closes this stage as a popup
func (st *PopupStage) Close() {
	mn := st.Main
	if mn == nil {
		slog.Error("popupstage has no Main")
		return
	}
	mm := st.MainMgr()
	if mm == nil {
		slog.Error("popupstage has no MainMgr")
		return
	}
	mm.RenderCtx.Mu.RLock()
	defer mm.RenderCtx.Mu.RUnlock()

	cmgr := &mn.PopupMgr
	cmgr.PopDelete()
}
