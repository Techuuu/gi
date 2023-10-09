// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"strings"

	"goki.dev/colors"
	"goki.dev/gi/v2/gi"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/girl/units"
	"goki.dev/goosi"
	"goki.dev/goosi/events"
	"goki.dev/goosi/mimedata"
	"goki.dev/icons"
	"goki.dev/ki/v2"
	"goki.dev/laser"
	"goki.dev/mat32/v2"
	"goki.dev/pi/v2/filecat"
)

////////////////////////////////////////////////////////////////////////////////////////
//  TreeView -- a widget that graphically represents / manipulates a Ki Tree

// TreeView provides a graphical representation of source tree structure
// (which can be any type of Ki nodes), providing full manipulation abilities
// of that source tree (move, cut, add, etc) through drag-n-drop and
// cut/copy/paste and menu actions.
//
// There are special style Props interpreted by these nodes:
//   - no-templates -- if present (assumed to be true) then style templates are
//     not used to optimize rendering speed.  Set this for nodes that have
//     styling applied differentially to individual nodes (e.g., FileNode).
type TreeView struct {
	gi.WidgetBase

	// Ki Node that this widget is viewing in the tree -- the source
	SrcNode ki.Ki `copy:"-" json:"-" xml:"-" desc:"Ki Node that this widget is viewing in the tree -- the source"`

	// if the object we're viewing has its own CtxtMenu property defined, should we also still show the view's own context menu?
	ShowViewCtxtMenu bool `desc:"if the object we're viewing has its own CtxtMenu property defined, should we also still show the view's own context menu?"`

	// linear index of this node within the entire tree -- updated on full rebuilds and may sometimes be off, but close enough for expected uses
	ViewIdx int `desc:"linear index of this node within the entire tree -- updated on full rebuilds and may sometimes be off, but close enough for expected uses"`

	// styled amount to indent children relative to this node
	Indent units.Value `xml:"indent" desc:"styled amount to indent children relative to this node"`

	// styled depth for nodes be initialized as open -- nodes beyond this depth will be initialized as closed.  initial default is 4.
	OpenDepth int `xml:"open-depth" desc:"styled depth for nodes be initialized as open -- nodes beyond this depth will be initialized as closed.  initial default is 4."`

	// just the size of our widget -- our alloc includes all of our children, but we only draw us
	WidgetSize mat32.Vec2 `desc:"just the size of our widget -- our alloc includes all of our children, but we only draw us"`

	// [view: show-name] optional icon, displayed to the the left of the text label
	Icon icons.Icon `json:"-" xml:"icon" view:"show-name" desc:"optional icon, displayed to the the left of the text label"`

	// cached root of the view
	RootView *TreeView `json:"-" xml:"-" desc:"cached root of the view"`
}

// We do this instead of direct setting in TypeTreeView declaration
// because TreeViewProps references TypeTreeView, causing an init cycle.
func init() {
	// kit.Types.SetProps(TypeTreeView, TreeViewProps)
}

func (tv *TreeView) OnInit() {
	tv.Indent.SetEm(1)
	tv.AddStyles(func(s *styles.Style) {
		s.Border.Style.Set(styles.BorderNone)
		s.Margin.Set()
		s.Padding.Set(units.Dp(4 * gi.Prefs.DensityMul()))
		s.Text.Align = styles.AlignLeft
		s.AlignV = styles.AlignTop
		if w.StateIs(states.Selected) {
			s.BackgroundColor.SetSolid(colors.Scheme.TertiaryContainer)
		}
	})
}

func (tv *TreeView) OnChildAdded(child ki.Ki) {
	w, _ := gi.AsWidget(child)
	switch w.Name() {
	case "Parts":
		parts := w.(*gi.Layout)
		parts.AddStyles(func(s *styles.Style) {
			parts.Spacing.SetCh(0.5)
		})
	case "icon":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(1)
			s.Height.SetEm(1)
			s.Margin.Set()
			s.Padding.Set()
		})
	case "branch":
		cb := w.(*heckBox)
		cb.Icon = icons.KeyboardArrowDown
		cb.IconOff = icons.KeyboardArrowRight
		cb.AddStyles(func(s *styles.Style) {
			s.Margin.Set()
			s.Padding.Set()
			s.MaxWidth.SetEm(1.5)
			s.MaxHeight.SetEm(1.5)
			s.AlignV = styles.AlignMiddle
		})
	case "space":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(0.5)
		})
	case "label":
		w.AddStyles(func(s *styles.Style) {
			s.Margin.Set()
			s.Padding.Set()
			s.MinWidth.SetCh(16)
		})
	case "menu":
		menu := w.(*gi.Button)
		menu.Indicator = icons.None
	}
}

//////////////////////////////////////////////////////////////////////////////
//    End-User API

// SetRootNode sets the root view to the root of the source node that we are
// viewing, and builds-out the view of its tree.
// Calls ki.UniquifyNamesAll on source tree to ensure that node names are unique
// which is essential for proper viewing!
func (tv *TreeView) SetRootNode(sk ki.Ki) {
	updt := false
	ki.UniquifyNamesAll(sk)
	if tv.SrcNode != sk {
		updt = tv.UpdateStart()
		tv.SrcNode = sk
		sk.NodeSignal().Connect(tv.This(), SrcNodeSignalFunc) // we recv signals from source
	}
	tv.RootView = tv
	tvIdx := 0
	tv.SyncToSrc(&tvIdx, true, 0)
	tv.UpdateEnd(updt)
}

// SetSrcNode sets the source node that we are viewing,
// and builds-out the view of its tree.  It is called routinely
// via SyncToSrc during tree updating.
func (tv *TreeView) SetSrcNode(sk ki.Ki, tvIdx *int, init bool, depth int) {
	updt := false
	if tv.SrcNode != sk {
		updt = tv.UpdateStart()
		tv.SrcNode = sk
		sk.NodeSignal().Connect(tv.This(), SrcNodeSignalFunc) // we recv signals from source
	}
	tv.SyncToSrc(tvIdx, init, depth)
	tv.UpdateEnd(updt)
}

// ReSync resynchronizes the view relative to the underlying nodes
// and forces a full rerender
func (tv *TreeView) ReSync() {
	tv.SetFullReRender() //
	tvIdx := tv.ViewIdx
	tv.SyncToSrc(&tvIdx, false, 0)
	tv.UpdateSig()
}

// SyncToSrc updates the view tree to match the source tree, using
// ConfigChildren to maximally preserve existing tree elements.
// init means we are doing initial build, and depth tracks depth
// (only during init).
func (tv *TreeView) SyncToSrc(tvIdx *int, init bool, depth int) {
	// pr := prof.Start("TreeView.SyncToSrc")
	// defer pr.End()
	sk := tv.SrcNode
	nm := "tv_" + sk.Name()
	tv.SetName(nm)
	tv.ViewIdx = *tvIdx
	(*tvIdx)++
	tvPar := tv.TreeViewParent()
	if tvPar != nil {
		tv.RootView = tvPar.RootView
		if init && depth >= tv.RootView.OpenDepth {
			tv.SetClosed()
		}
	}
	vcprop := "view-closed"
	skids := *sk.Children()
	tnl := make(ki.Config, 0, len(skids))
	typ := ki.Type(tv.This()) // always make our type
	flds := make([]ki.Ki, 0)
	fldClosed := make([]bool, 0)
	sk.FuncFields(0, nil, func(k ki.Ki, level int, d any) bool {
		flds = append(flds, k)
		tnl.Add(typ, "tv_"+k.Name())
		ft := ki.FieldTag(sk.This(), k.Name(), vcprop)
		cls := false
		if vc, ok := laser.ToBool(ft); ok && vc {
			cls = true
		} else {
			if vcp, ok := k.PropInherit(vcprop, ki.NoInherit, ki.TypeProps); ok {
				if vc, ok := laser.ToBool(vcp); vc && ok {
					cls = true
				}
			}
		}
		fldClosed = append(fldClosed, cls)
		return true
	})
	for _, skid := range skids {
		tnl.Add(typ, "tv_"+skid.Name())
	}
	mods, updt := tv.ConfigChildren(tnl) // false = don't use unique names -- needs to!
	if mods {
		tv.SetFullReRender()
		// fmt.Printf("got mod on %v\n", tv.Path())
	}
	idx := 0
	for i, fld := range flds {
		vk := tv.Kids[idx].Embed(TypeTreeView).(*TreeView)
		vk.SetSrcNode(fld, tvIdx, init, depth+1)
		if mods {
			vk.SetClosedState(fldClosed[i])
		}
		idx++
	}
	for _, skid := range *sk.Children() {
		if len(tv.Kids) <= idx {
			break
		}
		vk := tv.Kids[idx].Embed(TypeTreeView).(*TreeView)
		vk.SetSrcNode(skid, tvIdx, init, depth+1)
		if mods {
			if vcp, ok := skid.PropInherit(vcprop, ki.NoInherit, ki.TypeProps); ok {
				if vc, ok := laser.ToBool(vcp); vc && ok {
					vk.SetClosed()
				}
			}
		}
		idx++
	}
	if !sk.HasChildren() {
		tv.SetClosed()
	}
	tv.UpdateEnd(updt)
}

// SrcNodeSignalFunc is the function for receiving node signals from our SrcNode
func SrcNodeSignalFunc(tvki, send ki.Ki, sig int64, data any) {
	tv := tvki.Embed(TypeTreeView).(*TreeView)
	// always keep name updated in case that changed
	if data != nil {
		dflags := data.(int64)
		if gi.UpdateTrace {
			// todo: fixme
			// fmt.Printf("treeview: %v got signal: %v from node: %v  data: %v  flags %v\n", tv.Path(), ki.NodeSignals(sig), send.Path(), kit.BitFlagsToString(dflags, ki.FlagsN), kit.BitFlagsToString(send.Flags(), ki.FlagsN))
		}
		if tv.This() == tv.RootView.This() && tv.HasFlag(int(TreeViewFlagUpdtRoot)) {
			tv.SetFullReRender() // re-render for any updates on root node
		}
		// if bitflag.HasAnyMask(dflags, int64(ki.StruUpdateFlagsMask)) {
		// 	if tv.This() == tv.RootView.This() {
		// 		tv.SetFullReRender() // re-render for struct updates on root node
		// 	}
		// 	tvIdx := tv.ViewIdx
		// 	if gi.UpdateTrace {
		// 		fmt.Printf("treeview: structupdate for node, idx: %v  %v", tvIdx, tv.Path())
		// 	}
		// 	tv.SyncToSrc(&tvIdx, false, 0)
		// } else {
		tv.UpdateSig()
		// }
	}
}

// IsClosed returns whether this node itself closed?
func (tv *TreeView) IsClosed() bool {
	return tv.HasFlag(int(TreeViewFlagClosed))
}

// SetClosed sets the closed flag for this node -- call Close() method to
// close a node and update view
func (tv *TreeView) SetClosed() {
	tv.SetFlag(int(TreeViewFlagClosed))
}

// SetOpen clears the closed flag for this node -- call Open() method to open
// a node and update view
func (tv *TreeView) SetOpen() {
	tv.ClearFlag(int(TreeViewFlagClosed))
}

// SetClosedState sets the closed state based on arg
func (tv *TreeView) SetClosedState(closed bool) {
	tv.SetFlagState(closed, int(TreeViewFlagClosed))
}

// IsChanged returns whether this node has the changed flag set?  Only updated
// on the root note by GUI actions.
func (tv *TreeView) IsChanged() bool {
	return tv.HasFlag(int(TreeViewFlagChanged))
}

// SetChanged is called whenever a gui action updates the tree -- sets Changed
// flag on root node and emits signal
func (tv *TreeView) SetChanged() {
	if tv.RootView == nil {
		return
	}
	tv.RootView.SetFlag(int(TreeViewFlagChanged))
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewChanged), tv.This())
}

// HasClosedParent returns whether this node have a closed parent? if so, don't render!
func (tv *TreeView) HasClosedParent() bool {
	pcol := false
	tv.WalkUpParent(0, tv.This(), func(k ki.Ki, level int, d any) bool {
		_, pg := gi.AsWidget(k)
		if pg == nil {
			return ki.Break
		}
		if ki.TypeEmbeds(pg, TypeTreeView) {
			// nw := pg.Embed(TypeTreeView).(*TreeView)
			if pg.HasFlag(int(TreeViewFlagClosed)) {
				pcol = true
				return ki.Break
			}
		}
		return ki.Continue
	})
	return pcol
}

// Label returns the display label for this node, satisfying the Labeler interface
func (tv *TreeView) Label() string {
	if lbl, has := gi.ToLabeler(tv.SrcNode); has {
		return lbl
	}
	return tv.SrcNode.Name()
}

// UpdateInactive updates the Inactive state based on SrcNode -- returns true if
// inactive.  The inactivity of individual nodes only affects display properties
// typically, and not overall functional behavior, which is controlled by
// inactivity of the root node (i.e, make the root inactive to make entire tree
// read-only and non-modifiable)
func (tv *TreeView) UpdateInactive() bool {
	tv.ClearDisabled()
	if tv.SrcNode == nil {
		tv.SetDisabled()
	} else {
		if inact, err := tv.SrcNode.PropTry("inactive"); err == nil {
			if bo, ok := laser.ToBool(inact); bo && ok {
				tv.SetDisabled()
			}
		}
	}
	return tv.IsDisabled()
}

// RootIsInactive returns the inactive status of the root node, which is what
// controls the functional inactivity of the tree -- if individual nodes
// are inactive that only affects display typically.
func (tv *TreeView) RootIsInactive() bool {
	if tv.RootView == nil {
		return true
	}
	return tv.RootView.IsDisabled()
}

//////////////////////////////////////////////////////////////////////////////
//    Signals etc

// TreeViewSignals are signals that treeview can send -- these are all sent
// from the root tree view widget node, with data being the relevant node
// widget
type TreeViewSignals int64 //enums:enum

const (
	// node was selected
	TreeViewSelected TreeViewSignals = iota

	// TreeView unselected
	TreeViewUnselected

	// TreeView all items were selected
	TreeViewAllSelected

	// TreeView all items were unselected
	TreeViewAllUnselected

	// closed TreeView was opened
	TreeViewOpened

	// open TreeView was closed -- children not visible
	TreeViewClosed

	// means that some kind of edit operation has taken place
	// by the user via the gui -- we don't track the details, just that
	// changes have happened
	TreeViewChanged

	// a node was inserted into the tree (Paste, DND)
	// in this case, the data is the *source node* that was inserted
	TreeViewInserted

	// a node was deleted from the tree (Cut, DND Move)
	TreeViewDeleted
)

// TreeViewFlags extend NodeBase NodeFlags to hold TreeView state
type TreeViewFlags ki.Flags //enums:bitflag

const (
	// TreeViewFlagClosed means node is toggled closed (children not visible)
	TreeViewFlagClosed TreeViewFlags = TreeViewFlags(gi.WidgetFlagsN) + iota

	// TreeViewFlagChanged is updated on the root node whenever a gui edit is
	// made through the tree view on the tree -- this does not track any other
	// changes that might have occurred in the tree itself.
	// Also emits a TreeViewChanged signal on the root node.
	TreeViewFlagChanged

	// TreeViewFlagNoTemplate -- this node is not using a style template -- should
	// be restyled on any full re-render change
	TreeViewFlagNoTemplate

	// TreeViewFlagUpdtRoot -- for any update signal that comes from the source
	// root node, do a full update of the treeview.  This increases responsiveness
	// of the updating and makes it easy to trigger a full update by updating the root
	// node, but can be slower when not needed
	TreeViewFlagUpdtRoot
)

// TreeViewStates are mutually-exclusive tree view states -- determines appearance
type TreeViewStates int32 //enums:enum

const (
	// TreeViewActive is normal state -- there but not being interacted with
	TreeViewActive TreeViewStates = iota

	// TreeViewSel is selected
	TreeViewSel

	// TreeViewFocus is in focus -- will respond to keyboard input
	TreeViewFocus

	// TreeViewInactive is inactive -- if SrcNode is nil, or source has "inactive" property
	// set, or treeview node has inactive property set directly
	TreeViewInactive
)

// TreeViewSelectors are Style selector names for the different states:
var TreeViewSelectors = []string{":active", ":selected", ":focus", ":inactive"}

// These are special properties established on the RootView for maintaining
// overall tree state
const (
	// TreeViewSelProp is a slice of tree views that are currently selected
	// -- much more efficient to update the list rather than regenerate it,
	// especially for a large tree
	TreeViewSelProp = "__SelectedList"

	// TreeViewSelModeProp is a bool that, if true, automatically selects nodes
	// when nodes are moved to via keyboard actions
	TreeViewSelModeProp = "__SelectMode"
)

//////////////////////////////////////////////////////////////////////////////
//    Selection

// SelectMode returns true if keyboard movements should automatically select nodes
func (tv *TreeView) SelectMode() bool {
	smp, err := tv.RootView.PropTry(TreeViewSelModeProp)
	if err != nil {
		tv.SetSelectMode(false)
		return false
	} else {
		return smp.(bool)
	}
}

// SetSelectMode updates the select mode
func (tv *TreeView) SetSelectMode(selMode bool) {
	tv.RootView.SetProp(TreeViewSelModeProp, selMode)
}

// SelectModeToggle toggles the SelectMode
func (tv *TreeView) SelectModeToggle() {
	if tv.SelectMode() {
		tv.SetSelectMode(false)
	} else {
		tv.SetSelectMode(true)
	}
}

// SelectedViews returns a slice of the currently-selected TreeViews within
// the entire tree, using a list maintained by the root node
func (tv *TreeView) SelectedViews() []*TreeView {
	if tv.RootView == nil {
		return nil
	}
	var sl []*TreeView
	slp, err := tv.RootView.PropTry(TreeViewSelProp)
	if err != nil {
		sl = make([]*TreeView, 0)
		tv.SetSelectedViews(sl)
	} else {
		sl = slp.([]*TreeView)
	}
	return sl
}

// SetSelectedViews updates the selected views to given list
func (tv *TreeView) SetSelectedViews(sl []*TreeView) {
	if tv.RootView != nil {
		tv.RootView.SetProp(TreeViewSelProp, sl)
	}
}

// SelectedSrcNodes returns a slice of the currently-selected source nodes
// in the entire tree view
func (tv *TreeView) SelectedSrcNodes() ki.Slice {
	sn := make(ki.Slice, 0)
	sl := tv.SelectedViews()
	for _, v := range sl {
		sn = append(sn, v.SrcNode)
	}
	return sn
}

// Select selects this node (if not already selected) -- must use this method
// to update global selection list
func (tv *TreeView) Select() {
	if !tv.StateIs(states.Selected) {
		tv.SetSelected()
		sl := tv.SelectedViews()
		sl = append(sl, tv)
		tv.SetSelectedViews(sl)
		tv.UpdateSig()
	}
}

// Unselect unselects this node (if selected) -- must use this method
// to update global selection list
func (tv *TreeView) Unselect() {
	if tv.StateIs(states.Selected) {
		tv.ClearSelected()
		sl := tv.SelectedViews()
		sz := len(sl)
		for i := 0; i < sz; i++ {
			if sl[i] == tv {
				sl = append(sl[:i], sl[i+1:]...)
				break
			}
		}
		tv.SetSelectedViews(sl)
		tv.UpdateSig()
	}
}

// UnselectAll unselects all selected items in the view
func (tv *TreeView) UnselectAll() {
	if tv.Scene == nil {
		return
	}
	wupdt := tv.TopUpdateStart()
	sl := tv.SelectedViews()
	tv.SetSelectedViews(nil) // clear in advance
	for _, v := range sl {
		v.ClearSelected()
		v.UpdateSig()
	}
	tv.TopUpdateEnd(wupdt)
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewAllUnselected), tv.This())
}

// SelectAll all items in view
func (tv *TreeView) SelectAll() {
	if tv.Scene == nil {
		return
	}
	wupdt := tv.TopUpdateStart()
	tv.UnselectAll()
	nn := tv.RootView
	nn.Select()
	for nn != nil {
		nn = nn.MoveDown(events.SelectQuiet)
	}
	tv.TopUpdateEnd(wupdt)
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewAllSelected), tv.This())
}

// SelectUpdate updates selection to include this node, using selectmode
// from mouse event (ExtendContinuous, ExtendOne).  Returns true if this node selected
func (tv *TreeView) SelectUpdate(mode events.SelectModes) bool {
	if mode == events.NoSelect {
		return false
	}
	wupdt := tv.TopUpdateStart()
	sel := false
	switch mode {
	case events.SelectOne:
		if tv.StateIs(states.Selected) {
			sl := tv.SelectedViews()
			if len(sl) > 1 {
				tv.UnselectAll()
				tv.Select()
				tv.GrabFocus()
				sel = true
			}
		} else {
			tv.UnselectAll()
			tv.Select()
			tv.GrabFocus()
			sel = true
		}
	case events.ExtendContinuous:
		sl := tv.SelectedViews()
		if len(sl) == 0 {
			tv.Select()
			tv.GrabFocus()
			sel = true
		} else {
			minIdx := -1
			maxIdx := 0
			for _, v := range sl {
				if minIdx < 0 {
					minIdx = v.ViewIdx
				} else {
					minIdx = min(minIdx, v.ViewIdx)
				}
				maxIdx = max(maxIdx, v.ViewIdx)
			}
			cidx := tv.ViewIdx
			nn := tv
			tv.Select()
			if tv.ViewIdx < minIdx {
				for cidx < minIdx {
					nn = nn.MoveDown(events.SelectQuiet) // just select
					cidx = nn.ViewIdx
				}
			} else if tv.ViewIdx > maxIdx {
				for cidx > maxIdx {
					nn = nn.MoveUp(events.SelectQuiet) // just select
					cidx = nn.ViewIdx
				}
			}
		}
	case events.ExtendOne:
		if tv.StateIs(states.Selected) {
			tv.UnselectAction()
		} else {
			tv.Select()
			tv.GrabFocus()
			sel = true
		}
	case events.SelectQuiet:
		tv.Select()
		// not sel -- no signal..
	case events.UnselectQuiet:
		tv.Unselect()
		// not sel -- no signal..
	}
	tv.TopUpdateEnd(wupdt)
	return sel
}

// SelectAction updates selection to include this node, using selectmode
// from mouse event (ExtendContinuous, ExtendOne), and emits selection signal
// returns true if signal emitted
func (tv *TreeView) SelectAction(mode events.SelectModes) bool {
	sel := tv.SelectUpdate(mode)
	if sel {
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewSelected), tv.This())
	}
	return sel
}

// UnselectAction unselects this node (if selected) -- and emits a signal
func (tv *TreeView) UnselectAction() {
	if tv.StateIs(states.Selected) {
		tv.Unselect()
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewUnselected), tv.This())
	}
}

//////////////////////////////////////////////////////////////////////////////
//    Moving

// MoveDown moves the selection down to next element in the tree, using given
// select mode (from keyboard modifiers) -- returns newly selected node
func (tv *TreeView) MoveDown(selMode events.SelectModes) *TreeView {
	if tv.Par == nil {
		return nil
	}
	if tv.IsClosed() || !tv.HasChildren() { // next sibling
		return tv.MoveDownSibling(selMode)
	} else {
		if tv.HasChildren() {
			nn := tv.Child(0).Embed(TypeTreeView).(*TreeView)
			if nn != nil {
				nn.SelectUpdate(selMode)
				return nn
			}
		}
	}
	return nil
}

// MoveDownAction moves the selection down to next element in the tree, using given
// select mode (from keyboard modifiers) -- and emits select event for newly selected item
func (tv *TreeView) MoveDownAction(selMode events.SelectModes) *TreeView {
	nn := tv.MoveDown(selMode)
	if nn != nil && nn != tv {
		nn.GrabFocus()
		nn.ScrollToMe()
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewSelected), nn.This())
	}
	return nn
}

// MoveDownSibling moves down only to siblings, not down into children, using
// given select mode (from keyboard modifiers)
func (tv *TreeView) MoveDownSibling(selMode events.SelectModes) *TreeView {
	if tv.Par == nil {
		return nil
	}
	if tv == tv.RootView {
		return nil
	}
	myidx, ok := tv.IndexInParent()
	if ok && myidx < len(*tv.Par.Children())-1 {
		nn := tv.Par.Child(myidx + 1).Embed(TypeTreeView).(*TreeView)
		if nn != nil {
			nn.SelectUpdate(selMode)
			return nn
		}
	} else {
		return tv.Par.Embed(TypeTreeView).(*TreeView).MoveDownSibling(selMode) // try up
	}
	return nil
}

// MoveUp moves selection up to previous element in the tree, using given
// select mode (from keyboard modifiers) -- returns newly selected node
func (tv *TreeView) MoveUp(selMode events.SelectModes) *TreeView {
	if tv.Par == nil || tv == tv.RootView {
		return nil
	}
	myidx, ok := tv.IndexInParent()
	if ok && myidx > 0 {
		nn := tv.Par.Child(myidx - 1).Embed(TypeTreeView).(*TreeView)
		if nn != nil {
			return nn.MoveToLastChild(selMode)
		}
	} else {
		if tv.Par != nil {
			nn := tv.Par.Embed(TypeTreeView).(*TreeView)
			if nn != nil {
				nn.SelectUpdate(selMode)
				return nn
			}
		}
	}
	return nil
}

// MoveUpAction moves the selection up to previous element in the tree, using given
// select mode (from keyboard modifiers) -- and emits select event for newly selected item
func (tv *TreeView) MoveUpAction(selMode events.SelectModes) *TreeView {
	nn := tv.MoveUp(selMode)
	if nn != nil && nn != tv {
		nn.GrabFocus()
		nn.ScrollToMe()
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewSelected), nn.This())
	}
	return nn
}

// TreeViewPageSteps is the number of steps to take in PageUp / Down events
var TreeViewPageSteps = 10

// MovePageUpAction moves the selection up to previous TreeViewPageSteps elements in the tree,
// using given select mode (from keyboard modifiers) -- and emits select event for newly selected item
func (tv *TreeView) MovePageUpAction(selMode events.SelectModes) *TreeView {
	wupdt := tv.TopUpdateStart()
	mvMode := selMode
	if selMode == events.SelectOne {
		mvMode = events.NoSelect
	} else if selMode == events.ExtendContinuous || selMode == events.ExtendOne {
		mvMode = events.SelectQuiet
	}
	fnn := tv.MoveUp(mvMode)
	if fnn != nil && fnn != tv {
		for i := 1; i < TreeViewPageSteps; i++ {
			nn := fnn.MoveUp(mvMode)
			if nn == nil || nn == fnn {
				break
			}
			fnn = nn
		}
		if selMode == events.SelectOne {
			fnn.SelectUpdate(selMode)
		}
		fnn.GrabFocus()
		fnn.ScrollToMe()
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewSelected), fnn.This())
	}
	tv.TopUpdateEnd(wupdt)
	return fnn
}

// MovePageDownAction moves the selection up to previous TreeViewPageSteps elements in the tree,
// using given select mode (from keyboard modifiers) -- and emits select event for newly selected item
func (tv *TreeView) MovePageDownAction(selMode events.SelectModes) *TreeView {
	wupdt := tv.TopUpdateStart()
	mvMode := selMode
	if selMode == events.SelectOne {
		mvMode = events.NoSelect
	} else if selMode == events.ExtendContinuous || selMode == events.ExtendOne {
		mvMode = events.SelectQuiet
	}
	fnn := tv.MoveDown(mvMode)
	if fnn != nil && fnn != tv {
		for i := 1; i < TreeViewPageSteps; i++ {
			nn := fnn.MoveDown(mvMode)
			if nn == nil || nn == fnn {
				break
			}
			fnn = nn
		}
		if selMode == events.SelectOne {
			fnn.SelectUpdate(selMode)
		}
		fnn.GrabFocus()
		fnn.ScrollToMe()
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewSelected), fnn.This())
	}
	tv.TopUpdateEnd(wupdt)
	return fnn
}

// MoveToLastChild moves to the last child under me, using given select mode
// (from keyboard modifiers)
func (tv *TreeView) MoveToLastChild(selMode events.SelectModes) *TreeView {
	if tv.Par == nil || tv == tv.RootView {
		return nil
	}
	if !tv.IsClosed() && tv.HasChildren() {
		nnk, err := tv.Children().ElemFromEndTry(0)
		if err == nil {
			nn := nnk.Embed(TypeTreeView).(*TreeView)
			return nn.MoveToLastChild(selMode)
		}
	} else {
		tv.SelectUpdate(selMode)
		return tv
	}
	return nil
}

// MoveHomeAction moves the selection up to top of the tree,
// using given select mode (from keyboard modifiers)
// and emits select event for newly selected item
func (tv *TreeView) MoveHomeAction(selMode events.SelectModes) *TreeView {
	tv.RootView.SelectUpdate(selMode)
	tv.RootView.GrabFocus()
	tv.RootView.ScrollToMe()
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewSelected), tv.RootView.This())
	return tv.RootView
}

// MoveEndAction moves the selection to the very last node in the tree
// using given select mode (from keyboard modifiers) -- and emits select event
// for newly selected item
func (tv *TreeView) MoveEndAction(selMode events.SelectModes) *TreeView {
	wupdt := tv.TopUpdateStart()
	mvMode := selMode
	if selMode == events.SelectOne {
		mvMode = events.NoSelect
	} else if selMode == events.ExtendContinuous || selMode == events.ExtendOne {
		mvMode = events.SelectQuiet
	}
	fnn := tv.MoveDown(mvMode)
	if fnn != nil && fnn != tv {
		for {
			nn := fnn.MoveDown(mvMode)
			if nn == nil || nn == fnn {
				break
			}
			fnn = nn
		}
		if selMode == events.SelectOne {
			fnn.SelectUpdate(selMode)
		}
		fnn.GrabFocus()
		fnn.ScrollToMe()
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewSelected), fnn.This())
	}
	tv.TopUpdateEnd(wupdt)
	return fnn
}

// Close closes the given node and updates the view accordingly (if it is not already closed)
func (tv *TreeView) Close() {
	if !tv.IsClosed() {
		updt := tv.UpdateStart()
		if tv.HasChildren() {
			tv.SetFullReRender()
		}
		tv.SetClosed()
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewClosed), tv.This())
		tv.UpdateEnd(updt)
	}
}

// Open opens the given node and updates the view accordingly (if it is not already opened)
func (tv *TreeView) Open() {
	if tv.IsClosed() {
		updt := tv.UpdateStart()
		if tv.HasChildren() {
			tv.SetFullReRender()
		}
		if tv.HasChildren() {
			tv.SetClosedState(false)
		}
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewOpened), tv.This())
		tv.UpdateEnd(updt)
	} else if !tv.HasChildren() {
		// non-children nodes get double-click open for example
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewOpened), tv.This())
	}
}

// ToggleClose toggles the close / open status: if closed, opens, and vice-versa
func (tv *TreeView) ToggleClose() {
	if tv.IsClosed() {
		tv.Open()
	} else {
		tv.Close()
	}
}

// OpenAll opens the given node and all of its sub-nodes
func (tv *TreeView) OpenAll() {
	wupdt := tv.TopUpdateStart()
	updt := tv.UpdateStart()
	tv.SetFullReRender()
	tv.WalkPre(func(k Ki) bool {
		tvki := k.Embed(TypeTreeView)
		if tvki != nil {
			tvki.(*TreeView).SetClosedState(false)
		}
		return ki.Continue
	})
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewOpened), tv.This())
	tv.UpdateEnd(updt)
	tv.TopUpdateEnd(wupdt)
}

// CloseAll closes the given node and all of its sub-nodes
func (tv *TreeView) CloseAll() {
	wupdt := tv.TopUpdateStart()
	updt := tv.UpdateStart()
	tv.SetFullReRender()
	tv.WalkPre(func(k Ki) bool {
		tvki := k.Embed(TypeTreeView)
		if tvki != nil {
			tvki.(*TreeView).SetClosedState(true)
			return ki.Continue
		}
		return ki.Break
	})
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewClosed), tv.This())
	tv.UpdateEnd(updt)
	tv.TopUpdateEnd(wupdt)
}

// OpenParents opens all the parents of this node, so that it will be visible
func (tv *TreeView) OpenParents() {
	wupdt := tv.TopUpdateStart()
	updt := tv.RootView.UpdateStart()
	tv.RootView.SetFullReRender()
	tv.WalkUpParent(0, tv.This(), func(k ki.Ki, level int, d any) bool {
		tvki := k.Embed(TypeTreeView)
		if tvki != nil {
			tvki.(*TreeView).SetClosedState(false)
			return ki.Continue
		}
		return ki.Break
	})
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewOpened), tv.This())
	tv.RootView.UpdateEnd(updt)
	tv.TopUpdateEnd(wupdt)
}

// FindSrcNode finds TreeView node for given source node, or nil if not found
func (tv *TreeView) FindSrcNode(kn ki.Ki) *TreeView {
	var ttv *TreeView
	tv.WalkPre(func(k Ki) bool {
		tvki := k.Embed(TypeTreeView)
		if tvki != nil {
			tvk := tvki.(*TreeView)
			if tvk.SrcNode == kn {
				ttv = tvk
				return ki.Break
			}
		}
		return ki.Continue
	})
	return ttv
}

//////////////////////////////////////////////////////////////////////////////
//    Modifying Source Tree

func (tv *TreeView) ContextMenuPos() (pos image.Point) {
	pos.X = tv.WinBBox.Min.X + int(tv.Indent.Dots)
	pos.Y = (tv.WinBBox.Min.Y + tv.WinBBox.Max.Y) / 2
	return
}

func (tv *TreeView) MakeContextMenu(m *gi.MenuActions) {
	// derived types put native menu code here
	if tv.CtxtMenuFunc != nil {
		tv.CtxtMenuFunc(tv.This().(gi.Widget), m)
	}
	// note: root inactivity is relevant factor here..
	if CtxtMenuView(tv.SrcNode, tv.RootIsInactive(), tv.Scene, m) { // our viewed obj's menu
		if tv.ShowViewCtxtMenu {
			m.AddSeparator("sep-tvmenu")
			CtxtMenuView(tv.This(), tv.RootIsInactive(), tv.Scene, m)
		}
	} else {
		CtxtMenuView(tv.This(), tv.RootIsInactive(), tv.Scene, m)
	}
}

// IsRootOrField returns true if given node is either the root of the
// tree or a field -- various operations can not be performed on these -- if
// string is passed, then a prompt dialog is presented with that as the name
// of the operation being attempted -- otherwise it silently returns (suitable
// for context menu UpdateFunc).
func (tv *TreeView) IsRootOrField(op string) bool {
	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView IsRootOrField nil SrcNode in: %v\n", tv.Path())
		return false
	}
	if sk.Is(Field) {
		if op != "" {
			gi.PromptDialog(tv.Scene, gi.DlgOpts{Title: "TreeView " + op, Prompt: fmt.Sprintf("Cannot %v fields", op)}, gi.AddOk, gi.NoCancel, nil, nil)
		}
		return true
	}
	if tv.This() == tv.RootView.This() {
		if op != "" {
			gi.PromptDialog(tv.Scene, gi.DlgOpts{Title: "TreeView " + op, Prompt: fmt.Sprintf("Cannot %v the root of the tree", op)}, gi.AddOk, gi.NoCancel, nil, nil)
		}
		return true
	}
	return false
}

// SrcInsertAfter inserts a new node in the source tree after this node, at
// the same (sibling) level, prompting for the type of node to insert
func (tv *TreeView) SrcInsertAfter() {
	tv.SrcInsertAt(1, "Insert After")
}

// SrcInsertBefore inserts a new node in the source tree before this node, at
// the same (sibling) level, prompting for the type of node to insert
func (tv *TreeView) SrcInsertBefore() {
	tv.SrcInsertAt(0, "Insert Before")
}

// SrcInsertAt inserts a new node in the source tree at given relative offset
// from this node, at the same (sibling) level, prompting for the type of node to insert
func (tv *TreeView) SrcInsertAt(rel int, actNm string) {
	if tv.IsRootOrField(actNm) {
		return
	}
	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView %v nil SrcNode in: %v\n", actNm, tv.Path())
		return
	}
	myidx, ok := sk.IndexInParent()
	if !ok {
		return
	}
	myidx += rel
	gi.NewKiDialog(tv.Scene, sk.BaseIface(),
		gi.DlgOpts{Title: actNm, Prompt: "Number and Type of Items to Insert:"},
		tv.Par.This(), func(recv, send ki.Ki, sig int64, data any) {
			if sig == int64(gi.DialogAccepted) {
				tvv, _ := recv.Embed(TypeTreeView).(*TreeView)
				par := tvv.SrcNode
				dlg, _ := send.(*gi.DialogStage)
				n, typ := gi.NewKiDialogValues(dlg)
				updt := par.UpdateStart()
				var ski ki.Ki
				for i := 0; i < n; i++ {
					nm := fmt.Sprintf("New%v%v", typ.Name(), myidx+rel+i)
					par.SetChildAdded()
					nki := par.InsertNewChild(typ, myidx+i, nm)
					if i == n-1 {
						ski = nki
					}
					tv.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewInserted), nki.This())
				}
				tvv.SetChanged()
				par.UpdateEnd(updt)
				if ski != nil {
					if tvk := tvv.ChildByName("tv_"+ski.Name(), 0); tvk != nil {
						stv, _ := tvk.Embed(TypeTreeView).(*TreeView)
						stv.SelectAction(events.SelectOne)
					}
				}
			}
		})
}

// SrcAddChild adds a new child node to this one in the source tree,
// prompting the user for the type of node to add
func (tv *TreeView) SrcAddChild() {
	ttl := "Add Child"
	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView %v nil SrcNode in: %v\n", ttl, tv.Path())
		return
	}
	gi.NewKiDialog(tv.Scene, sk.BaseIface(),
		gi.DlgOpts{Title: ttl, Prompt: "Number and Type of Items to Add:"},
		tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			if sig == int64(gi.DialogAccepted) {
				tvv, _ := recv.Embed(TypeTreeView).(*TreeView)
				sk := tvv.SrcNode
				dlg, _ := send.(*gi.DialogStage)
				n, typ := gi.NewKiDialogValues(dlg)
				updt := sk.UpdateStart()
				sk.SetChildAdded()
				var ski ki.Ki
				for i := 0; i < n; i++ {
					nm := fmt.Sprintf("New%v%v", typ.Name(), i)
					nki := sk.NewChild(typ, nm)
					if i == n-1 {
						ski = nki
					}
					tv.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewInserted), nki.This())
				}
				tvv.SetChanged()
				sk.UpdateEnd(updt)
				if ski != nil {
					tvv.Open()
					if tvk := tvv.ChildByName("tv_"+ski.Name(), 0); tvk != nil {
						stv, _ := tvk.Embed(TypeTreeView).(*TreeView)
						stv.SelectAction(events.SelectOne)
					}
				}
			}
		})
}

// SrcDelete deletes the source node corresponding to this view node in the source tree
func (tv *TreeView) SrcDelete() {
	ttl := "Delete"
	if tv.IsRootOrField(ttl) {
		return
	}
	if tv.MoveDown(events.SelectOne) == nil {
		tv.MoveUp(events.SelectOne)
	}
	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView %v nil SrcNode in: %v\n", ttl, tv.Path())
		return
	}
	sk.Delete(true)
	tv.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewDeleted), sk.This())
	tv.SetChanged()
}

// SrcDuplicate duplicates the source node corresponding to this view node in
// the source tree, and inserts the duplicate after this node (as a new
// sibling)
func (tv *TreeView) SrcDuplicate() {
	ttl := "TreeView Duplicate"
	if tv.IsRootOrField(ttl) {
		return
	}
	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView %v nil SrcNode in: %v\n", ttl, tv.Path())
		return
	}
	if tv.Par == nil {
		return
	}
	tvpar := tv.Par.Embed(TypeTreeView).(*TreeView)
	par := tvpar.SrcNode
	if par == nil {
		log.Printf("TreeView %v nil SrcNode in: %v\n", ttl, tvpar.Path())
		return
	}
	myidx, ok := sk.IndexInParent()
	if !ok {
		return
	}
	updt := par.UpdateStart()
	nm := fmt.Sprintf("%v_Copy", sk.Name())
	nwkid := sk.Clone()
	nwkid.SetName(nm)
	par.SetChildAdded()
	par.InsertChild(nwkid, myidx+1)
	tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewInserted), nwkid.This())
	par.UpdateEnd(updt)
	tvpar.SetChanged()
	if tvk := tvpar.ChildByName("tv_"+nm, 0); tvk != nil {
		stv, _ := tvk.Embed(TypeTreeView).(*TreeView)
		stv.SelectAction(events.SelectOne)
	}
}

// SrcEdit pulls up a StructViewDialog window on the source object viewed by this node
func (tv *TreeView) SrcEdit() {
	if tv.SrcNode == nil {
		log.Printf("TreeView SrcEdit nil SrcNode in: %v\n", tv.Path())
		return
	}
	tynm := laser.NonPtrType(ki.Type(tv.SrcNode)).Name()
	StructViewDialog(tv.Scene, tv.SrcNode, DlgOpts{Title: tynm}, nil, nil)
}

// SrcGoGiEditor pulls up a new GoGiEditor window on the source object viewed by this node
func (tv *TreeView) SrcGoGiEditor() {
	if tv.SrcNode == nil {
		log.Printf("TreeView SrcGoGiEditor nil SrcNode in: %v\n", tv.Path())
		return
	}
	GoGiEditorDialog(tv.SrcNode)
}

//////////////////////////////////////////////////////////////////////////////
//    Copy / Cut / Paste

// MimeData adds mimedata for this node: a text/plain of the Path, and
// an application/json of the source node.
// satisfies Clipper.MimeData interface
func (tv *TreeView) MimeData(md *mimedata.Mimes) {
	sroot := tv.RootView.SrcNode
	src := tv.SrcNode
	*md = append(*md, mimedata.NewTextData(src.PathFrom(sroot)))
	var buf bytes.Buffer
	err := src.WriteJSON(&buf, ki.Indent) // true = pretty for clipboard..
	if err == nil {
		*md = append(*md, &mimedata.Data{Type: filecat.DataJson, Data: buf.Bytes()})
	} else {
		log.Printf("gi.TreeView MimeData SaveJSON error: %v\n", err)
	}
}

// NodesFromMimeData creates a slice of Ki node(s) from given mime data
// and also a corresponding slice of original paths
func (tv *TreeView) NodesFromMimeData(md mimedata.Mimes) (ki.Slice, []string) {
	ni := len(md) / 2
	sl := make(ki.Slice, 0, ni)
	pl := make([]string, 0, ni)
	for _, d := range md {
		if d.Type == filecat.DataJson {
			nki, err := ki.ReadNewJSON(bytes.NewReader(d.Data))
			if err == nil {
				sl = append(sl, nki)
			} else {
				log.Printf("TreeView NodesFromMimeData: JSON load error: %v\n", err)
			}
		} else if d.Type == filecat.TextPlain { // paths
			pl = append(pl, string(d.Data))
		}
	}
	return sl, pl
}

// Copy copies to clip.Board, optionally resetting the selection
// satisfies gi.Clipper interface and can be overridden by subtypes
func (tv *TreeView) Copy(reset bool) {
	sels := tv.SelectedViews()
	nitms := max(1, len(sels))
	md := make(mimedata.Mimes, 0, 2*nitms)
	tv.This().(gi.Clipper).MimeData(&md) // source is always first..
	if nitms > 1 {
		for _, sn := range sels {
			if sn.This() != tv.This() {
				sn.This().(gi.Clipper).MimeData(&md)
			}
		}
	}
	goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Write(md)
	if reset {
		tv.UnselectAll()
	}
}

// Cut copies to clip.Board and deletes selected items
// satisfies gi.Clipper interface and can be overridden by subtypes
func (tv *TreeView) Cut() {
	if tv.IsRootOrField("Cut") {
		return
	}
	tv.Copy(false)
	sels := tv.SelectedSrcNodes()
	tv.UnselectAll()
	for _, sn := range sels {
		sn.Delete(true)
	}
	tv.SetChanged()
}

// Paste pastes clipboard at given node
// satisfies gi.Clipper interface and can be overridden by subtypes
func (tv *TreeView) Paste() {
	md := goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Read([]string{filecat.DataJson})
	if md != nil {
		tv.PasteMenu(md)
	}
}

// MakePasteMenu makes the menu of options for paste events
func (tv *TreeView) MakePasteMenu(m *gi.MenuActions, data any) {
	if len(*m) > 0 {
		return
	}
	m.AddAction(gi.ActOpts{Label: "Assign To", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
		tvv := recv.Embed(TypeTreeView).(*TreeView)
		tvv.PasteAssign(data.(mimedata.Mimes))
	})
	m.AddAction(gi.ActOpts{Label: "Add to Children", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
		tvv := recv.Embed(TypeTreeView).(*TreeView)
		tvv.PasteChildren(data.(mimedata.Mimes), events.DropCopy)
	})
	if !tv.IsRootOrField("") && tv.RootView.This() != tv.This() {
		m.AddAction(gi.ActOpts{Label: "Insert Before", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			tvv := recv.Embed(TypeTreeView).(*TreeView)
			tvv.PasteBefore(data.(mimedata.Mimes), events.DropCopy)
		})
		m.AddAction(gi.ActOpts{Label: "Insert After", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			tvv := recv.Embed(TypeTreeView).(*TreeView)
			tvv.PasteAfter(data.(mimedata.Mimes), events.DropCopy)
		})
	}
	m.AddAction(gi.ActOpts{Label: "Cancel", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
	})
	// todo: compare, etc..
}

// PasteMenu performs a paste from the clipboard using given data -- pops up
// a menu to determine what specifically to do
func (tv *TreeView) PasteMenu(md mimedata.Mimes) {
	tv.UnselectAll()
	if tv.SrcNode == nil {
		log.Printf("TreeView PasteMenu nil SrcNode in: %v\n", tv.Path())
		return
	}
	var menu gi.MenuActions
	tv.MakePasteMenu(&menu, md)
	pos := tv.ContextMenuPos()
	gi.NewMenu(menu, tv.This().(gi.Widget), pos).Run()
}

// PasteAssign assigns mime data (only the first one!) to this node
func (tv *TreeView) PasteAssign(md mimedata.Mimes) {
	sl, _ := tv.NodesFromMimeData(md)
	if len(sl) == 0 {
		return
	}
	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView PasteAssign nil SrcNode in: %v\n", tv.Path())
		return
	}
	sk.CopyFrom(sl[0])
	tv.SetChanged()
}

// PasteBefore inserts object(s) from mime data before this node.
// If another item with the same name already exists, it will
// append _Copy on the name of the inserted objects
func (tv *TreeView) PasteBefore(md mimedata.Mimes, mod events.DropMods) {
	tv.PasteAt(md, mod, 0, "Paste Before")
}

// PasteAfter inserts object(s) from mime data after this node.
// If another item with the same name already exists, it will
// append _Copy on the name of the inserted objects
func (tv *TreeView) PasteAfter(md mimedata.Mimes, mod events.DropMods) {
	tv.PasteAt(md, mod, 1, "Paste After")
}

// This is a kind of hack to prevent moved items from being deleted, using DND
const TreeViewTempMovedTag = `_\&MOVED\&`

// PasteAt inserts object(s) from mime data at rel position to this node.
// If another item with the same name already exists, it will
// append _Copy on the name of the inserted objects
func (tv *TreeView) PasteAt(md mimedata.Mimes, mod events.DropMods, rel int, actNm string) {
	sl, pl := tv.NodesFromMimeData(md)

	if tv.Par == nil {
		return
	}
	tvpar := tv.Par.Embed(TypeTreeView).(*TreeView)
	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView %v nil SrcNode in: %v\n", actNm, tv.Path())
		return
	}
	par := sk.Parent()
	if par == nil {
		gi.PromptDialog(tv.Scene, gi.DlgOpts{Title: actNm, Prompt: "Cannot insert after the root of the tree"}, gi.AddOk, gi.NoCancel, nil, nil)
		return
	}
	myidx, ok := sk.IndexInParent()
	if !ok {
		return
	}
	myidx += rel
	sroot := tv.RootView.SrcNode
	updt := par.UpdateStart()
	sz := len(sl)
	var ski ki.Ki
	for i, ns := range sl {
		orgpath := pl[i]
		if mod != events.DropMove {
			if cn := par.ChildByName(ns.Name(), 0); cn != nil {
				ns.SetName(ns.Name() + "_Copy")
			}
		}
		par.SetChildAdded()
		par.InsertChild(ns, myidx+i)
		npath := ns.PathFrom(sroot)
		if mod == events.DropMove && npath == orgpath { // we will be nuked immediately after drag
			ns.SetName(ns.Name() + TreeViewTempMovedTag) // special keyword :)
		}
		if i == sz-1 {
			ski = ns
		}
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewInserted), ns.This())
	}
	par.UpdateEnd(updt)
	tvpar.SetChanged()
	if ski != nil {
		if tvk := tvpar.ChildByName("tv_"+ski.Name(), 0); tvk != nil {
			stv, _ := tvk.Embed(TypeTreeView).(*TreeView)
			stv.SelectAction(events.SelectOne)
		}
	}
}

// PasteChildren inserts object(s) from mime data at end of children of this
// node
func (tv *TreeView) PasteChildren(md mimedata.Mimes, mod events.DropMods) {
	sl, _ := tv.NodesFromMimeData(md)

	sk := tv.SrcNode
	if sk == nil {
		log.Printf("TreeView PasteChildren nil SrcNode in: %v\n", tv.Path())
		return
	}
	updt := sk.UpdateStart()
	sk.SetChildAdded()
	for _, ns := range sl {
		sk.AddChild(ns)
		tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewInserted), ns.This())
	}
	sk.UpdateEnd(updt)
	tv.SetChanged()
}

//////////////////////////////////////////////////////////////////////////////
//    Drag-n-Drop

// DragNDropStart starts a drag-n-drop on this node -- it includes any other
// selected nodes as well, each as additional records in mimedata
func (tv *TreeView) DragNDropStart() {
	sels := tv.SelectedViews()
	nitms := max(1, len(sels))
	md := make(mimedata.Mimes, 0, 2*nitms)
	tv.This().(gi.Clipper).MimeData(&md) // source is always first..
	if nitms > 1 {
		for _, sn := range sels {
			if sn.This() != tv.This() {
				sn.This().(gi.Clipper).MimeData(&md)
			}
		}
	}
	sp := &gi.Sprite{}
	sp.GrabRenderFrom(tv) // todo: show number of items?
	gi.ImageClearer(sp.Pixels, 50.0)
	tv.ParentRenderWin().StartDragNDrop(tv.This(), md, sp)
}

// DragNDropTarget handles a drag-n-drop onto this node
func (tv *TreeView) DragNDropTarget(de events.Event) {
	de.Target = tv.This()
	if de.Mod == events.DropLink {
		de.Mod = events.DropCopy // link not supported -- revert to copy
	}
	de.SetHandled()
	tv.This().(gi.DragNDropper).Drop(de.Data, de.Mod)
}

// DragNDropExternal handles a drag-n-drop external drop onto this node
func (tv *TreeView) DragNDropExternal(de events.Event) {
	de.Target = tv.This()
	if de.Mod == events.DropLink {
		de.Mod = events.DropCopy // link not supported -- revert to copy
	}
	de.SetHandled()
	tv.This().(gi.DragNDropper).DropExternal(de.Data, de.Mod)
}

// DragNDropFinalize is called to finalize actions on the Source node prior to
// performing target actions -- mod must indicate actual action taken by the
// target, including ignore
func (tv *TreeView) DragNDropFinalize(mod events.DropMods) {
	if tv.Scene == nil {
		return
	}
	tv.UnselectAll()
	tv.ParentRenderWin().FinalizeDragNDrop(mod)
}

// DragNDropFinalizeDefMod is called to finalize actions on the Source node prior to
// performing target actions -- uses default drop mod in place when event was dropped.
func (tv *TreeView) DragNDropFinalizeDefMod() {
	win := tv.ParentRenderWin()
	if win == nil {
		return
	}
	tv.UnselectAll()
	win.FinalizeDragNDrop(win.EventMgr.DNDDropMod)
}

// Dragged is called after target accepts the drop -- we just remove
// elements that were moved
// satisfies gi.DragNDropper interface and can be overridden by subtypes
func (tv *TreeView) Dragged(de events.Event) {
	if de.Mod != events.DropMove {
		return
	}
	sroot := tv.RootView.SrcNode
	md := de.Data
	for _, d := range md {
		if d.Type == filecat.TextPlain { // link
			path := string(d.Data)
			sn := sroot.FindPath(path)
			if sn != nil {
				sn.Delete(true)
				tv.RootView.TreeViewSig.Emit(tv.RootView.This(), int64(TreeViewDeleted), sn.This())
			}
			sn = sroot.FindPath(path + TreeViewTempMovedTag)
			if sn != nil {
				psplt := strings.Split(path, "/")
				orgnm := psplt[len(psplt)-1]
				sn.SetName(orgnm)
				sn.UpdateSig()
			}
		}
	}
}

// MakeDropMenu makes the menu of options for dropping on a target
func (tv *TreeView) MakeDropMenu(m *gi.MenuActions, data any, mod events.DropMods) {
	if len(*m) > 0 {
		return
	}
	switch mod {
	case events.DropCopy:
		m.AddLabel("Copy (Use Shift to Move):")
	case events.DropMove:
		m.AddLabel("Move:")
	}
	if mod == events.DropCopy {
		m.AddAction(gi.ActOpts{Label: "Assign To", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			tvv := recv.Embed(TypeTreeView).(*TreeView)
			tvv.DropAssign(data.(mimedata.Mimes))
		})
	}
	m.AddAction(gi.ActOpts{Label: "Add to Children", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
		tvv := recv.Embed(TypeTreeView).(*TreeView)
		tvv.DropChildren(data.(mimedata.Mimes), mod) // captures mod
	})
	if !tv.IsRootOrField("") && tv.RootView.This() != tv.This() {
		m.AddAction(gi.ActOpts{Label: "Insert Before", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			tvv := recv.Embed(TypeTreeView).(*TreeView)
			tvv.DropBefore(data.(mimedata.Mimes), mod) // captures mod
		})
		m.AddAction(gi.ActOpts{Label: "Insert After", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			tvv := recv.Embed(TypeTreeView).(*TreeView)
			tvv.DropAfter(data.(mimedata.Mimes), mod) // captures mod
		})
	}
	m.AddAction(gi.ActOpts{Label: "Cancel", Data: data}, tv.This(), func(recv, send ki.Ki, sig int64, data any) {
		tvv := recv.Embed(TypeTreeView).(*TreeView)
		tvv.DropCancel()
	})
	// todo: compare, etc..
}

// Drop pops up a menu to determine what specifically to do with dropped items
// satisfies gi.DragNDropper interface and can be overridden by subtypes
func (tv *TreeView) Drop(md mimedata.Mimes, mod events.DropMods) {
	var menu gi.MenuActions
	tv.MakeDropMenu(&menu, md, mod)
	pos := tv.ContextMenuPos()
	gi.NewMenu(menu, tv.This().(gi.Widget), pos).Run()
}

// DropExternal is not handled by base case but could be in derived
func (tv *TreeView) DropExternal(md mimedata.Mimes, mod events.DropMods) {
	tv.DropCancel()
}

// DropAssign assigns mime data (only the first one!) to this node
func (tv *TreeView) DropAssign(md mimedata.Mimes) {
	tv.PasteAssign(md)
	tv.DragNDropFinalize(events.DropCopy)
}

// DropBefore inserts object(s) from mime data before this node
func (tv *TreeView) DropBefore(md mimedata.Mimes, mod events.DropMods) {
	tv.PasteBefore(md, mod)
	tv.DragNDropFinalize(mod)
}

// DropAfter inserts object(s) from mime data after this node
func (tv *TreeView) DropAfter(md mimedata.Mimes, mod events.DropMods) {
	tv.PasteAfter(md, mod)
	tv.DragNDropFinalize(mod)
}

// DropChildren inserts object(s) from mime data at end of children of this node
func (tv *TreeView) DropChildren(md mimedata.Mimes, mod events.DropMods) {
	tv.PasteChildren(md, mod)
	tv.DragNDropFinalize(mod)
}

// DropCancel cancels the drop action e.g., preventing deleting of source
// items in a Move case
func (tv *TreeView) DropCancel() {
	tv.DragNDropFinalize(events.DropIgnore)
}

////////////////////////////////////////////////////
// Infrastructure

func (tv *TreeView) TreeViewParent() *TreeView {
	if tv.Par == nil {
		return nil
	}
	if ki.TypeEmbeds(tv.Par, TypeTreeView) {
		return tv.Par.Embed(TypeTreeView).(*TreeView)
	}
	// I am rootview!
	return nil
}

// RootTreeView returns the root node of TreeView tree -- typically cached in
// RootView on each node, but this can be used if that cached value needs
// to be updated for any reason.
func (tv *TreeView) RootTreeView() *TreeView {
	rn := tv
	tv.WalkUp(0, tv.This(), func(k ki.Ki, level int, d any) bool {
		_, pg := gi.AsWidget(k)
		if pg == nil {
			return false
		}
		if ki.TypeEmbeds(k, TypeTreeView) {
			rn = k.Embed(TypeTreeView).(*TreeView)
			return true
		} else {
			return false
		}
	})
	return rn
}

func (tv *TreeView) KeyInput(kt events.Event) {
	if gi.KeyEventTrace {
		fmt.Printf("TreeView KeyInput: %v\n", tv.Path())
	}
	kf := gi.KeyFun(kt.KeyChord())
	selMode := events.SelectModeBits(kt.Modifiers)

	if selMode == events.SelectOne {
		if tv.SelectMode() {
			selMode = events.ExtendContinuous
		}
	}

	// first all the keys that work for inactive and active
	switch kf {
	case gi.KeyFunCancelSelect:
		tv.UnselectAll()
		tv.SetSelectMode(false)
		kt.SetHandled()
	case gi.KeyFunMoveRight:
		tv.Open()
		kt.SetHandled()
	case gi.KeyFunMoveLeft:
		tv.Close()
		kt.SetHandled()
	case gi.KeyFunMoveDown:
		tv.MoveDownAction(selMode)
		kt.SetHandled()
	case gi.KeyFunMoveUp:
		tv.MoveUpAction(selMode)
		kt.SetHandled()
	case gi.KeyFunPageUp:
		tv.MovePageUpAction(selMode)
		kt.SetHandled()
	case gi.KeyFunPageDown:
		tv.MovePageDownAction(selMode)
		kt.SetHandled()
	case gi.KeyFunHome:
		tv.MoveHomeAction(selMode)
		kt.SetHandled()
	case gi.KeyFunEnd:
		tv.MoveEndAction(selMode)
		kt.SetHandled()
	case gi.KeyFunSelectMode:
		tv.SelectModeToggle()
		kt.SetHandled()
	case gi.KeyFunSelectAll:
		tv.SelectAll()
		kt.SetHandled()
	case gi.KeyFunEnter:
		tv.ToggleClose()
		kt.SetHandled()
	case gi.KeyFunCopy:
		tv.This().(gi.Clipper).Copy(true)
		kt.SetHandled()
	}
	if !tv.RootIsInactive() && !kt.IsHandled() {
		switch kf {
		case gi.KeyFunDelete:
			tv.SrcDelete()
			kt.SetHandled()
		case gi.KeyFunDuplicate:
			tv.SrcDuplicate()
			kt.SetHandled()
		case gi.KeyFunInsert:
			tv.SrcInsertBefore()
			kt.SetHandled()
		case gi.KeyFunInsertAfter:
			tv.SrcInsertAfter()
			kt.SetHandled()
		case gi.KeyFunCut:
			tv.This().(gi.Clipper).Cut()
			kt.SetHandled()
		case gi.KeyFunPaste:
			tv.This().(gi.Clipper).Paste()
			kt.SetHandled()
		}
	}
}

func (tv *TreeView) TreeViewEvents() {
	tvwe.AddFunc(events.KeyChord, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		tvv := recv.Embed(TypeTreeView).(*TreeView)
		kt := d.(*events.Key)
		tvv.KeyInput(kt)
	})
	tvwe.AddFunc(goosi.DNDEvent, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		if recv == nil {
			return
		}
		de := d.(events.Event)
		tvv := recv.Embed(TypeTreeView).(*TreeView)
		switch de.Action {
		case events.Start:
			tvv.DragNDropStart()
		case events.DropOnTarget:
			tvv.DragNDropTarget(de)
		case events.DropFmSource:
			tvv.This().(gi.DragNDropper).Dragged(de)
		case events.External:
			tvv.DragNDropExternal(de)
		}
	})
	tvwe.AddFunc(goosi.DNDFocusEvent, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		if recv == nil {
			return
		}
		de := d.(*events.FocusEvent)
		tvv := recv.Embed(TypeTreeView).(*TreeView)
		switch de.Action {
		case events.Enter:
			tvv.ParentRenderWin().DNDSetCursor(de.Mod)
		case events.Exit:
			tvv.ParentRenderWin().DNDNotCursor()
		case events.Hover:
			tvv.Open()
		}
	})
	if tv.HasChildren() {
		if wb, ok := tv.BranchPart(); ok {
			wb.ButtonSig.ConnectOnly(tv.This(), func(recv, send ki.Ki, sig int64, data any) {
				if sig == int64(gi.ButtonToggled) {
					tvv, _ := recv.Embed(TypeTreeView).(*TreeView)
					tvv.ToggleClose()
				}
			})
		}
	}
	if lbl, ok := tv.LabelPart(); ok {
		// HiPri is needed to override label's native processing
		lblwe.AddFunc(events.MouseUp, gi.HiPri, func(recv, send ki.Ki, sig int64, d any) {
			lb, _ := recv.(*gi.Label)
			tvvi := lb.Parent().Parent()
			if tvvi == nil || tvvi.This() == nil { // deleted
				return
			}
			tvv := tvvi.Embed(TypeTreeView).(*TreeView)
			me := d.(events.Event)
			switch me.Button {
			case events.Left:
				switch me.Action {
				case events.DoubleClick:
					tvv.ToggleClose()
					me.SetHandled()
				case events.Release:
					tvv.SelectAction(me.SelectMode())
					me.SetHandled()
				}
			case events.Right:
				if me.Action == events.Release {
					me.SetHandled()
					tvv.This().(gi.Widget).ContextMenu()
				}
			}
		})
	}
}

////////////////////////////////////////////////////
// Widget interface

// qt calls the open / close thing a "branch"
// http://doc.qt.io/qt-5/stylesheet-examples.html#customizing-qtreeview

// BranchPart returns the branch in parts, if it exists
func (tv *TreeView) BranchPart() (*gi.CheckBox, bool) {
	if icc := tv.Parts.ChildByName("branch", 0); icc != nil {
		return icc.(*gi.CheckBox), true
	}
	return nil, false
}

// IconPart returns the icon in parts, if it exists
func (tv *TreeView) IconPart() (*gi.Icon, bool) {
	if icc := tv.Parts.ChildByName("icon", 1); icc != nil {
		return icc.(*gi.Icon), true
	}
	return nil, false
}

// LabelPart returns the label in parts, if it exists
func (tv *TreeView) LabelPart() (*gi.Label, bool) {
	if lbl := tv.Parts.ChildByName("label", 1); lbl != nil {
		return lbl.(*gi.Label), true
	}
	return nil, false
}

func (tv *TreeView) ConfigParts(sc *gi.Scene) {
	parts := tv.NewParts(gi.LayoutHoriz)
	parts.Style.Template = "giv.TreeView.Parts"
	config := ki.Config{}
	if tv.HasChildren() {
		config.Add(gi.TypeCheckBox, "branch")
	}
	if tv.Icon.IsValid() {
		config.Add(gi.IconType, "icon")
	}
	config.Add(gi.LabelType, "label")
	_, updt := parts.ConfigChildren(config)
	if tv.HasChildren() {
		if wb, ok := tv.BranchPart(); ok {
			if wb.Style.Template != "giv.TreeView.Branch" {
				wb.SetProp("no-focus", true) // note: cannot be in compiled props
				wb.Style.Template = "giv.TreeView.Branch"
				// STYTODO: do we really need this?
				wb.ApplyStyle(sc) // this is key for getting styling to take effect on first try
			}
		}
	}
	if tv.Icon.IsValid() {
		if ic, ok := tv.IconPart(); ok {
			// this only works after a second redraw..
			// ic.Sty.Template = "giv.TreeView.Icon"
			ic.SetIcon(tv.Icon)
		}
	}
	if lbl, ok := tv.LabelPart(); ok {
		// this does not work! even with redraws
		// lbl.Sty.Template = "giv.TreeView.Label"
		lbl.Props = nil
		// if tv.HasFlag(int(TreeViewFlagNoTemplate)) {
		// 	lbl.Redrawable = true // this prevents select highlight from rendering properly
		// }
		tv.Style.Font.CopyNonDefaultProps(lbl.This()) // copy our properties to label
		lbl.SetText(tv.Label())
	}
	parts.UpdateEnd(updt)
	tv.UpdateEndLayout(sc, updt)
}

var TreeViewProps = ki.Props{
	"CtxtMenuActive": ki.PropSlice{
		{"SrcAddChild", ki.Props{
			"label": "Add Child",
		}},
		{"SrcInsertBefore", ki.Props{
			"label":    "Insert Before",
			"shortcut": gi.KeyFunInsert,
			"updtfunc": ActionUpdateFunc(func(tvi any, act *gi.Action) {
				tv := tvi.(ki.Ki).Embed(TypeTreeView).(*TreeView)
				act.SetState(tv.IsRootOrField(""), states.Disabled)
			}),
		}},
		{"SrcInsertAfter", ki.Props{
			"label":    "Insert After",
			"shortcut": gi.KeyFunInsertAfter,
			"updtfunc": ActionUpdateFunc(func(tvi any, act *gi.Action) {
				tv := tvi.(ki.Ki).Embed(TypeTreeView).(*TreeView)
				act.SetState(tv.IsRootOrField(""), states.Disabled)
			}),
		}},
		{"SrcDuplicate", ki.Props{
			"label":    "Duplicate",
			"shortcut": gi.KeyFunDuplicate,
			"updtfunc": ActionUpdateFunc(func(tvi any, act *gi.Action) {
				tv := tvi.(ki.Ki).Embed(TypeTreeView).(*TreeView)
				act.SetState(tv.IsRootOrField(""), states.Disabled)
			}),
		}},
		{"SrcDelete", ki.Props{
			"label":    "Delete",
			"shortcut": gi.KeyFunDelete,
			"updtfunc": ActionUpdateFunc(func(tvi any, act *gi.Action) {
				tv := tvi.(ki.Ki).Embed(TypeTreeView).(*TreeView)
				act.SetState(tv.IsRootOrField(""), states.Disabled)
			}),
		}},
		{"sep-edit", ki.BlankProp{}},
		{"Copy", ki.Props{
			"shortcut": gi.KeyFunCopy,
			"Args": ki.PropSlice{
				{"reset", ki.Props{
					"value": true,
				}},
			},
		}},
		{"Cut", ki.Props{
			"shortcut": gi.KeyFunCut,
			"updtfunc": ActionUpdateFunc(func(tvi any, act *gi.Action) {
				tv := tvi.(ki.Ki).Embed(TypeTreeView).(*TreeView)
				act.SetState(tv.IsRootOrField(""), states.Disabled)
			}),
		}},
		{"Paste", ki.Props{
			"shortcut": gi.KeyFunPaste,
		}},
		{"sep-win", ki.BlankProp{}},
		{"SrcEdit", ki.Props{
			"label": "Edit",
		}},
		{"SrcGoGiEditor", ki.Props{
			"label": "GoGi Editor",
		}},
		{"sep-open", ki.BlankProp{}},
		{"OpenAll", ki.Props{}},
		{"CloseAll", ki.Props{}},
	},
	"CtxtMenuInactive": ki.PropSlice{
		{"Copy", ki.Props{
			"shortcut": gi.KeyFunCopy,
			"Args": ki.PropSlice{
				{"reset", ki.Props{
					"value": true,
				}},
			},
		}},
		{"SrcEdit", ki.Props{
			"label": "Edit",
		}},
		{"SrcGoGiEditor", ki.Props{
			"label": "GoGi Editor",
		}},
	},
}

func (tv *TreeView) ConfigWidget(sc *gi.Scene) {
	// // optimized init -- avoid tree walking
	if tv.RootView != tv {
		tv.Scene = tv.RootView.Scene
	} else {
		tv.Scene = tv.ParentScene()
	}
	tv.Style.Defaults()
	tv.Style.Template = "giv.TreeView." + ki.Type(tv).Name()
	tv.LayState.Defaults() // doesn't overwrite
	tv.ConfigParts(sc)
	// tv.ConnectToScene()
}

func (tv *TreeView) StyleTreeView() {
	tv.UpdateInactive()
	if !tv.HasChildren() {
		tv.SetClosed()
	}
	if tv.HasClosedParent() {
		tv.ClearFlag(int(gi.CanFocus))
		return
	}
	tv.StyMu.Lock()
	tv.SetCanFocusIfActive()
	hasTempl, saveTempl := false, false
	_, noTempl := tv.PropInherit("no-templates", ki.NoInherit, ki.TypeProps)
	tv.SetFlagState(noTempl, int(TreeViewFlagNoTemplate))
	if !noTempl {
		hasTempl, saveTempl = tv.Style.FromTemplate()
	}
	// STYTODO: figure out better way to handle styling (we can't just cache with style funcs)
	if !hasTempl || saveTempl {
		tv.ApplyStyleWidget()
	}
	if hasTempl && saveTempl {
		tv.Style.SaveTemplate()
	}
	tv.Indent.ToDots(&tv.Style.UnContext)
	tv.Parts.Style.InheritFields(&tv.Style)
	tv.StyMu.Unlock()
	tv.ConfigParts(sc)
}

func (tv *TreeView) ApplyStyle(sc *gi.Scene) {
	tv.StyleTreeView()
}

// TreeView is tricky for alloc because it is both a layout of its children but has to
// maintain its own bbox for its own widget.

func (tv *TreeView) GetSize(sc *gi.Scene, iter int) {
	tv.InitLayout(sc)
	if tv.HasClosedParent() {
		return // nothing
	}
	tv.GetSizeParts(sc, iter) // get our size from parts
	tv.WidgetSize = tv.LayState.Alloc.Size
	h := mat32.Ceil(tv.WidgetSize.Y)
	w := tv.WidgetSize.X

	if !tv.IsClosed() {
		// we layout children under us
		for _, kid := range tv.Kids {
			gis := kid.(gi.Widget).AsWidget()
			if gis == nil || gis.This() == nil {
				continue
			}
			h += mat32.Ceil(gis.LayState.Alloc.Size.Y)
			w = mat32.Max(w, tv.Indent.Dots+gis.LayState.Alloc.Size.X)
		}
	}
	tv.LayState.Alloc.Size = mat32.Vec2{w, h}
	tv.WidgetSize.X = w // stretch
}

func (tv *TreeView) DoLayoutParts(parBBox image.Rectangle, iter int) {
	spc := tv.Style.BoxSpace()
	tv.Parts.LayState.Alloc.Pos = tv.LayState.Alloc.Pos.Add(spc.Pos())
	tv.Parts.LayState.Alloc.PosOrig = tv.Parts.LayState.Alloc.Pos
	tv.Parts.LayState.Alloc.Size = tv.WidgetSize.Sub(spc.Size())
	tv.Parts.DoLayout(sc, parBBox, iter)
}

func (tv *TreeView) DoLayout(sc *gi.Scene, parBBox image.Rectangle, iter int) bool {
	if tv.HasClosedParent() {
		tv.LayState.Alloc.PosRel.X = -1000000 // put it very far off screen..
	}

	psize := tv.AddParentPos() // have to add our pos first before computing below:

	rn := tv.RootView
	// our alloc size is root's size minus our total indentation
	tv.LayState.Alloc.Size.X = rn.LayState.Alloc.Size.X - (tv.LayState.Alloc.Pos.X - rn.LayState.Alloc.Pos.X)
	tv.WidgetSize.X = tv.LayState.Alloc.Size.X

	tv.LayState.Alloc.PosOrig = tv.LayState.Alloc.Pos
	gi.SetUnitContext(&tv.Style, tv.Scene, tv.NodeSize(), psize) // update units with final layout
	tv.BBox = tv.This().(gi.Widget).BBoxes()                     // only compute once, at this point
	tv.This().(gi.Widget).ComputeBBoxes(sc, parBBox, image.Point{})

	if gi.LayoutTrace {
		fmt.Printf("Layout: %v reduced X allocsize: %v rn: %v  pos: %v rn pos: %v\n", tv.Path(), tv.WidgetSize.X, rn.LayState.Alloc.Size.X, tv.LayState.Alloc.Pos.X, rn.LayState.Alloc.Pos.X)
		fmt.Printf("Layout: %v alloc pos: %v size: %v scbb: %v winbb: %v\n", tv.Path(), tv.LayState.Alloc.Pos, tv.LayState.Alloc.Size, tv.ScBBox, tv.WinBBox)
	}

	tv.DoLayoutParts(parBBox, iter) // use OUR version
	h := mat32.Ceil(tv.WidgetSize.Y)
	if !tv.IsClosed() {
		for _, kid := range tv.Kids {
			if kid == nil || kid.This() == nil {
				continue
			}
			ni := kid.(gi.Widget).AsWidget()
			if ni == nil {
				continue
			}
			ni.LayState.Alloc.PosRel.Y = h
			ni.LayState.Alloc.PosRel.X = tv.Indent.Dots
			h += mat32.Ceil(ni.LayState.Alloc.Size.Y)
		}
	}
	return tv.DoLayoutChildren(iter)
}

func (tv *TreeView) BBoxes() image.Rectangle {
	// we have unusual situation of bbox != alloc
	tp := tv.LayState.Alloc.PosOrig.ToPointFloor()
	ts := tv.WidgetSize.ToPointCeil()
	return image.Rect(tp.X, tp.Y, tp.X+ts.X, tp.Y+ts.Y)
}

func (tv *TreeView) ChildrenBBoxes(sc *gi.Scene) image.Rectangle {
	ar := tv.BBoxFromAlloc() // need to use allocated size which includes children
	if tv.Par != nil {       // use parents children bbox to determine where we can draw
		pwi, _ := gi.AsWidget(tv.Par)
		ar = ar.Intersect(pwi.ChildrenBBoxes(sc))
	}
	return ar
}

func (tv *TreeView) IsVisible() bool {
	if tv == nil || tv.This() == nil || tv.Scene == nil {
		return false
	}
	if tv.RootView == nil || tv.RootView.This() == nil {
		return false
	}
	if tv.RootView.Par == nil || tv.RootView.Par.This() == nil {
		return false
	}
	if tv.This() == tv.RootView.This() { // root is ALWAYS visible so updates there work
		return true
	}
	if tv.HasFlag(Invisible) {
		return false
	}
	return tv.RootView.Par.This().(gi.Widget).IsVisible()
}

func (tv *TreeView) PushBounds() bool {
	if tv == nil || tv.This() == nil {
		return false
	}
	if !tv.This().(gi.Widget).IsVisible() {
		return false
	}
	if tv.ScBBox.Empty() && tv.This() != tv.RootView.This() { // root must always connect!
		tv.ClearFullReRender()
		return false
	}
	rs := tv.Render()
	rs.PushBounds(tv.ScBBox)
	tv.ConnectToScene()
	if gi.RenderTrace {
		fmt.Printf("Render: %v at %v\n", tv.Path(), tv.ScBBox)
	}
	return true
}

func (tv *TreeView) Render(sc *gi.Scene) {
	if tv.HasClosedParent() {
		return // nothing
	}
	// restyle on re-render -- this is not actually necessary
	// if tv.HasFlag(int(TreeViewFlagNoTemplate)) && (tv.NeedsFullReRender() || tv.RootView.NeedsFullReRender()) {
	// 	fmt.Printf("restyle: %v\n", tv.Nm)
	// 	tv.StyleTreeView()
	// 	tv.ConfigParts(vp)
	// }
	// fmt.Printf("tv rend: %v\n", tv.Nm)
	if tv.PushBounds() {
		if !tv.ScBBox.Empty() { // we are root and just here for the connections :)
			tv.UpdateInactive()
			// if tv.StateIs(states.Selected) {
			// 	tv.Style = tv.StateStyles[TreeViewSel]
			// } else if tv.StateIs(states.Focused) {
			// 	tv.Style = tv.StateStyles[TreeViewFocus]
			// } else if tv.IsDisabled() {
			// 	tv.Style = tv.StateStyles[TreeViewInactive]
			// } else {
			// 	tv.Style = tv.StateStyles[TreeViewActive]
			// }
			tv.This().(gi.Widget).SetTypeHandlers()

			// note: this is std except using WidgetSize instead of AllocSize
			rs, pc, st := tv.RenderLock(sc)
			pc.FontStyle = *st.FontRender()
			// SidesTODO: look here if tree view borders break
			// pc.StrokeStyle.SetColor(&st.Border.Color)
			// pc.StrokeStyle.Width = st.Border.Width
			bg := st.BackgroundColor
			if bg.IsNil() {
				bg = tv.ParentBackgroundColor()
			}
			pc.FillStyle.SetFullColor(&bg)
			// tv.RenderStdBox()
			pos := tv.LayState.Alloc.Pos.Add(st.TotalMargin().Pos())
			sz := tv.WidgetSize.Sub(st.TotalMargin().Size())
			tv.RenderBoxImpl(pos, sz, st.Border)
			tv.RenderUnlock(rs)
			tv.RenderParts()
		}
		tv.PopBounds()
	}
	// we always have to render our kids b/c we could be out of scope but they could be in!
	tv.RenderChildren()
	tv.ClearFullReRender()
}

func (tv *TreeView) SetTypeHandlers() {
	tv.TreeViewEvents()
}

//
// func (tv *TreeView) FocusChanged(change gi.FocusChanges) {
// 	switch change {
// 	case gi.FocusLost:
// 		tv.UpdateSig()
// 	case gi.FocusGot:
// 		if tv.This() == tv.RootView.This() {
// 			sl := tv.SelectedViews()
// 			if len(sl) > 0 {
// 				fsl := sl[0]
// 				if fsl != tv {
// 					fsl.GrabFocus()
// 					return
// 				}
// 			}
// 		}
// 		tv.ScrollToMe()
// 		tv.EmitFocusedSignal()
// 		tv.UpdateSig()
// 	case gi.FocusInactive: // don't care..
// 	case gi.FocusActive:
// 	}
// }
