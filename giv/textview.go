// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"goki.dev/colors"
	"goki.dev/cursors"
	"goki.dev/gi/v2/giv/textbuf"
	"goki.dev/gi/v2/histyle"
	"goki.dev/girl/paint"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/goosi"
	"goki.dev/laser"
	"goki.dev/mat32/v2"

	"goki.dev/gi/v2/gi"
	"goki.dev/girl/units"
	"goki.dev/glop/indent"
	"goki.dev/goosi/events"
	"goki.dev/goosi/events/key"
	"goki.dev/goosi/mimedata"
	"goki.dev/ki/v2"
	"goki.dev/pi/v2/filecat"
	"goki.dev/pi/v2/lex"
	"goki.dev/pi/v2/pi"
	"goki.dev/pi/v2/token"
)

// TextView is a widget for editing multiple lines of text (as compared to
// TextField for a single line).  The View is driven by a TextBuf buffer which
// contains all the text, and manages all the edits, sending update signals
// out to the views -- multiple views can be attached to a given buffer.  All
// updating in the TextView should be within a single goroutine -- it would
// require extensive protections throughout code otherwise.
type TextView struct {
	gi.WidgetBase

	// the text buffer that we're editing
	Buf *TextBuf `json:"-" xml:"-" desc:"the text buffer that we're editing"`

	// text that is displayed when the field is empty, in a lower-contrast manner
	Placeholder string `json:"-" xml:"placeholder" desc:"text that is displayed when the field is empty, in a lower-contrast manner"`

	// width of cursor -- set from cursor-width property (inherited)
	CursorWidth units.Value `xml:"cursor-width" desc:"width of cursor -- set from cursor-width property (inherited)"`

	// the color used for the side bar containing the line numbers; this should be set in Stylers like all other style properties
	LineNumberColor colors.Full `desc:"the color used for the side bar containing the line numbers; this should be set in Stylers like all other style properties"`

	// the color used for the user text selection background color; this should be set in Stylers like all other style properties
	SelectColor colors.Full `desc:"the color used for the user text selection background color; this should be set in Stylers like all other style properties"`

	// the color used for the text highlight background color (like in find); this should be set in Stylers like all other style properties
	HighlightColor colors.Full `desc:"the color used for the text highlight background color (like in find); this should be set in Stylers like all other style properties"`

	// the color used for the text field cursor (caret); this should be set in Stylers like all other style properties
	CursorColor colors.Full `desc:"the color used for the text field cursor (caret); this should be set in Stylers like all other style properties"`

	// number of lines in the view -- sync'd with the Buf after edits, but always reflects storage size of Renders etc
	NLines int `json:"-" xml:"-" desc:"number of lines in the view -- sync'd with the Buf after edits, but always reflects storage size of Renders etc"`

	// renders of the text lines, with one render per line (each line could visibly wrap-around, so these are logical lines, not display lines)
	Renders []paint.Text `json:"-" xml:"-" desc:"renders of the text lines, with one render per line (each line could visibly wrap-around, so these are logical lines, not display lines)"`

	// starting offsets for top of each line
	Offs []float32 `json:"-" xml:"-" desc:"starting offsets for top of each line"`

	// number of line number digits needed
	LineNoDigs int `json:"-" xml:"-" desc:"number of line number digits needed"`

	// horizontal offset for start of text after line numbers
	LineNoOff float32 `json:"-" xml:"-" desc:"horizontal offset for start of text after line numbers"`

	// render for line numbers
	LineNoRender paint.Text `json:"-" xml:"-" desc:"render for line numbers"`

	// total size of all lines as rendered
	LinesSize image.Point `json:"-" xml:"-" desc:"total size of all lines as rendered"`

	// size params to use in render call
	RenderSz mat32.Vec2 `json:"-" xml:"-" desc:"size params to use in render call"`

	// current cursor position
	CursorPos lex.Pos `json:"-" xml:"-" desc:"current cursor position"`

	// desired cursor column -- where the cursor was last when moved using left / right arrows -- used when doing up / down to not always go to short line columns
	CursorCol int `json:"-" xml:"-" desc:"desired cursor column -- where the cursor was last when moved using left / right arrows -- used when doing up / down to not always go to short line columns"`

	// if true, scroll screen to cursor on next render
	ScrollToCursorOnRender bool `json:"-" xml:"-" desc:"if true, scroll screen to cursor on next render"`

	// cursor position to scroll to
	ScrollToCursorPos lex.Pos `json:"-" xml:"-" desc:"cursor position to scroll to"`

	// current index within PosHistory
	PosHistIdx int `json:"-" xml:"-" desc:"current index within PosHistory"`

	// starting point for selection -- will either be the start or end of selected region depending on subsequent selection.
	SelectStart lex.Pos `json:"-" xml:"-" desc:"starting point for selection -- will either be the start or end of selected region depending on subsequent selection."`

	// current selection region
	SelectReg textbuf.Region `json:"-" xml:"-" desc:"current selection region"`

	// previous selection region, that was actually rendered -- needed to update render
	PrevSelectReg textbuf.Region `json:"-" xml:"-" desc:"previous selection region, that was actually rendered -- needed to update render"`

	// highlighted regions, e.g., for search results
	Highlights []textbuf.Region `json:"-" xml:"-" desc:"highlighted regions, e.g., for search results"`

	// highlighted regions, specific to scope markers
	Scopelights []textbuf.Region `json:"-" xml:"-" desc:"highlighted regions, specific to scope markers"`

	// if true, select text as cursor moves
	SelectMode bool `json:"-" xml:"-" desc:"if true, select text as cursor moves"`

	// if true, complete regardless of any disqualifying reasons
	ForceComplete bool `json:"-" xml:"-" desc:"if true, complete regardless of any disqualifying reasons"`

	// interactive search data
	ISearch ISearch `json:"-" xml:"-" desc:"interactive search data"`

	// query replace data
	QReplace QReplace `json:"-" xml:"-" desc:"query replace data"`

	// font height, cached during styling
	FontHeight float32 `json:"-" xml:"-" desc:"font height, cached during styling"`

	// line height, cached during styling
	LineHeight float32 `json:"-" xml:"-" desc:"line height, cached during styling"`

	// height in lines and width in chars of the visible area
	VisSize image.Point `json:"-" xml:"-" desc:"height in lines and width in chars of the visible area"`

	// oscillates between on and off for blinking
	BlinkOn bool `json:"-" xml:"-" desc:"oscillates between on and off for blinking"`

	// [view: -] mutex protecting cursor rendering -- shared between blink and main code
	CursorMu sync.Mutex `json:"-" xml:"-" view:"-" desc:"mutex protecting cursor rendering -- shared between blink and main code"`

	// at least one of the renders has links -- determines if we set the cursor for hand movements
	HasLinks       bool `json:"-" xml:"-" desc:"at least one of the renders has links -- determines if we set the cursor for hand movements"`
	lastRecenter   int
	lastAutoInsert rune
	lastFilename   gi.FileName
}

// NewTextViewLayout adds a new layout with textview
// to given parent node, with given name.  Layout adds "-lay" suffix.
// Textview should always have a parent Layout to manage
// the scrollbars.
func NewTextViewLayout(parent ki.Ki, name string) (*TextView, *gi.Layout) {
	ly := parent.NewChild(gi.LayoutType, name+"-lay").(*gi.Layout)
	tv := NewTextView(ly, name)
	return tv, ly
}

func (tv *TextView) OnInit() {
	tv.TextViewEvents()
	tv.TextViewStyles()
}

func (tv *TextView) TextViewStyles() {
	tv.AddStyles(func(s *styles.Style) {
		tv.CursorWidth.SetDp(1)
		tv.LineNumberColor.SetSolid(colors.Scheme.SurfaceContainerHighest)
		tv.SelectColor.SetSolid(colors.Scheme.TertiaryContainer)
		tv.HighlightColor.SetSolid(colors.Orange)
		tv.CursorColor.SetSolid(colors.Scheme.OnSurface)

		s.Cursor = cursors.Text
		if gi.Prefs.Editor.WordWrap {
			s.Text.WhiteSpace = styles.WhiteSpacePreWrap
		} else {
			s.Text.WhiteSpace = styles.WhiteSpacePre
		}
		s.Border.Style.Set(styles.BorderNone) // don't render our own border
		s.Border.Radius = styles.BorderRadiusLarge
		s.Margin.Set()
		s.Padding.Set(units.Dp(4))
		s.AlignV = styles.AlignTop
		s.Text.Align = styles.AlignLeft
		s.Text.TabSize = 4
		s.Color = colors.Scheme.OnSurface

		if w.StateIs(states.Focused) {
			s.BackgroundColor.SetSolid(colors.Scheme.Surface)
		} else {
			s.BackgroundColor.SetSolid(colors.Scheme.SurfaceContainerHigh)
		}
	})
}

// TextViewSignals are signals that text view can send
type TextViewSignals int64 //enums:enum

const (
	// TextViewDone signal indicates return was pressed and an edit was completed -- data is the text
	TextViewDone TextViewSignals = iota

	// TextViewSelected signal indicates some text was selected (for Inactive state, selection is via WidgetSig)
	TextViewSelected

	// TextViewCursorMoved signal indicates cursor moved emitted for every cursor movement -- e.g., for displaying cursor pos
	TextViewCursorMoved

	// TextViewISearch is emitted for every update of interactive search process -- see
	// ISearch.* members for current state
	TextViewISearch

	// TextViewQReplace is emitted for every update of query-replace process -- see
	// QReplace.* members for current state
	TextViewQReplace
)

// TextViewStates are mutually-exclusive textfield states -- determines appearance
type TextViewStates int32 //enums:enum

const (
	// TextViewActive is the normal state -- there but not being interacted with
	TextViewActive TextViewStates = iota

	// TextViewFocus states means textvieww is the focus -- will respond to keyboard input
	TextViewFocus

	// TextViewInactive means the textview is inactive -- not editable
	TextViewInactive

	// TextViewSel means the text region is selected
	TextViewSel

	// TextViewHighlight means the text region is highlighted
	TextViewHighlight
)

// Style selector names for the different states
var TextViewSelectors = []string{":active", ":focus", ":inactive", ":selected", ":highlight"}

// TextViewFlags extend NodeBase NodeFlags to hold TextView state
type TextViewFlags ki.Flags //enums:bitflag

const (
	// TextViewNeedsRefresh indicates when refresh is required
	TextViewNeedsRefresh TextViewFlags = TextViewFlags(gi.WidgetFlagsN) + iota

	// TextViewInReLayout indicates that we are currently resizing ourselves via parent layout
	TextViewInReLayout

	// TextViewRenderScrolls indicates that parent layout scrollbars need to be re-rendered at next rerender
	TextViewRenderScrolls

	// TextViewFocusActive is set if the keyboard focus is active -- when we lose active focus we apply changes
	TextViewFocusActive

	// TextViewHasLineNos indicates that this view has line numbers (per TextBuf option)
	TextViewHasLineNos

	// TextViewLastWasTabAI indicates that last key was a Tab auto-indent
	TextViewLastWasTabAI

	// TextViewLastWasUndo indicates that last key was an undo
	TextViewLastWasUndo
)

// IsFocusActive returns true if we have active focus for keyboard input
func (tv *TextView) IsFocusActive() bool {
	return tv.HasFlag(int(TextViewFocusActive))
}

// EditDone completes editing and copies the active edited text to the text --
// called when the return key is pressed or goes out of focus
func (tv *TextView) EditDone() {
	if tv.Buf != nil {
		tv.Buf.EditDone()
	}
	tv.ClearSelected()
}

// Refresh re-displays everything anew from the buffer
func (tv *TextView) Refresh() {
	if tv == nil || tv.This() == nil {
		return
	}
	if !tv.This().(gi.Widget).IsVisible() {
		return
	}
	tv.LayoutAllLines(false)
	tv.RenderAllLines()
	tv.ClearNeedsRefresh()
}

// Remarkup triggers a complete re-markup of the entire text --
// can do this when needed if the markup gets off due to multi-line
// formatting issues -- via Recenter key
func (tv *TextView) ReMarkup() {
	if tv.Buf == nil {
		return
	}
	tv.Buf.ReMarkup()
}

// NeedsRefresh checks if a refresh is required -- atomically safe for other
// routines to set the NeedsRefresh flag
func (tv *TextView) NeedsRefresh() bool {
	return tv.HasFlag(int(TextViewNeedsRefresh))
}

// SetNeedsRefresh flags that a refresh is required -- atomically safe for
// other routines to call this
func (tv *TextView) SetNeedsRefresh() {
	tv.SetFlag(int(TextViewNeedsRefresh))
}

// ClearNeedsRefresh clears needs refresh flag -- atomically safe
func (tv *TextView) ClearNeedsRefresh() {
	tv.ClearFlag(int(TextViewNeedsRefresh))
}

// RefreshIfNeeded re-displays everything if SetNeedsRefresh was called --
// returns true if refreshed
func (tv *TextView) RefreshIfNeeded() bool {
	if tv.NeedsRefresh() {
		tv.Refresh()
		return true
	}
	return false
}

// IsChanged returns true if buffer was changed (edited)
func (tv *TextView) IsChanged() bool {
	if tv.Buf != nil && tv.Buf.IsChanged() {
		return true
	}
	return false
}

// HasLineNos returns true if view is showing line numbers (per textbuf option, cached here)
func (tv *TextView) HasLineNos() bool {
	return tv.HasFlag(int(TextViewHasLineNos))
}

// Clear resets all the text in the buffer for this view
func (tv *TextView) Clear() {
	if tv.Buf == nil {
		return
	}
	tv.Buf.New(0)
}

///////////////////////////////////////////////////////////////////////////////
//  Buffer communication

// ResetState resets all the random state variables, when opening a new buffer etc
func (tv *TextView) ResetState() {
	tv.SelectReset()
	tv.Highlights = nil
	tv.ISearch.On = false
	tv.QReplace.On = false
	if tv.Buf == nil || tv.lastFilename != tv.Buf.Filename { // don't reset if reopening..
		tv.CursorPos = lex.Pos{}
	}
	if tv.Buf != nil {
		tv.Buf.SetInactive(tv.IsDisabled())
	}
}

// SetBuf sets the TextBuf that this is a view of, and interconnects their signals
func (tv *TextView) SetBuf(buf *TextBuf) {
	if buf != nil && tv.Buf == buf {
		return
	}
	// had := false
	if tv.Buf != nil {
		// had = true
		tv.Buf.DeleteView(tv)
	}
	tv.Buf = buf
	tv.ResetState()
	if buf != nil {
		buf.AddView(tv)
		bhl := len(buf.PosHistory)
		if bhl > 0 {
			tv.CursorPos = buf.PosHistory[bhl-1]
			tv.PosHistIdx = bhl - 1
		}
	}
	tv.LayoutAllLines(false)
	tv.SetFullReRender()
	tv.UpdateSig()
	tv.SetCursorShow(tv.CursorPos)
}

// LinesInserted inserts new lines of text and reformats them
func (tv *TextView) LinesInserted(tbe *textbuf.Edit) {
	stln := tbe.Reg.Start.Ln + 1
	nsz := (tbe.Reg.End.Ln - tbe.Reg.Start.Ln)
	if stln > len(tv.Renders) { // invalid
		return
	}

	// Renders
	tmprn := make([]paint.Text, nsz)
	nrn := append(tv.Renders, tmprn...)
	copy(nrn[stln+nsz:], nrn[stln:])
	copy(nrn[stln:], tmprn)
	tv.Renders = nrn

	// Offs
	tmpof := make([]float32, nsz)
	nof := append(tv.Offs, tmpof...)
	copy(nof[stln+nsz:], nof[stln:])
	copy(nof[stln:], tmpof)
	tv.Offs = nof

	tv.NLines += nsz

	tv.LayoutLines(tbe.Reg.Start.Ln, tbe.Reg.End.Ln, false)
	tv.RenderAllLines()
}

// LinesDeleted deletes lines of text and reformats remaining one
func (tv *TextView) LinesDeleted(tbe *textbuf.Edit) {
	stln := tbe.Reg.Start.Ln
	edln := tbe.Reg.End.Ln
	dsz := edln - stln

	tv.Renders = append(tv.Renders[:stln], tv.Renders[edln:]...)
	tv.Offs = append(tv.Offs[:stln], tv.Offs[edln:]...)

	tv.NLines -= dsz

	tv.LayoutLines(tbe.Reg.Start.Ln, tbe.Reg.Start.Ln, true)
	tv.RenderAllLines()
}

// TextViewBufSigRecv receives a signal from the buffer and updates view accordingly
func TextViewBufSigRecv(rvwki ki.Ki, sbufki ki.Ki, sig int64, data any) {
	tv := rvwki.Embed(TypeTextView).(*TextView)
	switch TextBufSignals(sig) {
	case TextBufDone:
	case TextBufNew:
		tv.ResetState()
		tv.SetNeedsRefresh() // in case not visible
		tv.Refresh()
		tv.SetCursorShow(tv.CursorPos)
	case TextBufInsert:
		if tv.Renders == nil || !tv.This().(gi.Widget).IsVisible() {
			return
		}
		tbe := data.(*textbuf.Edit)
		// fmt.Printf("tv %v got %v\n", tv.Nm, tbe.Reg.Start)
		if tbe.Reg.Start.Ln != tbe.Reg.End.Ln {
			// fmt.Printf("tv %v lines insert %v - %v\n", tv.Nm, tbe.Reg.Start, tbe.Reg.End)
			tv.LinesInserted(tbe)
		} else {
			rerend := tv.LayoutLines(tbe.Reg.Start.Ln, tbe.Reg.End.Ln, false)
			if rerend {
				// fmt.Printf("tv %v line insert rerend %v - %v\n", tv.Nm, tbe.Reg.Start, tbe.Reg.End)
				tv.RenderAllLines()
			} else {
				// fmt.Printf("tv %v line insert no rerend %v - %v.  markup: %v\n", tv.Nm, tbe.Reg.Start, tbe.Reg.End, len(tv.Buf.HiTags[tbe.Reg.Start.Ln]))
				tv.RenderLines(tbe.Reg.Start.Ln, tbe.Reg.End.Ln)
			}
		}
	case TextBufDelete:
		if tv.Renders == nil || !tv.This().(gi.Widget).IsVisible() {
			return
		}
		tbe := data.(*textbuf.Edit)
		if tbe.Reg.Start.Ln != tbe.Reg.End.Ln {
			tv.LinesDeleted(tbe)
		} else {
			rerend := tv.LayoutLines(tbe.Reg.Start.Ln, tbe.Reg.End.Ln, true)
			if rerend {
				tv.RenderAllLines()
			} else {
				tv.RenderLines(tbe.Reg.Start.Ln, tbe.Reg.End.Ln)
			}
		}
	case TextBufMarkUpdt:
		tv.SetNeedsRefresh() // comes from another goroutine
	case TextBufClosed:
		tv.SetBuf(nil)
	}
}

///////////////////////////////////////////////////////////////////////////////
//  Text formatting and rendering

// ParentLayout returns our parent layout.
// we ensure this is our immediate parent which is necessary for textview
func (tv *TextView) ParentLayout() *gi.Layout {
	if tv.Par == nil {
		return nil
	}
	return AsLayout(tv.Par)
}

// RenderSize is the size we should pass to text rendering, based on alloc
func (tv *TextView) RenderSize() mat32.Vec2 {
	spc := tv.Style.BoxSpace()
	if tv.Par == nil {
		return mat32.Vec2Zero
	}
	parw := tv.ParentLayout()
	if parw == nil {
		log.Printf("giv.TextView Programmer Error: A TextView MUST be located within a parent Layout object -- instead parent is %v at: %v\n", ki.Type(tv.Par), tv.Path())
		return mat32.Vec2Zero
	}
	paloc := parw.LayState.Alloc.SizeOrig
	if !paloc.IsNil() {
		// fmt.Printf("paloc: %v, psc: %v  lineonoff: %v\n", paloc, parw.ScBBox, tv.LineNoOff)
		tv.RenderSz = paloc.Sub(parw.ExtraSize).Sub(spc.Size())
		// SidesTODO: this is sketchy
		tv.RenderSz.X -= spc.Size().X / 2 // extra space
		// fmt.Printf("alloc rendersz: %v\n", tv.RenderSz)
	} else {
		sz := tv.LayState.Alloc.SizeOrig
		if sz.IsNil() {
			sz = tv.LayState.SizePrefOrMax()
		}
		if !sz.IsNil() {
			sz.SetSub(spc.Size())
		}
		tv.RenderSz = sz
		// fmt.Printf("fallback rendersz: %v\n", tv.RenderSz)
	}
	tv.RenderSz.X -= tv.LineNoOff
	// fmt.Printf("rendersz: %v\n", tv.RenderSz)
	return tv.RenderSz
}

// HiStyle applies the highlighting styles from buffer markup style
func (tv *TextView) HiStyle() {
	// STYTODO: need to figure out what to do with histyle
	if !tv.Buf.Hi.HasHi() {
		return
	}
	tv.CSS = tv.Buf.Hi.CSSProps
	if chp, ok := ki.SubProps(tv.CSS, ".chroma"); ok {
		for ky, vl := range chp { // apply to top level
			tv.SetProp(ky, vl)
		}
	}
}

// LayoutAllLines generates TextRenders of lines from our TextBuf, from the
// Markup version of the source, and returns whether the current rendered size
// is different from what it was previously
func (tv *TextView) LayoutAllLines(inLayout bool) bool {
	if inLayout && tv.HasFlag(int(TextViewInReLayout)) {
		return false
	}
	if tv.Buf == nil || tv.Buf.NumLines() == 0 {
		tv.NLines = 0
		return tv.ResizeIfNeeded(image.Point{})
	}
	tv.StyMu.RLock()
	needSty := tv.Style.Font.Size.Val == 0
	tv.StyMu.RUnlock()
	if needSty {
		// fmt.Print("textview: no style\n")
		return false
		// tv.StyleTextView() // this fails on mac
	}
	tv.lastFilename = tv.Buf.Filename

	tv.Buf.Hi.TabSize = tv.Style.Text.TabSize
	tv.HiStyle()
	// fmt.Printf("layout all: %v\n", tv.Nm)

	tv.NLines = tv.Buf.NumLines()
	nln := tv.NLines
	if cap(tv.Renders) >= nln {
		tv.Renders = tv.Renders[:nln]
	} else {
		tv.Renders = make([]paint.Text, nln)
	}
	if cap(tv.Offs) >= nln {
		tv.Offs = tv.Offs[:nln]
	} else {
		tv.Offs = make([]float32, nln)
	}

	tv.VisSizes()
	sz := tv.RenderSz

	// fmt.Printf("rendersize: %v\n", sz)
	sty := &tv.Style
	fst := sty.FontRender()
	fst.BackgroundColor.SetColor(nil)
	off := float32(0)
	mxwd := sz.X // always start with our render size

	tv.Buf.MarkupMu.RLock()
	tv.HasLinks = false
	for ln := 0; ln < nln; ln++ {
		tv.Renders[ln].SetHTMLPre(tv.Buf.Markup[ln], fst, &sty.Text, &sty.UnContext, tv.CSS)
		tv.Renders[ln].LayoutStdLR(&sty.Text, sty.FontRender(), &sty.UnContext, sz)
		if !tv.HasLinks && len(tv.Renders[ln].Links) > 0 {
			tv.HasLinks = true
		}
		tv.Offs[ln] = off
		lsz := mat32.Max(tv.Renders[ln].Size.Y, tv.LineHeight)
		off += lsz
		mxwd = mat32.Max(mxwd, tv.Renders[ln].Size.X)
	}
	tv.Buf.MarkupMu.RUnlock()

	extraHalf := tv.LineHeight * 0.5 * float32(tv.VisSize.Y)
	nwSz := mat32.Vec2{mxwd, off + extraHalf}.ToPointCeil()
	// fmt.Printf("lay lines: diff: %v  old: %v  new: %v\n", diff, tv.LinesSize, nwSz)
	if inLayout {
		tv.LinesSize = nwSz
		return tv.SetSize()
	}
	return tv.ResizeIfNeeded(nwSz)
}

// SetSize updates our size only if larger than our allocation
func (tv *TextView) SetSize() bool {
	sty := &tv.Style
	spc := sty.BoxSpace()
	rndsz := tv.RenderSz
	rndsz.X += tv.LineNoOff
	netsz := mat32.Vec2{float32(tv.LinesSize.X) + tv.LineNoOff, float32(tv.LinesSize.Y)}
	cursz := tv.LayState.Alloc.Size.Sub(spc.Size())
	if cursz.X < 10 || cursz.Y < 10 {
		nwsz := netsz.Max(rndsz)
		tv.GetSizeFromWH(nwsz.X, nwsz.Y)
		tv.LayState.Size.Need = tv.LayState.Alloc.Size
		tv.LayState.Size.Pref = tv.LayState.Alloc.Size
		return true
	}
	nwsz := netsz.Max(rndsz)
	alloc := tv.LayState.Alloc.Size
	tv.GetSizeFromWH(nwsz.X, nwsz.Y)
	if alloc != tv.LayState.Alloc.Size {
		tv.LayState.Size.Need = tv.LayState.Alloc.Size
		tv.LayState.Size.Pref = tv.LayState.Alloc.Size
		return true
	}
	// fmt.Printf("NO resize: netsz: %v  cursz: %v rndsz: %v\n", netsz, cursz, rndsz)
	return false
}

// ResizeIfNeeded resizes the edit area if different from current setting --
// returns true if resizing was performed
func (tv *TextView) ResizeIfNeeded(nwSz image.Point) bool {
	if nwSz == tv.LinesSize {
		return false
	}
	// fmt.Printf("%v needs resize: %v\n", tv.Nm, nwSz)
	tv.LinesSize = nwSz
	diff := tv.SetSize()
	if !diff {
		// fmt.Printf("%v resize no setsize: %v\n", tv.Nm, nwSz)
		return false
	}
	ly := tv.ParentLayout()
	if ly != nil {
		tv.SetFlag(int(TextViewInReLayout))
		gi.GatherSizes(ly) // can't call GetSize b/c that resets layout
		ly.DoLayoutTree()
		tv.SetFlag(int(TextViewRenderScrolls))
		tv.ClearFlag(int(TextViewInReLayout))
		// fmt.Printf("resized: %v\n", tv.LayState.Alloc.Size)
	}
	return true
}

// LayoutLines generates render of given range of lines (including
// highlighting). end is *inclusive* line.  isDel means this is a delete and
// thus offsets for all higher lines need to be recomputed.  returns true if
// overall number of effective lines (e.g., from word-wrap) is now different
// than before, and thus a full re-render is needed.
func (tv *TextView) LayoutLines(st, ed int, isDel bool) bool {
	if tv.Buf == nil || tv.Buf.NumLines() == 0 {
		return false
	}
	sty := &tv.Style
	fst := sty.FontRender()
	fst.BackgroundColor.SetColor(nil)
	mxwd := float32(tv.LinesSize.X)
	rerend := false

	tv.Buf.MarkupMu.RLock()
	for ln := st; ln <= ed; ln++ {
		curspans := len(tv.Renders[ln].Spans)
		tv.Renders[ln].SetHTMLPre(tv.Buf.Markup[ln], fst, &sty.Text, &sty.UnContext, tv.CSS)
		tv.Renders[ln].LayoutStdLR(&sty.Text, sty.FontRender(), &sty.UnContext, tv.RenderSz)
		if !tv.HasLinks && len(tv.Renders[ln].Links) > 0 {
			tv.HasLinks = true
		}
		nwspans := len(tv.Renders[ln].Spans)
		if nwspans != curspans && (nwspans > 1 || curspans > 1) {
			rerend = true
		}
		mxwd = mat32.Max(mxwd, tv.Renders[ln].Size.X)
	}
	tv.Buf.MarkupMu.RUnlock()

	// update all offsets to end of text
	if rerend || isDel || st != ed {
		ofst := st - 1
		if ofst < 0 {
			ofst = 0
		}
		off := tv.Offs[ofst]
		for ln := ofst; ln < tv.NLines; ln++ {
			tv.Offs[ln] = off
			lsz := mat32.Max(tv.Renders[ln].Size.Y, tv.LineHeight)
			off += lsz
		}
		extraHalf := tv.LineHeight * 0.5 * float32(tv.VisSize.Y)
		nwSz := mat32.Vec2{mxwd, off + extraHalf}.ToPointCeil()
		tv.ResizeIfNeeded(nwSz)
	} else {
		nwSz := mat32.Vec2{mxwd, 0}.ToPointCeil()
		nwSz.Y = tv.LinesSize.Y
		tv.ResizeIfNeeded(nwSz)
	}
	return rerend
}

///////////////////////////////////////////////////////////////////////////////
//  Cursor Navigation

// CursorMovedSig sends the signal that cursor has moved
func (tv *TextView) CursorMovedSig() {
	tv.TextViewSig.Emit(tv.This(), int64(TextViewCursorMoved), tv.CursorPos)
}

// ValidateCursor sets current cursor to a valid cursor position
func (tv *TextView) ValidateCursor() {
	if tv.Buf != nil {
		tv.CursorPos = tv.Buf.ValidPos(tv.CursorPos)
	} else {
		tv.CursorPos = lex.PosZero
	}
}

// WrappedLines returns the number of wrapped lines (spans) for given line number
func (tv *TextView) WrappedLines(ln int) int {
	if ln >= len(tv.Renders) {
		return 0
	}
	return len(tv.Renders[ln].Spans)
}

// WrappedLineNo returns the wrapped line number (span index) and rune index
// within that span of the given character position within line in position,
// and false if out of range (last valid position returned in that case -- still usable).
func (tv *TextView) WrappedLineNo(pos lex.Pos) (si, ri int, ok bool) {
	if pos.Ln >= len(tv.Renders) {
		return 0, 0, false
	}
	return tv.Renders[pos.Ln].RuneSpanPos(pos.Ch)
}

// SetCursor sets a new cursor position, enforcing it in range
func (tv *TextView) SetCursor(pos lex.Pos) {
	if tv.NLines == 0 || tv.Buf == nil {
		tv.CursorPos = lex.PosZero
		return
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	cpln := tv.CursorPos.Ln
	tv.ClearScopelights()
	tv.CursorPos = tv.Buf.ValidPos(pos)
	if cpln != tv.CursorPos.Ln && tv.HasLineNos() { // update cursor position highlight
		rs := tv.Render()
		rs.PushBounds(tv.ScBBox)
		rs.Lock()
		tv.RenderLineNo(cpln, true, true) // render bg, and do vpupload
		tv.RenderLineNo(tv.CursorPos.Ln, true, true)
		rs.Unlock()
		tv.Scene.Render.PopBounds()
	}
	tv.Buf.MarkupLine(tv.CursorPos.Ln)
	tv.CursorMovedSig()
	txt := tv.Buf.Line(tv.CursorPos.Ln)
	ch := tv.CursorPos.Ch
	if ch < len(txt) {
		r := txt[ch]
		if r == '{' || r == '}' || r == '(' || r == ')' || r == '[' || r == ']' {
			tp, found := tv.Buf.BraceMatch(txt[ch], tv.CursorPos)
			if found {
				tv.Scopelights = append(tv.Scopelights, textbuf.NewRegionPos(tv.CursorPos, lex.Pos{tv.CursorPos.Ln, tv.CursorPos.Ch + 1}))
				tv.Scopelights = append(tv.Scopelights, textbuf.NewRegionPos(tp, lex.Pos{tp.Ln, tp.Ch + 1}))
				if tv.CursorPos.Ln < tp.Ln {
					tv.RenderLines(tv.CursorPos.Ln, tp.Ln)
				} else {
					tv.RenderLines(tp.Ln, tv.CursorPos.Ln)
				}
			}
		}
	}
}

// SetCursorShow sets a new cursor position, enforcing it in range, and shows
// the cursor (scroll to if hidden, render)
func (tv *TextView) SetCursorShow(pos lex.Pos) {
	tv.SetCursor(pos)
	tv.ScrollCursorToCenterIfHidden()
	tv.RenderCursor(true)
}

// SetCursorCol sets the current target cursor column (CursorCol) to that
// of the given position
func (tv *TextView) SetCursorCol(pos lex.Pos) {
	if wln := tv.WrappedLines(pos.Ln); wln > 1 {
		si, ri, ok := tv.WrappedLineNo(pos)
		if ok && si > 0 {
			tv.CursorCol = ri
		} else {
			tv.CursorCol = pos.Ch
		}
	} else {
		tv.CursorCol = pos.Ch
	}
}

// SavePosHistory saves the cursor position in history stack of cursor positions
func (tv *TextView) SavePosHistory(pos lex.Pos) {
	if tv.Buf == nil {
		return
	}
	tv.Buf.SavePosHistory(pos)
	tv.PosHistIdx = len(tv.Buf.PosHistory) - 1
}

// CursorToHistPrev moves cursor to previous position on history list --
// returns true if moved
func (tv *TextView) CursorToHistPrev() bool {
	if tv.NLines == 0 || tv.Buf == nil {
		tv.CursorPos = lex.PosZero
		return false
	}
	sz := len(tv.Buf.PosHistory)
	if sz == 0 {
		return false
	}
	tv.PosHistIdx--
	if tv.PosHistIdx < 0 {
		tv.PosHistIdx = 0
		return false
	}
	tv.PosHistIdx = min(sz-1, tv.PosHistIdx)
	pos := tv.Buf.PosHistory[tv.PosHistIdx]
	tv.CursorPos = tv.Buf.ValidPos(pos)
	tv.CursorMovedSig()
	tv.ScrollCursorToCenterIfHidden()
	tv.RenderCursor(true)
	return true
}

// CursorToHistNext moves cursor to previous position on history list --
// returns true if moved
func (tv *TextView) CursorToHistNext() bool {
	if tv.NLines == 0 || tv.Buf == nil {
		tv.CursorPos = lex.PosZero
		return false
	}
	sz := len(tv.Buf.PosHistory)
	if sz == 0 {
		return false
	}
	tv.PosHistIdx++
	if tv.PosHistIdx >= sz-1 {
		tv.PosHistIdx = sz - 1
		return false
	}
	pos := tv.Buf.PosHistory[tv.PosHistIdx]
	tv.CursorPos = tv.Buf.ValidPos(pos)
	tv.CursorMovedSig()
	tv.ScrollCursorToCenterIfHidden()
	tv.RenderCursor(true)
	return true
}

// SelectRegUpdate updates current select region based on given cursor position
// relative to SelectStart position
func (tv *TextView) SelectRegUpdate(pos lex.Pos) {
	if pos.IsLess(tv.SelectStart) {
		tv.SelectReg.Start = pos
		tv.SelectReg.End = tv.SelectStart
	} else {
		tv.SelectReg.Start = tv.SelectStart
		tv.SelectReg.End = pos
	}
}

// CursorSelect updates selection based on cursor movements, given starting
// cursor position and tv.CursorPos is current
func (tv *TextView) CursorSelect(org lex.Pos) {
	if !tv.SelectMode {
		return
	}
	tv.SelectRegUpdate(tv.CursorPos)
	tv.RenderSelectLines()
}

// CursorForward moves the cursor forward
func (tv *TextView) CursorForward(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	for i := 0; i < steps; i++ {
		tv.CursorPos.Ch++
		if tv.CursorPos.Ch > tv.Buf.LineLen(tv.CursorPos.Ln) {
			if tv.CursorPos.Ln < tv.NLines-1 {
				tv.CursorPos.Ch = 0
				tv.CursorPos.Ln++
			} else {
				tv.CursorPos.Ch = tv.Buf.LineLen(tv.CursorPos.Ln)
			}
		}
	}
	tv.SetCursorCol(tv.CursorPos)
	tv.SetCursorShow(tv.CursorPos)
	tv.CursorSelect(org)
}

// CursorForwardWord moves the cursor forward by words
func (tv *TextView) CursorForwardWord(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	for i := 0; i < steps; i++ {
		txt := tv.Buf.Line(tv.CursorPos.Ln)
		sz := len(txt)
		if sz > 0 && tv.CursorPos.Ch < sz {
			ch := tv.CursorPos.Ch
			var done = false
			for ch < sz && !done { // if on a wb, go past
				r1 := txt[ch]
				r2 := rune(-1)
				if ch < sz-1 {
					r2 = txt[ch+1]
				}
				if lex.IsWordBreak(r1, r2) {
					ch++
				} else {
					done = true
				}
			}
			done = false
			for ch < sz && !done {
				r1 := txt[ch]
				r2 := rune(-1)
				if ch < sz-1 {
					r2 = txt[ch+1]
				}
				if !lex.IsWordBreak(r1, r2) {
					ch++
				} else {
					done = true
				}
			}
			tv.CursorPos.Ch = ch
		} else {
			if tv.CursorPos.Ln < tv.NLines-1 {
				tv.CursorPos.Ch = 0
				tv.CursorPos.Ln++
			} else {
				tv.CursorPos.Ch = tv.Buf.LineLen(tv.CursorPos.Ln)
			}
		}
	}
	tv.SetCursorCol(tv.CursorPos)
	tv.SetCursorShow(tv.CursorPos)
	tv.CursorSelect(org)
}

// CursorDown moves the cursor down line(s)
func (tv *TextView) CursorDown(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	pos := tv.CursorPos
	for i := 0; i < steps; i++ {
		gotwrap := false
		if wln := tv.WrappedLines(pos.Ln); wln > 1 {
			si, ri, _ := tv.WrappedLineNo(pos)
			if si < wln-1 {
				si++
				mxlen := min(len(tv.Renders[pos.Ln].Spans[si].Text), tv.CursorCol)
				if tv.CursorCol < mxlen {
					ri = tv.CursorCol
				} else {
					ri = mxlen
				}
				nwc, _ := tv.Renders[pos.Ln].SpanPosToRuneIdx(si, ri)
				if si == wln-1 && ri == mxlen {
					nwc++
				}
				pos.Ch = nwc
				gotwrap = true

			}
		}
		if !gotwrap {
			pos.Ln++
			if pos.Ln >= tv.NLines {
				pos.Ln = tv.NLines - 1
				break
			}
			mxlen := min(tv.Buf.LineLen(pos.Ln), tv.CursorCol)
			if tv.CursorCol < mxlen {
				pos.Ch = tv.CursorCol
			} else {
				pos.Ch = mxlen
			}
		}
	}
	tv.SetCursorShow(pos)
	tv.CursorSelect(org)
}

// CursorPageDown moves the cursor down page(s), where a page is defined abcdef
// dynamically as just moving the cursor off the screen
func (tv *TextView) CursorPageDown(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	for i := 0; i < steps; i++ {
		lvln := tv.LastVisibleLine(tv.CursorPos.Ln)
		tv.CursorPos.Ln = lvln
		if tv.CursorPos.Ln >= tv.NLines {
			tv.CursorPos.Ln = tv.NLines - 1
		}
		tv.CursorPos.Ch = min(tv.Buf.LineLen(tv.CursorPos.Ln), tv.CursorCol)
		tv.ScrollCursorToTop()
		tv.RenderCursor(true)
	}
	tv.SetCursor(tv.CursorPos)
	tv.CursorSelect(org)
}

// CursorBackward moves the cursor backward
func (tv *TextView) CursorBackward(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	for i := 0; i < steps; i++ {
		tv.CursorPos.Ch--
		if tv.CursorPos.Ch < 0 {
			if tv.CursorPos.Ln > 0 {
				tv.CursorPos.Ln--
				tv.CursorPos.Ch = tv.Buf.LineLen(tv.CursorPos.Ln)
			} else {
				tv.CursorPos.Ch = 0
			}
		}
	}
	tv.SetCursorCol(tv.CursorPos)
	tv.SetCursorShow(tv.CursorPos)
	tv.CursorSelect(org)
}

// CursorBackwardWord moves the cursor backward by words
func (tv *TextView) CursorBackwardWord(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	for i := 0; i < steps; i++ {
		txt := tv.Buf.Line(tv.CursorPos.Ln)
		sz := len(txt)
		if sz > 0 && tv.CursorPos.Ch > 0 {
			ch := min(tv.CursorPos.Ch, sz-1)
			var done = false
			for ch < sz && !done { // if on a wb, go past
				r1 := txt[ch]
				r2 := rune(-1)
				if ch > 0 {
					r2 = txt[ch-1]
				}
				if lex.IsWordBreak(r1, r2) {
					ch--
					if ch == -1 {
						done = true
					}
				} else {
					done = true
				}
			}
			done = false
			for ch < sz && ch >= 0 && !done {
				r1 := txt[ch]
				r2 := rune(-1)
				if ch > 0 {
					r2 = txt[ch-1]
				}
				if !lex.IsWordBreak(r1, r2) {
					ch--
				} else {
					done = true
				}
			}
			tv.CursorPos.Ch = ch
		} else {
			if tv.CursorPos.Ln > 0 {
				tv.CursorPos.Ln--
				tv.CursorPos.Ch = tv.Buf.LineLen(tv.CursorPos.Ln)
			} else {
				tv.CursorPos.Ch = 0
			}
		}
	}
	tv.SetCursorCol(tv.CursorPos)
	tv.SetCursorShow(tv.CursorPos)
	tv.CursorSelect(org)
}

// CursorUp moves the cursor up line(s)
func (tv *TextView) CursorUp(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	pos := tv.CursorPos
	for i := 0; i < steps; i++ {
		gotwrap := false
		if wln := tv.WrappedLines(pos.Ln); wln > 1 {
			si, ri, _ := tv.WrappedLineNo(pos)
			if si > 0 {
				ri = tv.CursorCol
				// fmt.Printf("up cursorcol: %v\n", tv.CursorCol)
				nwc, _ := tv.Renders[pos.Ln].SpanPosToRuneIdx(si-1, ri)
				pos.Ch = nwc
				gotwrap = true
			}
		}
		if !gotwrap {
			pos.Ln--
			if pos.Ln < 0 {
				pos.Ln = 0
				break
			}
			if wln := tv.WrappedLines(pos.Ln); wln > 1 { // just entered end of wrapped line
				si := wln - 1
				ri := tv.CursorCol
				nwc, _ := tv.Renders[pos.Ln].SpanPosToRuneIdx(si, ri)
				pos.Ch = nwc
			} else {
				mxlen := min(tv.Buf.LineLen(pos.Ln), tv.CursorCol)
				if tv.CursorCol < mxlen {
					pos.Ch = tv.CursorCol
				} else {
					pos.Ch = mxlen
				}
			}
		}
	}
	tv.SetCursorShow(pos)
	tv.CursorSelect(org)
}

// CursorPageUp moves the cursor up page(s), where a page is defined
// dynamically as just moving the cursor off the screen
func (tv *TextView) CursorPageUp(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	for i := 0; i < steps; i++ {
		lvln := tv.FirstVisibleLine(tv.CursorPos.Ln)
		tv.CursorPos.Ln = lvln
		if tv.CursorPos.Ln <= 0 {
			tv.CursorPos.Ln = 0
		}
		tv.CursorPos.Ch = min(tv.Buf.LineLen(tv.CursorPos.Ln), tv.CursorCol)
		tv.ScrollCursorToBottom()
		tv.RenderCursor(true)
	}
	tv.SetCursor(tv.CursorPos)
	tv.CursorSelect(org)
}

// CursorRecenter re-centers the view around the cursor position, toggling
// between putting cursor in middle, top, and bottom of view
func (tv *TextView) CursorRecenter() {
	tv.ValidateCursor()
	tv.SavePosHistory(tv.CursorPos)
	cur := (tv.lastRecenter + 1) % 3
	switch cur {
	case 0:
		tv.ScrollCursorToBottom()
	case 1:
		tv.ScrollCursorToVertCenter()
	case 2:
		tv.ScrollCursorToTop()
	}
	tv.lastRecenter = cur
}

// CursorStartLine moves the cursor to the start of the line, updating selection
// if select mode is active
func (tv *TextView) CursorStartLine() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	pos := tv.CursorPos

	gotwrap := false
	if wln := tv.WrappedLines(pos.Ln); wln > 1 {
		si, ri, _ := tv.WrappedLineNo(pos)
		if si > 0 {
			ri = 0
			nwc, _ := tv.Renders[pos.Ln].SpanPosToRuneIdx(si, ri)
			pos.Ch = nwc
			tv.CursorPos = pos
			tv.CursorCol = ri
			gotwrap = true
		}
	}
	if !gotwrap {
		tv.CursorPos.Ch = 0
		tv.CursorCol = tv.CursorPos.Ch
	}
	// fmt.Printf("sol cursorcol: %v\n", tv.CursorCol)
	tv.SetCursor(tv.CursorPos)
	tv.ScrollCursorToLeft()
	tv.RenderCursor(true)
	tv.CursorSelect(org)
}

// CursorStartDoc moves the cursor to the start of the text, updating selection
// if select mode is active
func (tv *TextView) CursorStartDoc() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	tv.CursorPos.Ln = 0
	tv.CursorPos.Ch = 0
	tv.CursorCol = tv.CursorPos.Ch
	tv.SetCursor(tv.CursorPos)
	tv.ScrollCursorToTop()
	tv.RenderCursor(true)
	tv.CursorSelect(org)
}

// CursorEndLine moves the cursor to the end of the text
func (tv *TextView) CursorEndLine() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	pos := tv.CursorPos

	gotwrap := false
	if wln := tv.WrappedLines(pos.Ln); wln > 1 {
		si, ri, _ := tv.WrappedLineNo(pos)
		ri = len(tv.Renders[pos.Ln].Spans[si].Text) - 1
		nwc, _ := tv.Renders[pos.Ln].SpanPosToRuneIdx(si, ri)
		if si == len(tv.Renders[pos.Ln].Spans)-1 { // last span
			ri++
			nwc++
		}
		tv.CursorCol = ri
		pos.Ch = nwc
		tv.CursorPos = pos
		gotwrap = true
	}
	if !gotwrap {
		tv.CursorPos.Ch = tv.Buf.LineLen(tv.CursorPos.Ln)
		tv.CursorCol = tv.CursorPos.Ch
	}
	tv.SetCursor(tv.CursorPos)
	tv.ScrollCursorToRight()
	tv.RenderCursor(true)
	tv.CursorSelect(org)
}

// CursorEndDoc moves the cursor to the end of the text, updating selection if
// select mode is active
func (tv *TextView) CursorEndDoc() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	tv.CursorPos.Ln = max(tv.NLines-1, 0)
	tv.CursorPos.Ch = tv.Buf.LineLen(tv.CursorPos.Ln)
	tv.CursorCol = tv.CursorPos.Ch
	tv.SetCursor(tv.CursorPos)
	tv.ScrollCursorToBottom()
	tv.RenderCursor(true)
	tv.CursorSelect(org)
}

// todo: ctrl+backspace = delete word
// shift+arrow = select
// uparrow = start / down = end

// CursorBackspace deletes character(s) immediately before cursor
func (tv *TextView) CursorBackspace(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	if tv.HasSelection() {
		org = tv.SelectReg.Start
		tv.DeleteSelection()
		tv.SetCursorShow(org)
		return
	}
	// note: no update b/c signal from buf will drive update
	tv.CursorBackward(steps)
	tv.ScrollCursorToCenterIfHidden()
	tv.RenderCursor(true)
	tv.Buf.DeleteText(tv.CursorPos, org, EditSignal)
}

// CursorDelete deletes character(s) immediately after the cursor
func (tv *TextView) CursorDelete(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	if tv.HasSelection() {
		tv.DeleteSelection()
		return
	}
	// note: no update b/c signal from buf will drive update
	org := tv.CursorPos
	tv.CursorForward(steps)
	tv.Buf.DeleteText(org, tv.CursorPos, EditSignal)
	tv.SetCursorShow(org)
}

// CursorBackspaceWord deletes words(s) immediately before cursor
func (tv *TextView) CursorBackspaceWord(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	if tv.HasSelection() {
		tv.DeleteSelection()
		tv.SetCursorShow(org)
		return
	}
	// note: no update b/c signal from buf will drive update
	tv.CursorBackwardWord(steps)
	tv.ScrollCursorToCenterIfHidden()
	tv.RenderCursor(true)
	tv.Buf.DeleteText(tv.CursorPos, org, EditSignal)
}

// CursorDeleteWord deletes word(s) immediately after the cursor
func (tv *TextView) CursorDeleteWord(steps int) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	if tv.HasSelection() {
		tv.DeleteSelection()
		return
	}
	// note: no update b/c signal from buf will drive update
	org := tv.CursorPos
	tv.CursorForwardWord(steps)
	tv.Buf.DeleteText(org, tv.CursorPos, EditSignal)
	tv.SetCursorShow(org)
}

// CursorKill deletes text from cursor to end of text
func (tv *TextView) CursorKill() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	org := tv.CursorPos
	pos := tv.CursorPos

	atEnd := false
	if wln := tv.WrappedLines(pos.Ln); wln > 1 {
		si, ri, _ := tv.WrappedLineNo(pos)
		llen := len(tv.Renders[pos.Ln].Spans[si].Text)
		if si == wln-1 {
			llen--
		}
		atEnd = (ri == llen)
	} else {
		llen := tv.Buf.LineLen(pos.Ln)
		atEnd = (tv.CursorPos.Ch == llen)
	}
	if atEnd {
		tv.CursorForward(1)
	} else {
		tv.CursorEndLine()
	}
	tv.Buf.DeleteText(org, tv.CursorPos, EditSignal)
	tv.SetCursorShow(org)
}

// CursorTranspose swaps the character at the cursor with the one before it
func (tv *TextView) CursorTranspose() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.ValidateCursor()
	pos := tv.CursorPos
	if pos.Ch == 0 {
		return
	}
	ppos := pos
	ppos.Ch--
	tv.Buf.LinesMu.Lock()
	lln := len(tv.Buf.Lines[pos.Ln])
	end := false
	if pos.Ch >= lln {
		end = true
		pos.Ch = lln - 1
		ppos.Ch = lln - 2
	}
	chr := tv.Buf.Lines[pos.Ln][pos.Ch]
	pchr := tv.Buf.Lines[pos.Ln][ppos.Ch]
	tv.Buf.LinesMu.Unlock()
	repl := string([]rune{chr, pchr})
	pos.Ch++
	tv.Buf.ReplaceText(ppos, pos, ppos, repl, EditSignal, ReplaceMatchCase)
	if !end {
		tv.SetCursorShow(pos)
	}
}

// CursorTranspose swaps the word at the cursor with the one before it
func (tv *TextView) CursorTransposeWord() {
}

// JumpToLinePrompt jumps to given line number (minus 1) from prompt
func (tv *TextView) JumpToLinePrompt() {
	gi.StringPromptDialog(tv.Sc, "", "Line no..",
		gi.DlgOpts{Title: "Jump To Line", Prompt: "Line Number to jump to"},
		tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			dlg := send.(*gi.DialogStage)
			if sig == int64(gi.DialogAccepted) {
				val := gi.StringPromptDialogValue(dlg)
				ln, ok := laser.ToInt(val)
				if ok {
					tv.JumpToLine(int(ln))
				}
			}
		})

}

// JumpToLine jumps to given line number (minus 1)
func (tv *TextView) JumpToLine(ln int) {
	wupdt := tv.TopUpdateStart()
	tv.SetCursorShow(lex.Pos{Ln: ln - 1})
	tv.SavePosHistory(tv.CursorPos)
	tv.TopUpdateEnd(wupdt)
}

// FindNextLink finds next link after given position, returns false if no such links
func (tv *TextView) FindNextLink(pos lex.Pos) (lex.Pos, textbuf.Region, bool) {
	for ln := pos.Ln; ln < tv.NLines; ln++ {
		if len(tv.Renders[ln].Links) == 0 {
			pos.Ch = 0
			pos.Ln = ln + 1
			continue
		}
		rend := &tv.Renders[ln]
		si, ri, _ := rend.RuneSpanPos(pos.Ch)
		for ti := range rend.Links {
			tl := &rend.Links[ti]
			if tl.StartSpan >= si && tl.StartIdx >= ri {
				st, _ := rend.SpanPosToRuneIdx(tl.StartSpan, tl.StartIdx)
				ed, _ := rend.SpanPosToRuneIdx(tl.EndSpan, tl.EndIdx)
				reg := textbuf.NewRegion(ln, st, ln, ed)
				pos.Ch = st + 1 // get into it so next one will go after..
				return pos, reg, true
			}
		}
		pos.Ln = ln + 1
		pos.Ch = 0
	}
	return pos, textbuf.RegionNil, false
}

// FindPrevLink finds previous link before given position, returns false if no such links
func (tv *TextView) FindPrevLink(pos lex.Pos) (lex.Pos, textbuf.Region, bool) {
	for ln := pos.Ln - 1; ln >= 0; ln-- {
		if len(tv.Renders[ln].Links) == 0 {
			if ln-1 >= 0 {
				pos.Ch = tv.Buf.LineLen(ln-1) - 2
			} else {
				ln = tv.NLines
				pos.Ch = tv.Buf.LineLen(ln - 2)
			}
			continue
		}
		rend := &tv.Renders[ln]
		si, ri, _ := rend.RuneSpanPos(pos.Ch)
		nl := len(rend.Links)
		for ti := nl - 1; ti >= 0; ti-- {
			tl := &rend.Links[ti]
			if tl.StartSpan <= si && tl.StartIdx < ri {
				st, _ := rend.SpanPosToRuneIdx(tl.StartSpan, tl.StartIdx)
				ed, _ := rend.SpanPosToRuneIdx(tl.EndSpan, tl.EndIdx)
				reg := textbuf.NewRegion(ln, st, ln, ed)
				pos.Ln = ln
				pos.Ch = st + 1
				return pos, reg, true
			}
		}
	}
	return pos, textbuf.RegionNil, false
}

// HighlightRegion creates a new highlighted region, and renders it,
// un-drawing any prior highlights
func (tv *TextView) HighlightRegion(reg textbuf.Region) {
	prevh := tv.Highlights
	tv.Highlights = []textbuf.Region{reg}
	tv.UpdateHighlights(prevh)
}

// CursorNextLink moves cursor to next link. wraparound wraps around to top of
// buffer if none found -- returns true if found
func (tv *TextView) CursorNextLink(wraparound bool) bool {
	if tv.NLines == 0 {
		return false
	}
	tv.ValidateCursor()
	npos, reg, has := tv.FindNextLink(tv.CursorPos)
	if !has {
		if !wraparound {
			return false
		}
		npos, reg, has = tv.FindNextLink(lex.Pos{}) // wraparound
		if !has {
			return false
		}
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.HighlightRegion(reg)
	tv.SetCursorShow(npos)
	tv.SavePosHistory(tv.CursorPos)
	return true
}

// CursorPrevLink moves cursor to previous link. wraparound wraps around to
// bottom of buffer if none found. returns true if found
func (tv *TextView) CursorPrevLink(wraparound bool) bool {
	if tv.NLines == 0 {
		return false
	}
	tv.ValidateCursor()
	npos, reg, has := tv.FindPrevLink(tv.CursorPos)
	if !has {
		if !wraparound {
			return false
		}
		npos, reg, has = tv.FindPrevLink(lex.Pos{}) // wraparound
		if !has {
			return false
		}
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.HighlightRegion(reg)
	tv.SetCursorShow(npos)
	tv.SavePosHistory(tv.CursorPos)
	return true
}

///////////////////////////////////////////////////////////////////////////////
//    Undo / Redo

// Undo undoes previous action
func (tv *TextView) Undo() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tbe := tv.Buf.Undo()
	if tbe != nil {
		if tbe.Delete { // now an insert
			tv.SetCursorShow(tbe.Reg.End)
		} else {
			tv.SetCursorShow(tbe.Reg.Start)
		}
	} else {
		tv.CursorMovedSig() // updates status..
		tv.ScrollCursorToCenterIfHidden()
	}
	tv.SavePosHistory(tv.CursorPos)
}

// Redo redoes previously undone action
func (tv *TextView) Redo() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tbe := tv.Buf.Redo()
	if tbe != nil {
		if tbe.Delete {
			tv.SetCursorShow(tbe.Reg.Start)
		} else {
			tv.SetCursorShow(tbe.Reg.End)
		}
	} else {
		tv.ScrollCursorToCenterIfHidden()
	}
	tv.SavePosHistory(tv.CursorPos)
}

///////////////////////////////////////////////////////////////////////////////
//    Search / Find

// FindMatches finds the matches with given search string (literal, not regex)
// and case sensitivity, updates highlights for all.  returns false if none
// found
func (tv *TextView) FindMatches(find string, useCase, lexItems bool) ([]textbuf.Match, bool) {
	fsz := len(find)
	if fsz == 0 {
		tv.Highlights = nil
		return nil, false
	}
	_, matches := tv.Buf.Search([]byte(find), !useCase, lexItems)
	if len(matches) == 0 {
		tv.Highlights = nil
		tv.RenderAllLines()
		return matches, false
	}
	hi := make([]textbuf.Region, len(matches))
	for i, m := range matches {
		hi[i] = m.Reg
		if i > TextViewMaxFindHighlights {
			break
		}
	}
	tv.Highlights = hi
	tv.RenderAllLines()
	return matches, true
}

// MatchFromPos finds the match at or after the given text position -- returns 0, false if none
func (tv *TextView) MatchFromPos(matches []textbuf.Match, cpos lex.Pos) (int, bool) {
	for i, m := range matches {
		reg := tv.Buf.AdjustReg(m.Reg)
		if reg.Start == cpos || cpos.IsLess(reg.Start) {
			return i, true
		}
	}
	return 0, false
}

// ISearch holds all the interactive search data
type ISearch struct {

	// if true, in interactive search mode
	On bool `json:"-" xml:"-" desc:"if true, in interactive search mode"`

	// current interactive search string
	Find string `json:"-" xml:"-" desc:"current interactive search string"`

	// pay attention to case in isearch -- triggered by typing an upper-case letter
	UseCase bool `json:"-" xml:"-" desc:"pay attention to case in isearch -- triggered by typing an upper-case letter"`

	// current search matches
	Matches []textbuf.Match `json:"-" xml:"-" desc:"current search matches"`

	// position within isearch matches
	Pos int `json:"-" xml:"-" desc:"position within isearch matches"`

	// position in search list from previous search
	PrevPos int `json:"-" xml:"-" desc:"position in search list from previous search"`

	// starting position for search -- returns there after on cancel
	StartPos lex.Pos `json:"-" xml:"-" desc:"starting position for search -- returns there after on cancel"`
}

// TextViewMaxFindHighlights is the maximum number of regions to highlight on find
var TextViewMaxFindHighlights = 1000

// PrevISearchString is the previous ISearch string
var PrevISearchString string

// ISearchMatches finds ISearch matches -- returns true if there are any
func (tv *TextView) ISearchMatches() bool {
	got := false
	tv.ISearch.Matches, got = tv.FindMatches(tv.ISearch.Find, tv.ISearch.UseCase, false)
	return got
}

// ISearchNextMatch finds next match after given cursor position, and highlights
// it, etc
func (tv *TextView) ISearchNextMatch(cpos lex.Pos) bool {
	if len(tv.ISearch.Matches) == 0 {
		tv.ISearchSig()
		return false
	}
	tv.ISearch.Pos, _ = tv.MatchFromPos(tv.ISearch.Matches, cpos)
	tv.ISearchSelectMatch(tv.ISearch.Pos)
	return true
}

// ISearchSelectMatch selects match at given match index (e.g., tv.ISearch.Pos)
func (tv *TextView) ISearchSelectMatch(midx int) {
	nm := len(tv.ISearch.Matches)
	if midx >= nm {
		tv.ISearchSig()
		return
	}
	m := tv.ISearch.Matches[midx]
	reg := tv.Buf.AdjustReg(m.Reg)
	pos := reg.Start
	tv.SelectReg = reg
	tv.SetCursor(pos)
	tv.SavePosHistory(tv.CursorPos)
	tv.ScrollCursorToCenterIfHidden()
	tv.RenderSelectLines()
	tv.ISearchSig()
}

// ISearchSig sends the signal that ISearch is updated
func (tv *TextView) ISearchSig() {
	tv.TextViewSig.Emit(tv.This(), int64(TextViewISearch), tv.CursorPos)
}

// ISearchStart is an emacs-style interactive search mode -- this is called when
// the search command itself is entered
func (tv *TextView) ISearchStart() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	if tv.ISearch.On {
		if tv.ISearch.Find != "" { // already searching -- find next
			sz := len(tv.ISearch.Matches)
			if sz > 0 {
				if tv.ISearch.Pos < sz-1 {
					tv.ISearch.Pos++
				} else {
					tv.ISearch.Pos = 0
				}
				tv.ISearchSelectMatch(tv.ISearch.Pos)
			}
		} else { // restore prev
			if PrevISearchString != "" {
				tv.ISearch.Find = PrevISearchString
				tv.ISearch.UseCase = lex.HasUpperCase(tv.ISearch.Find)
				tv.ISearchMatches()
				tv.ISearchNextMatch(tv.CursorPos)
				tv.ISearch.StartPos = tv.CursorPos
			}
			// nothing..
		}
	} else {
		tv.ISearch.On = true
		tv.ISearch.Find = ""
		tv.ISearch.StartPos = tv.CursorPos
		tv.ISearch.UseCase = false
		tv.ISearch.Matches = nil
		tv.SelectReset()
		tv.ISearch.Pos = -1
		tv.ISearchSig()
	}
}

// ISearchKeyInput is an emacs-style interactive search mode -- this is called
// when keys are typed while in search mode
func (tv *TextView) ISearchKeyInput(kt events.Event) {
	r := kt.Rune
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	// if tv.ISearch.Find == PrevISearchString { // undo starting point
	// 	tv.ISearch.Find = ""
	// }
	if unicode.IsUpper(r) { // todo: more complex
		tv.ISearch.UseCase = true
	}
	tv.ISearch.Find += string(r)
	tv.ISearchMatches()
	sz := len(tv.ISearch.Matches)
	if sz == 0 {
		tv.ISearch.Pos = -1
		tv.ISearchSig()
		return
	}
	tv.ISearchNextMatch(tv.CursorPos)
}

// ISearchBackspace gets rid of one item in search string
func (tv *TextView) ISearchBackspace() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	if tv.ISearch.Find == PrevISearchString { // undo starting point
		tv.ISearch.Find = ""
		tv.ISearch.UseCase = false
		tv.ISearch.Matches = nil
		tv.SelectReset()
		tv.ISearch.Pos = -1
		tv.ISearchSig()
		return
	}
	if len(tv.ISearch.Find) <= 1 {
		tv.SelectReset()
		tv.ISearch.Find = ""
		tv.ISearch.UseCase = false
		return
	}
	tv.ISearch.Find = tv.ISearch.Find[:len(tv.ISearch.Find)-1]
	tv.ISearchMatches()
	sz := len(tv.ISearch.Matches)
	if sz == 0 {
		tv.ISearch.Pos = -1
		tv.ISearchSig()
		return
	}
	tv.ISearchNextMatch(tv.CursorPos)
}

// ISearchCancel cancels ISearch mode
func (tv *TextView) ISearchCancel() {
	if !tv.ISearch.On {
		return
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	if tv.ISearch.Find != "" {
		PrevISearchString = tv.ISearch.Find
	}
	tv.ISearch.PrevPos = tv.ISearch.Pos
	tv.ISearch.Find = ""
	tv.ISearch.UseCase = false
	tv.ISearch.On = false
	tv.ISearch.Pos = -1
	tv.ISearch.Matches = nil
	tv.Highlights = nil
	tv.SavePosHistory(tv.CursorPos)
	tv.RenderAllLines()
	tv.SelectReset()
	tv.ISearchSig()
}

///////////////////////////////////////////////////////////////////////////////
//    Query-Replace

// QReplace holds all the query-replace data
type QReplace struct {

	// if true, in interactive search mode
	On bool `json:"-" xml:"-" desc:"if true, in interactive search mode"`

	// current interactive search string
	Find string `json:"-" xml:"-" desc:"current interactive search string"`

	// current interactive search string
	Replace string `json:"-" xml:"-" desc:"current interactive search string"`

	// pay attention to case in isearch -- triggered by typing an upper-case letter
	UseCase bool `json:"-" xml:"-" desc:"pay attention to case in isearch -- triggered by typing an upper-case letter"`

	// search only as entire lexically-tagged item boundaries -- key for replacing short local variables like i
	LexItems bool `json:"-" xml:"-" desc:"search only as entire lexically-tagged item boundaries -- key for replacing short local variables like i"`

	// current search matches
	Matches []textbuf.Match `json:"-" xml:"-" desc:"current search matches"`

	// position within isearch matches
	Pos int `json:"-" xml:"-" desc:"position within isearch matches"`

	// position in search list from previous search
	PrevPos int `json:"-" xml:"-" desc:"position in search list from previous search"`

	// starting position for search -- returns there after on cancel
	StartPos lex.Pos `json:"-" xml:"-" desc:"starting position for search -- returns there after on cancel"`
}

// PrevQReplaceFinds are the previous QReplace strings
var PrevQReplaceFinds []string

// PrevQReplaceRepls are the previous QReplace strings
var PrevQReplaceRepls []string

// QReplaceSig sends the signal that QReplace is updated
func (tv *TextView) QReplaceSig() {
	tv.TextViewSig.Emit(tv.This(), int64(TextViewQReplace), tv.CursorPos)
}

// QReplaceDialog prompts the user for a query-replace items, with choosers with history
func QReplaceDialog(ctx gi.Widget, opts gi.DlgOpts, find string, lexitems bool, fun func(dlg *gi.Dialog)) *gi.Dialog {
	dlg := gi.NewStdDialog(ctx, opts, fun)

	frame := dlg.Stage.Scene
	prIdx := dlg.PromptWidgetIdx(frame)

	tff := frame.InsertNewChild(gi.ComboBoxType, prIdx+1, "find").(*gi.Chooser)
	tff.Editable = true
	tff.SetStretchMaxWidth()
	tff.SetMinPrefWidth(units.Ch(60))
	tff.ConfigParts(vp)
	tff.ItemsFromStringList(PrevQReplaceFinds, true, 0)
	if find != "" {
		tff.SetCurVal(find)
	}

	tfr := frame.InsertNewChild(gi.ComboBoxType, prIdx+2, "repl").(*gi.Chooser)
	tfr.Editable = true
	tfr.SetStretchMaxWidth()
	tfr.SetMinPrefWidth(units.Ch(60))
	tfr.ConfigParts(vp)
	tfr.ItemsFromStringList(PrevQReplaceRepls, true, 0)

	lb := frame.InsertNewChild(gi.TypeCheckBox, prIdx+3, "lexb").(*gi.Switch)
	lb.SetText("Lexical Items")
	lb.SetState(lexitems, states.Checked)
	lb.Tooltip = "search matches entire lexically tagged items -- good for finding local variable names like 'i' and not matching everything"

	return dlg
}

// QReplaceDialogValues gets the string values
func QReplaceDialogValues(dlg *gi.Dialog) (find, repl string, lexItems bool) {
	frame := dlg.Stage.Scene
	tff := frame.ChildByName("find", 1).(*gi.Chooser)
	if tf, found := tff.TextField(); found {
		find = tf.Text()
	}
	tfr := frame.ChildByName("repl", 2).(*gi.Chooser)
	if tf, found := tfr.TextField(); found {
		repl = tf.Text()
	}
	lb := frame.ChildByName("lexb", 3).(*gi.Switch)
	lexItems = lb.StateIs(states.Checked)
	return
}

// QReplacePrompt is an emacs-style query-replace mode -- this starts the process, prompting
// user for items to search etc
func (tv *TextView) QReplacePrompt() {
	find := ""
	if tv.HasSelection() {
		find = string(tv.Selection().ToBytes())
	}
	QReplaceDialog(tv, gi.DlgOpts{Title: "Query-Replace", Prompt: "Enter strings for find and replace, then select Ok -- with dialog dismissed press <b>y</b> to replace current match, <b>n</b> to skip, <b>Enter</b> or <b>q</b> to quit, <b>!</b> to replace-all remaining"}, find, tv.QReplace.LexItems, func(dlg *gi.Dialog) {
		if dlg.Accepted {
			find, repl, lexItems := QReplaceDialogValues(dlg)
			tv.QReplaceStart(find, repl, lexItems)
		}
	})
}

// QReplaceStart starts query-replace using given find, replace strings
func (tv *TextView) QReplaceStart(find, repl string, lexItems bool) {
	tv.QReplace.On = true
	tv.QReplace.Find = find
	tv.QReplace.Replace = repl
	tv.QReplace.LexItems = lexItems
	tv.QReplace.StartPos = tv.CursorPos
	tv.QReplace.UseCase = lex.HasUpperCase(find)
	tv.QReplace.Matches = nil
	tv.QReplace.Pos = -1

	gi.StringsInsertFirstUnique(&PrevQReplaceFinds, find, gi.Prefs.Params.SavedPathsMax)
	gi.StringsInsertFirstUnique(&PrevQReplaceRepls, repl, gi.Prefs.Params.SavedPathsMax)

	tv.QReplaceMatches()
	tv.QReplace.Pos, _ = tv.MatchFromPos(tv.QReplace.Matches, tv.CursorPos)
	tv.QReplaceSelectMatch(tv.QReplace.Pos)
	tv.QReplaceSig()
}

// QReplaceMatches finds QReplace matches -- returns true if there are any
func (tv *TextView) QReplaceMatches() bool {
	got := false
	tv.QReplace.Matches, got = tv.FindMatches(tv.QReplace.Find, tv.QReplace.UseCase, tv.QReplace.LexItems)
	return got
}

// QReplaceNextMatch finds next match using, QReplace.Pos and highlights it, etc
func (tv *TextView) QReplaceNextMatch() bool {
	nm := len(tv.QReplace.Matches)
	if nm == 0 {
		return false
	}
	tv.QReplace.Pos++
	if tv.QReplace.Pos >= nm {
		return false
	}
	tv.QReplaceSelectMatch(tv.QReplace.Pos)
	return true
}

// QReplaceSelectMatch selects match at given match index (e.g., tv.QReplace.Pos)
func (tv *TextView) QReplaceSelectMatch(midx int) {
	nm := len(tv.QReplace.Matches)
	if midx >= nm {
		return
	}
	m := tv.QReplace.Matches[midx]
	reg := tv.Buf.AdjustReg(m.Reg)
	pos := reg.Start
	tv.SelectReg = reg
	tv.SetCursor(pos)
	tv.SavePosHistory(tv.CursorPos)
	tv.ScrollCursorToCenterIfHidden()
	tv.RenderSelectLines()
	tv.QReplaceSig()
}

// QReplaceReplace replaces at given match index (e.g., tv.QReplace.Pos)
func (tv *TextView) QReplaceReplace(midx int) {
	nm := len(tv.QReplace.Matches)
	if midx >= nm {
		return
	}
	m := tv.QReplace.Matches[midx]
	rep := tv.QReplace.Replace
	reg := tv.Buf.AdjustReg(m.Reg)
	pos := reg.Start
	// last arg is matchCase, only if not using case to match and rep is also lower case
	matchCase := !tv.QReplace.UseCase && !lex.HasUpperCase(rep)
	tv.Buf.ReplaceText(reg.Start, reg.End, pos, rep, EditSignal, matchCase)
	tv.Highlights[midx] = textbuf.RegionNil
	tv.SetCursor(pos)
	tv.SavePosHistory(tv.CursorPos)
	tv.ScrollCursorToCenterIfHidden()
	tv.QReplaceSig()
}

// QReplaceReplaceAll replaces all remaining from index
func (tv *TextView) QReplaceReplaceAll(midx int) {
	nm := len(tv.QReplace.Matches)
	if midx >= nm {
		return
	}
	for mi := midx; mi < nm; mi++ {
		tv.QReplaceReplace(mi)
	}
}

// QReplaceKeyInput is an emacs-style interactive search mode -- this is called
// when keys are typed while in search mode
func (tv *TextView) QReplaceKeyInput(kt events.Event) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)

	switch {
	case kt.Rune == 'y':
		tv.QReplaceReplace(tv.QReplace.Pos)
		if !tv.QReplaceNextMatch() {
			tv.QReplaceCancel()
		}
	case kt.Rune == 'n':
		if !tv.QReplaceNextMatch() {
			tv.QReplaceCancel()
		}
	case kt.Rune == 'q' || kt.KeyChord() == "ReturnEnter":
		tv.QReplaceCancel()
	case kt.Rune == '!':
		tv.QReplaceReplaceAll(tv.QReplace.Pos)
		tv.QReplaceCancel()
	}
}

// QReplaceCancel cancels QReplace mode
func (tv *TextView) QReplaceCancel() {
	if !tv.QReplace.On {
		return
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.QReplace.On = false
	tv.QReplace.Pos = -1
	tv.QReplace.Matches = nil
	tv.Highlights = nil
	tv.SavePosHistory(tv.CursorPos)
	tv.RenderAllLines()
	tv.SelectReset()
	tv.QReplaceSig()
}

// EscPressed emitted for KeyFunAbort or KeyFunCancelSelect -- effect depends on state..
func (tv *TextView) EscPressed() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	switch {
	case tv.ISearch.On:
		tv.ISearchCancel()
		tv.SetCursorShow(tv.ISearch.StartPos)
	case tv.QReplace.On:
		tv.QReplaceCancel()
		tv.SetCursorShow(tv.ISearch.StartPos)
	case tv.HasSelection():
		tv.SelectReset()
	default:
		tv.Highlights = nil
		tv.RenderAllLines()
	}
}

// ReCaseSelection changes the case of the currently-selected text.
// Returns the new text -- empty if nothing selected.
func (tv *TextView) ReCaseSelection(c textbuf.Cases) string {
	if !tv.HasSelection() {
		return ""
	}
	sel := tv.Selection()
	nstr := textbuf.ReCaseString(string(sel.ToBytes()), c)
	tv.Buf.ReplaceText(sel.Reg.Start, sel.Reg.End, sel.Reg.Start, nstr, EditSignal, ReplaceNoMatchCase)
	return nstr
}

///////////////////////////////////////////////////////////////////////////////
//    Selection

// ClearSelected resets both the global selected flag and any current selection
func (tv *TextView) ClearSelected() {
	tv.WidgetBase.ClearSelected()
	tv.SelectReset()
}

// HasSelection returns whether there is a selected region of text
func (tv *TextView) HasSelection() bool {
	if tv.SelectReg.Start.IsLess(tv.SelectReg.End) {
		return true
	}
	return false
}

// Selection returns the currently selected text as a textbuf.Edit, which
// captures start, end, and full lines in between -- nil if no selection
func (tv *TextView) Selection() *textbuf.Edit {
	if tv.HasSelection() {
		return tv.Buf.Region(tv.SelectReg.Start, tv.SelectReg.End)
	}
	return nil
}

// SelectModeToggle toggles the SelectMode, updating selection with cursor movement
func (tv *TextView) SelectModeToggle() {
	if tv.SelectMode {
		tv.SelectMode = false
	} else {
		tv.SelectMode = true
		tv.SelectStart = tv.CursorPos
		tv.SelectRegUpdate(tv.CursorPos)
	}
	tv.SavePosHistory(tv.CursorPos)
}

// SelectAll selects all the text
func (tv *TextView) SelectAll() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.SelectReg.Start = lex.PosZero
	tv.SelectReg.End = tv.Buf.EndPos()
	tv.RenderAllLines()
}

// WordBefore returns the word before the lex.Pos
// uses IsWordBreak to determine the bounds of the word
func (tv *TextView) WordBefore(tp lex.Pos) *textbuf.Edit {
	txt := tv.Buf.Line(tp.Ln)
	ch := tp.Ch
	ch = min(ch, len(txt))
	st := ch
	for i := ch - 1; i >= 0; i-- {
		if i == 0 { // start of line
			st = 0
			break
		}
		r1 := txt[i]
		r2 := txt[i-1]
		if lex.IsWordBreak(r1, r2) {
			st = i + 1
			break
		}
	}
	if st != ch {
		return tv.Buf.Region(lex.Pos{Ln: tp.Ln, Ch: st}, tp)
	}
	return nil
}

// IsWordStart returns true if the cursor is just before the start of a word
// word is a string of characters none of which are classified as a word break
func (tv *TextView) IsWordStart(tp lex.Pos) bool {
	txt := tv.Buf.Line(tv.CursorPos.Ln)
	sz := len(txt)
	if sz == 0 {
		return false
	}
	if tp.Ch >= len(txt) { // end of line
		return false
	}
	if tp.Ch == 0 { // start of line
		r := txt[0]
		if lex.IsWordBreak(r, rune(-1)) {
			return false
		}
		return true
	}
	r1 := txt[tp.Ch-1]
	r2 := txt[tp.Ch]
	if lex.IsWordBreak(r1, rune(-1)) && !lex.IsWordBreak(r2, rune(-1)) {
		return true
	}
	return false
}

// IsWordEnd returns true if the cursor is just past the last letter of a word
// word is a string of characters none of which are classified as a word break
func (tv *TextView) IsWordEnd(tp lex.Pos) bool {
	txt := tv.Buf.Line(tv.CursorPos.Ln)
	sz := len(txt)
	if sz == 0 {
		return false
	}
	if tp.Ch >= len(txt) { // end of line
		r := txt[len(txt)-1]
		if lex.IsWordBreak(r, rune(-1)) {
			return true
		}
		return false
	}
	if tp.Ch == 0 { // start of line
		r := txt[0]
		if lex.IsWordBreak(r, rune(-1)) {
			return false
		}
		return true
	}
	r1 := txt[tp.Ch-1]
	r2 := txt[tp.Ch]
	if !lex.IsWordBreak(r1, rune(-1)) && lex.IsWordBreak(r2, rune(-1)) {
		return true
	}
	return false
}

// IsWordMiddle - returns true if the cursor is anywhere inside a word,
// i.e. the character before the cursor and the one after the cursor
// are not classified as word break characters
func (tv *TextView) IsWordMiddle(tp lex.Pos) bool {
	txt := tv.Buf.Line(tv.CursorPos.Ln)
	sz := len(txt)
	if sz < 2 {
		return false
	}
	if tp.Ch >= len(txt) { // end of line
		return false
	}
	if tp.Ch == 0 { // start of line
		return false
	}
	r1 := txt[tp.Ch-1]
	r2 := txt[tp.Ch]
	if !lex.IsWordBreak(r1, rune(-1)) && !lex.IsWordBreak(r2, rune(-1)) {
		return true
	}
	return false
}

// SelectWord selects the word (whitespace, punctuation delimited) that the cursor is on
// returns true if word selected
func (tv *TextView) SelectWord() bool {
	txt := tv.Buf.Line(tv.CursorPos.Ln)
	sz := len(txt)
	if sz == 0 {
		return false
	}
	reg := tv.WordAt()
	tv.SelectReg = reg
	tv.SelectStart = tv.SelectReg.Start
	return true
}

// WordAt finds the region of the word at the current cursor position
func (tv *TextView) WordAt() (reg textbuf.Region) {
	reg.Start = tv.CursorPos
	reg.End = tv.CursorPos
	txt := tv.Buf.Line(tv.CursorPos.Ln)
	sz := len(txt)
	if sz == 0 {
		return reg
	}
	sch := min(tv.CursorPos.Ch, sz-1)
	if !lex.IsWordBreak(txt[sch], rune(-1)) {
		for sch > 0 {
			r2 := rune(-1)
			if sch-2 >= 0 {
				r2 = txt[sch-2]
			}
			if lex.IsWordBreak(txt[sch-1], r2) {
				break
			}
			sch--
		}
		reg.Start.Ch = sch
		ech := tv.CursorPos.Ch + 1
		for ech < sz {
			r2 := rune(-1)
			if ech < sz-1 {
				r2 = rune(txt[ech+1])
			}
			if lex.IsWordBreak(txt[ech], r2) {
				break
			}
			ech++
		}
		reg.End.Ch = ech
	} else { // keep the space start -- go to next space..
		ech := tv.CursorPos.Ch + 1
		for ech < sz {
			if !lex.IsWordBreak(txt[ech], rune(-1)) {
				break
			}
			ech++
		}
		for ech < sz {
			r2 := rune(-1)
			if ech < sz-1 {
				r2 = rune(txt[ech+1])
			}
			if lex.IsWordBreak(txt[ech], r2) {
				break
			}
			ech++
		}
		reg.End.Ch = ech
	}
	return reg
}

// SelectReset resets the selection
func (tv *TextView) SelectReset() {
	tv.SelectMode = false
	if !tv.HasSelection() {
		return
	}
	stln := tv.SelectReg.Start.Ln
	edln := tv.SelectReg.End.Ln
	tv.SelectReg = textbuf.RegionNil
	tv.PrevSelectReg = textbuf.RegionNil
	tv.RenderLines(stln, edln)
}

// RenderSelectLines renders the lines within the current selection region
func (tv *TextView) RenderSelectLines() {
	if tv.PrevSelectReg == textbuf.RegionNil {
		tv.RenderLines(tv.SelectReg.Start.Ln, tv.SelectReg.End.Ln)
	} else {
		stln := min(tv.SelectReg.Start.Ln, tv.PrevSelectReg.Start.Ln)
		edln := max(tv.SelectReg.End.Ln, tv.PrevSelectReg.End.Ln)
		tv.RenderLines(stln, edln)
	}
	tv.PrevSelectReg = tv.SelectReg
}

///////////////////////////////////////////////////////////////////////////////
//    Cut / Copy / Paste

// TextViewClipHistory is the text view clipboard history -- everything that has been copied
var TextViewClipHistory [][]byte

// Maximum amount of clipboard history to retain
var TextViewClipHistMax = 100

// TextViewClipHistAdd adds the given clipboard bytes to top of history stack
func TextViewClipHistAdd(clip []byte) {
	max := TextViewClipHistMax
	if TextViewClipHistory == nil {
		TextViewClipHistory = make([][]byte, 0, max)
	}

	ch := &TextViewClipHistory

	sz := len(*ch)
	if sz > max {
		*ch = (*ch)[:max]
	}
	if sz >= max {
		copy((*ch)[1:max], (*ch)[0:max-1])
		(*ch)[0] = clip
	} else {
		*ch = append(*ch, nil)
		if sz > 0 {
			copy((*ch)[1:], (*ch)[0:sz])
		}
		(*ch)[0] = clip
	}
}

// TextViewClipHistChooseLen is the max length of clip history to show in chooser
var TextViewClipHistChooseLen = 40

// TextViewClipHistChooseList returns a string slice of length-limited clip history, for chooser
func TextViewClipHistChooseList() []string {
	cl := make([]string, len(TextViewClipHistory))
	for i, hc := range TextViewClipHistory {
		szl := len(hc)
		if szl > TextViewClipHistChooseLen {
			cl[i] = string(hc[:TextViewClipHistChooseLen])
		} else {
			cl[i] = string(hc)
		}
	}
	return cl
}

// PasteHist presents a chooser of clip history items, pastes into text if selected
func (tv *TextView) PasteHist() {
	if TextViewClipHistory == nil {
		return
	}
	cl := TextViewClipHistChooseList()
	gi.StringsChooserPopup(cl, "", tv, func(recv, send ki.Ki, sig int64, data any) {
		ac := send.(*gi.Button)
		idx := ac.Data.(int)
		clip := TextViewClipHistory[idx]
		if clip != nil {
			wupdt := tv.TopUpdateStart()
			defer tv.TopUpdateEnd(wupdt)
			goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Write(mimedata.NewTextBytes(clip))
			tv.InsertAtCursor(clip)
			tv.SavePosHistory(tv.CursorPos)
		}
	})
}

// Cut cuts any selected text and adds it to the clipboard, also returns cut text
func (tv *TextView) Cut() *textbuf.Edit {
	if !tv.HasSelection() {
		return nil
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	org := tv.SelectReg.Start
	cut := tv.DeleteSelection()
	if cut != nil {
		cb := cut.ToBytes()
		goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Write(mimedata.NewTextBytes(cb))
		TextViewClipHistAdd(cb)
	}
	tv.SetCursorShow(org)
	tv.SavePosHistory(tv.CursorPos)
	return cut
}

// DeleteSelection deletes any selected text, without adding to clipboard --
// returns text deleted as textbuf.Edit (nil if none)
func (tv *TextView) DeleteSelection() *textbuf.Edit {
	tbe := tv.Buf.DeleteText(tv.SelectReg.Start, tv.SelectReg.End, EditSignal)
	tv.SelectReset()
	return tbe
}

// Copy copies any selected text to the clipboard, and returns that text,
// optionally resetting the current selection
func (tv *TextView) Copy(reset bool) *textbuf.Edit {
	tbe := tv.Selection()
	if tbe == nil {
		return nil
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	cb := tbe.ToBytes()
	TextViewClipHistAdd(cb)
	goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Write(mimedata.NewTextBytes(cb))
	if reset {
		tv.SelectReset()
	}
	tv.SavePosHistory(tv.CursorPos)
	return tbe
}

// Paste inserts text from the clipboard at current cursor position
func (tv *TextView) Paste() {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	data := goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Read([]string{filecat.TextPlain})
	if data != nil {
		tv.InsertAtCursor(data.TypeData(filecat.TextPlain))
		tv.SavePosHistory(tv.CursorPos)
	}
}

// InsertAtCursor inserts given text at current cursor position
func (tv *TextView) InsertAtCursor(txt []byte) {
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	if tv.HasSelection() {
		tbe := tv.DeleteSelection()
		tv.CursorPos = tbe.AdjustPos(tv.CursorPos, textbuf.AdjustPosDelStart) // move to start if in reg
	}
	tbe := tv.Buf.InsertText(tv.CursorPos, txt, EditSignal)
	if tbe == nil {
		return
	}
	pos := tbe.Reg.End
	if len(txt) == 1 && txt[0] == '\n' {
		pos.Ch = 0 // sometimes it doesn't go to the start..
	}
	tv.SetCursorShow(pos)
	tv.SetCursorCol(tv.CursorPos)
}

///////////////////////////////////////////////////////////
//  Rectangular regions

// TextViewClipRect is the internal clipboard for Rect rectangle-based
// regions -- the raw text is posted on the system clipboard but the
// rect information is in a special format.
var TextViewClipRect *textbuf.Edit

// CutRect cuts rectangle defined by selected text (upper left to lower right)
// and adds it to the clipboard, also returns cut text.
func (tv *TextView) CutRect() *textbuf.Edit {
	if !tv.HasSelection() {
		return nil
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	npos := lex.Pos{Ln: tv.SelectReg.End.Ln, Ch: tv.SelectReg.Start.Ch}
	cut := tv.Buf.DeleteTextRect(tv.SelectReg.Start, tv.SelectReg.End, EditSignal)
	if cut != nil {
		cb := cut.ToBytes()
		goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Write(mimedata.NewTextBytes(cb))
		TextViewClipRect = cut
	}
	tv.SetCursorShow(npos)
	tv.SavePosHistory(tv.CursorPos)
	return cut
}

// CopyRect copies any selected text to the clipboard, and returns that text,
// optionally resetting the current selection
func (tv *TextView) CopyRect(reset bool) *textbuf.Edit {
	tbe := tv.Buf.RegionRect(tv.SelectReg.Start, tv.SelectReg.End)
	if tbe == nil {
		return nil
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	cb := tbe.ToBytes()
	goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).Write(mimedata.NewTextBytes(cb))
	TextViewClipRect = tbe
	if reset {
		tv.SelectReset()
	}
	tv.SavePosHistory(tv.CursorPos)
	return tbe
}

// PasteRect inserts text from the clipboard at current cursor position
func (tv *TextView) PasteRect() {
	if TextViewClipRect == nil {
		return
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	ce := TextViewClipRect.Clone()
	nl := ce.Reg.End.Ln - ce.Reg.Start.Ln
	nch := ce.Reg.End.Ch - ce.Reg.Start.Ch
	ce.Reg.Start.Ln = tv.CursorPos.Ln
	ce.Reg.End.Ln = tv.CursorPos.Ln + nl
	ce.Reg.Start.Ch = tv.CursorPos.Ch
	ce.Reg.End.Ch = tv.CursorPos.Ch + nch
	tbe := tv.Buf.InsertTextRect(ce, EditSignal)

	pos := tbe.Reg.End
	tv.SetCursorShow(pos)
	tv.SetCursorCol(tv.CursorPos)
	tv.SavePosHistory(tv.CursorPos)
}

///////////////////////////////////////////////////////////
//  Context Menu

// ContextMenu displays the context menu with options dependent on situation
func (tv *TextView) ContextMenu() {
	if !tv.HasSelection() && tv.Buf.IsSpellEnabled(tv.CursorPos) {
		if tv.Buf.Spell != nil {
			if tv.OfferCorrect() {
				return
			}
		}
	}
	tv.WidgetBase.ContextMenu()
}

// ContextMenuPos returns the position of the context menu
func (tv *TextView) ContextMenuPos() (pos image.Point) {
	em := tv.EventMgr()
	if em != nil {
		return em.LastMousePos
	}
	return image.Point{100, 100}
}

// MakeContextMenu builds the textview context menu
func (tv *TextView) MakeContextMenu(m *gi.Menu) {
	ac := m.AddButton(gi.ActOpts{Label: "Copy", ShortcutKey: gi.KeyFunCopy}, func(act *gi.Button) {
		tv.Copy(true)
	})
	ac.SetEnabledState(tv.HasSelection())
	if !tv.IsDisabled() {
		ac = m.AddButton(gi.ActOpts{Label: "Cut", ShortcutKey: gi.KeyFunCut},
			tv.This(), func(recv, send ki.Ki, sig int64, data any) {
				tv := recv.Embed(TypeTextView).(*TextView)
				tv.Cut()
			})
		ac.SetEnabledState(tv.HasSelection())
		ac = m.AddButton(gi.ActOpts{Label: "Paste", ShortcutKey: gi.KeyFunPaste},
			tv.This(), func(recv, send ki.Ki, sig int64, data any) {
				tv := recv.Embed(TypeTextView).(*TextView)
				tv.Paste()
			})
		ac.SetState(goosi.TheApp.ClipBoard(tv.ParentRenderWin().RenderWin).IsEmpty(), states.Disabled)
	} else {
		ac = m.AddButton(gi.ActOpts{Label: "Clear"},
			tv.This(), func(recv, send ki.Ki, sig int64, data any) {
				tv := recv.Embed(TypeTextView).(*TextView)
				tv.Clear()
			})
	}
}

///////////////////////////////////////////////////////////////////////////////
//    Complete and Spell

// OfferComplete pops up a menu of possible completions
func (tv *TextView) OfferComplete() {
	if tv.Buf.Complete == nil || tv.ISearch.On || tv.QReplace.On || tv.IsDisabled() {
		return
	}
	tv.Buf.Complete.Cancel()
	if !tv.Buf.Opts.Completion && !tv.ForceComplete {
		return
	}
	if tv.Buf.InComment(tv.CursorPos) || tv.Buf.InLitString(tv.CursorPos) {
		return
	}

	tv.Buf.Complete.SrcLn = tv.CursorPos.Ln
	tv.Buf.Complete.SrcCh = tv.CursorPos.Ch
	st := lex.Pos{tv.CursorPos.Ln, 0}
	en := lex.Pos{tv.CursorPos.Ln, tv.CursorPos.Ch}
	tbe := tv.Buf.Region(st, en)
	var s string
	if tbe != nil {
		s = string(tbe.ToBytes())
		s = strings.TrimLeft(s, " \t") // trim ' ' and '\t'
	}

	//	count := tv.Buf.ByteOffs[tv.CursorPos.Ln] + tv.CursorPos.Ch
	cpos := tv.CharStartPos(tv.CursorPos).ToPoint() // physical location
	cpos.X += 5
	cpos.Y += 10
	tv.Buf.SetByteOffs() // make sure the pos offset is updated!!
	tv.Buf.CurView = tv
	tv.Buf.Complete.Show(s, tv.CursorPos.Ln, tv.CursorPos.Ch, tv.Scene, cpos, tv.ForceComplete)
}

// CancelComplete cancels any pending completion -- call this when new events
// have moved beyond any prior completion scenario
func (tv *TextView) CancelComplete() {
	tv.ForceComplete = false
	if tv.Buf == nil {
		return
	}
	if tv.Buf.Complete == nil {
		return
	}
	if tv.Buf.Complete.Cancel() {
		tv.Buf.CurView = nil
	}
}

// Lookup attempts to lookup symbol at current location, popping up a window
// if something is found
func (tv *TextView) Lookup() {
	if tv.Buf.Complete == nil || tv.ISearch.On || tv.QReplace.On || tv.IsDisabled() {
		return
	}

	var ln int
	var ch int
	if tv.HasSelection() {
		ln = tv.SelectReg.Start.Ln
		if tv.SelectReg.End.Ln != ln {
			return // no multiline selections for lookup
		}
		ch = tv.SelectReg.End.Ch
	} else {
		ln = tv.CursorPos.Ln
		if tv.IsWordEnd(tv.CursorPos) {
			ch = tv.CursorPos.Ch
		} else {
			ch = tv.WordAt().End.Ch
		}
	}
	tv.Buf.Complete.SrcLn = ln
	tv.Buf.Complete.SrcCh = ch
	st := lex.Pos{tv.CursorPos.Ln, 0}
	en := lex.Pos{tv.CursorPos.Ln, ch}

	tbe := tv.Buf.Region(st, en)
	var s string
	if tbe != nil {
		s = string(tbe.ToBytes())
		s = strings.TrimLeft(s, " \t") // trim ' ' and '\t'
	}

	//	count := tv.Buf.ByteOffs[tv.CursorPos.Ln] + tv.CursorPos.Ch
	cpos := tv.CharStartPos(tv.CursorPos).ToPoint() // physical location
	cpos.X += 5
	cpos.Y += 10
	tv.Buf.SetByteOffs() // make sure the pos offset is updated!!
	tv.Buf.CurView = tv
	tv.Buf.Complete.Lookup(s, tv.CursorPos.Ln, tv.CursorPos.Ch, tv.Scene, cpos, tv.ForceComplete)
}

// ISpellKeyInput locates the word to spell check based on cursor position and
// the key input, then passes the text region to SpellCheck
func (tv *TextView) ISpellKeyInput(kt events.Event) {
	if !tv.Buf.IsSpellEnabled(tv.CursorPos) {
		return
	}

	isDoc := tv.Buf.Info.Cat == filecat.Doc
	tp := tv.CursorPos

	kf := gi.KeyFun(kt.KeyChord())
	switch kf {
	case gi.KeyFunMoveUp:
		if isDoc {
			tv.Buf.SpellCheckLineTag(tp.Ln)
		}
	case gi.KeyFunMoveDown:
		if isDoc {
			tv.Buf.SpellCheckLineTag(tp.Ln)
		}
	case gi.KeyFunMoveRight:
		if tv.IsWordEnd(tp) {
			reg := tv.WordBefore(tp)
			tv.SpellCheck(reg)
			break
		}
		if tp.Ch == 0 { // end of line
			tp.Ln--
			if isDoc {
				tv.Buf.SpellCheckLineTag(tp.Ln) // redo prior line
			}
			tp.Ch = tv.Buf.LineLen(tp.Ln)
			reg := tv.WordBefore(tp)
			tv.SpellCheck(reg)
			break
		}
		txt := tv.Buf.Line(tp.Ln)
		var r rune
		atend := false
		if tp.Ch >= len(txt) {
			atend = true
			tp.Ch++
		} else {
			r = txt[tp.Ch]
		}
		if atend || lex.IsWordBreak(r, rune(-1)) {
			tp.Ch-- // we are one past the end of word
			reg := tv.WordBefore(tp)
			tv.SpellCheck(reg)
		}
	case gi.KeyFunEnter:
		tp.Ln--
		if isDoc {
			tv.Buf.SpellCheckLineTag(tp.Ln) // redo prior line
		}
		tp.Ch = tv.Buf.LineLen(tp.Ln)
		reg := tv.WordBefore(tp)
		tv.SpellCheck(reg)
	case gi.KeyFunFocusNext:
		tp.Ch-- // we are one past the end of word
		reg := tv.WordBefore(tp)
		tv.SpellCheck(reg)
	case gi.KeyFunBackspace, gi.KeyFunDelete:
		if tv.IsWordMiddle(tv.CursorPos) {
			reg := tv.WordAt()
			tv.SpellCheck(tv.Buf.Region(reg.Start, reg.End))
		} else {
			reg := tv.WordBefore(tp)
			tv.SpellCheck(reg)
		}
	case gi.KeyFunNil:
		if unicode.IsSpace(kt.Rune) || unicode.IsPunct(kt.Rune) && kt.Rune != '\'' { // contractions!
			tp.Ch-- // we are one past the end of word
			reg := tv.WordBefore(tp)
			tv.SpellCheck(reg)
		} else {
			if tv.IsWordMiddle(tv.CursorPos) {
				reg := tv.WordAt()
				tv.SpellCheck(tv.Buf.Region(reg.Start, reg.End))
			}
		}
	}
}

// SpellCheck offers spelling corrections if we are at a word break or other word termination
// and the word before the break is unknown -- returns true if misspelled word found
func (tv *TextView) SpellCheck(reg *textbuf.Edit) bool {
	if tv.Buf.Spell == nil {
		return false
	}
	wb := string(reg.ToBytes())
	lwb := lex.FirstWordApostrophe(wb) // only lookup words
	if len(lwb) <= 2 {
		return false
	}
	widx := strings.Index(wb, lwb) // adjust region for actual part looking up
	ld := len(wb) - len(lwb)
	reg.Reg.Start.Ch += widx
	reg.Reg.End.Ch += widx - ld

	sugs, knwn := tv.Buf.Spell.CheckWord(lwb)
	if knwn {
		tv.Buf.RemoveTag(reg.Reg.Start, token.TextSpellErr)
		ln := reg.Reg.Start.Ln
		tv.LayoutLines(ln, ln, false)
		tv.RenderLines(ln, ln)
		return false
	}
	// fmt.Printf("spell err: %s\n", wb)
	tv.Buf.Spell.SetWord(wb, sugs, reg.Reg.Start.Ln, reg.Reg.Start.Ch)
	tv.Buf.RemoveTag(reg.Reg.Start, token.TextSpellErr)
	tv.Buf.AddTagEdit(reg, token.TextSpellErr)
	ln := reg.Reg.Start.Ln
	tv.LayoutLines(ln, ln, false)
	tv.RenderLines(ln, ln)
	return true
}

// OfferCorrect pops up a menu of possible spelling corrections for word at
// current CursorPos -- if no misspelling there or not in spellcorrect mode
// returns false
func (tv *TextView) OfferCorrect() bool {
	if tv.Buf.Spell == nil || tv.ISearch.On || tv.QReplace.On || tv.IsDisabled() {
		return false
	}
	sel := tv.SelectReg
	if !tv.SelectWord() {
		tv.SelectReg = sel
		return false
	}
	tbe := tv.Selection()
	if tbe == nil {
		tv.SelectReg = sel
		return false
	}
	tv.SelectReg = sel
	wb := string(tbe.ToBytes())
	wbn := strings.TrimLeft(wb, " \t")
	if len(wb) != len(wbn) {
		return false // SelectWord captures leading whitespace - don't offer if there is leading whitespace
	}
	sugs, knwn := tv.Buf.Spell.CheckWord(wb)
	if knwn && !tv.Buf.Spell.IsLastLearned(wb) {
		return false
	}
	tv.Buf.Spell.SetWord(wb, sugs, tbe.Reg.Start.Ln, tbe.Reg.Start.Ch)

	cpos := tv.CharStartPos(tv.CursorPos).ToPoint() // physical location
	cpos.X += 5
	cpos.Y += 10
	tv.Buf.CurView = tv
	tv.Buf.Spell.Show(wb, tv.Scene, cpos)
	return true
}

// CancelCorrect cancels any pending spell correction -- call this when new events
// have moved beyond any prior correction scenario
func (tv *TextView) CancelCorrect() {
	if tv.Buf.Spell == nil || tv.ISearch.On || tv.QReplace.On {
		return
	}
	if !tv.Buf.Opts.SpellCorrect {
		return
	}
	tv.Buf.CurView = nil
	tv.Buf.Spell.Cancel()
}

///////////////////////////////////////////////////////////////////////////////
//    Scrolling

// ScrollInView tells any parent scroll layout to scroll to get given box
// (e.g., cursor BBox) in view -- returns true if scrolled
func (tv *TextView) ScrollInView(bbox image.Rectangle) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.ScrollToBox(bbox)
}

// ScrollCursorInView tells any parent scroll layout to scroll to get cursor
// in view -- returns true if scrolled
func (tv *TextView) ScrollCursorInView() bool {
	if tv == nil || tv.This() == nil {
		return false
	}
	if tv.This().(gi.Widget).IsVisible() {
		curBBox := tv.CursorBBox(tv.CursorPos)
		return tv.ScrollInView(curBBox)
	}
	return false
}

// AutoScroll tells any parent scroll layout to scroll to do its autoscroll
// based on given location -- for dragging
func (tv *TextView) AutoScroll(pos image.Point) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.AutoScroll(pos)
}

// ScrollCursorToCenterIfHidden checks if the cursor is not visible, and if
// so, scrolls to the center, along both dimensions.
func (tv *TextView) ScrollCursorToCenterIfHidden() bool {
	curBBox := tv.CursorBBox(tv.CursorPos)
	did := false
	if (curBBox.Min.Y-int(tv.LineHeight)) < tv.ScBBox.Min.Y || (curBBox.Max.Y+int(tv.LineHeight)) > tv.ScBBox.Max.Y {
		did = tv.ScrollCursorToVertCenter()
	}
	if curBBox.Max.X < tv.ScBBox.Min.X || curBBox.Min.X > tv.ScBBox.Max.X {
		did = did || tv.ScrollCursorToHorizCenter()
	}
	return did
}

///////////////////////////////////////////////////////////////////////////////
//    Scrolling -- Vertical

// ScrollToTop tells any parent scroll layout to scroll to get given vertical
// coordinate at top of view to extent possible -- returns true if scrolled
func (tv *TextView) ScrollToTop(pos int) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.ScrollDimToStart(mat32.Y, pos)
}

// ScrollCursorToTop tells any parent scroll layout to scroll to get cursor
// at top of view to extent possible -- returns true if scrolled.
func (tv *TextView) ScrollCursorToTop() bool {
	curBBox := tv.CursorBBox(tv.CursorPos)
	return tv.ScrollToTop(curBBox.Min.Y)
}

// ScrollToBottom tells any parent scroll layout to scroll to get given
// vertical coordinate at bottom of view to extent possible -- returns true if
// scrolled
func (tv *TextView) ScrollToBottom(pos int) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.ScrollDimToEnd(mat32.Y, pos)
}

// ScrollCursorToBottom tells any parent scroll layout to scroll to get cursor
// at bottom of view to extent possible -- returns true if scrolled.
func (tv *TextView) ScrollCursorToBottom() bool {
	curBBox := tv.CursorBBox(tv.CursorPos)
	return tv.ScrollToBottom(curBBox.Max.Y)
}

// ScrollToVertCenter tells any parent scroll layout to scroll to get given
// vertical coordinate to center of view to extent possible -- returns true if
// scrolled
func (tv *TextView) ScrollToVertCenter(pos int) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.ScrollDimToCenter(mat32.Y, pos)
}

// ScrollCursorToVertCenter tells any parent scroll layout to scroll to get
// cursor at vert center of view to extent possible -- returns true if
// scrolled.
func (tv *TextView) ScrollCursorToVertCenter() bool {
	curBBox := tv.CursorBBox(tv.CursorPos)
	mid := (curBBox.Min.Y + curBBox.Max.Y) / 2
	return tv.ScrollToVertCenter(mid)
}

///////////////////////////////////////////////////////////////////////////////
//    Scrolling -- Horizontal

// ScrollToLeft tells any parent scroll layout to scroll to get given
// horizontal coordinate at left of view to extent possible -- returns true if
// scrolled
func (tv *TextView) ScrollToLeft(pos int) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.ScrollDimToStart(mat32.X, pos)
}

// ScrollCursorToLeft tells any parent scroll layout to scroll to get cursor
// at left of view to extent possible -- returns true if scrolled.
func (tv *TextView) ScrollCursorToLeft() bool {
	_, ri, _ := tv.WrappedLineNo(tv.CursorPos)
	if ri <= 0 {
		return tv.ScrollToLeft(tv.ObjBBox.Min.X - int(tv.Style.BoxSpace().Left) - 2)
	}
	curBBox := tv.CursorBBox(tv.CursorPos)
	return tv.ScrollToLeft(curBBox.Min.X)
}

// ScrollToRight tells any parent scroll layout to scroll to get given
// horizontal coordinate at right of view to extent possible -- returns true
// if scrolled
func (tv *TextView) ScrollToRight(pos int) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.ScrollDimToEnd(mat32.X, pos)
}

// ScrollCursorToRight tells any parent scroll layout to scroll to get cursor
// at right of view to extent possible -- returns true if scrolled.
func (tv *TextView) ScrollCursorToRight() bool {
	curBBox := tv.CursorBBox(tv.CursorPos)
	return tv.ScrollToRight(curBBox.Max.X)
}

// ScrollToHorizCenter tells any parent scroll layout to scroll to get given
// horizontal coordinate to center of view to extent possible -- returns true if
// scrolled
func (tv *TextView) ScrollToHorizCenter(pos int) bool {
	ly := tv.ParentScrollLayout()
	if ly == nil {
		return false
	}
	return ly.ScrollDimToCenter(mat32.X, pos)
}

// ScrollCursorToHorizCenter tells any parent scroll layout to scroll to get
// cursor at horiz center of view to extent possible -- returns true if
// scrolled.
func (tv *TextView) ScrollCursorToHorizCenter() bool {
	curBBox := tv.CursorBBox(tv.CursorPos)
	mid := (curBBox.Min.X + curBBox.Max.X) / 2
	return tv.ScrollToHorizCenter(mid)
}

///////////////////////////////////////////////////////////////////////////////
//    Rendering

// CharStartPos returns the starting (top left) render coords for the given
// position -- makes no attempt to rationalize that pos (i.e., if not in
// visible range, position will be out of range too)
func (tv *TextView) CharStartPos(pos lex.Pos) mat32.Vec2 {
	spos := tv.RenderStartPos()
	spos.X += tv.LineNoOff
	if pos.Ln >= len(tv.Offs) {
		if len(tv.Offs) > 0 {
			pos.Ln = len(tv.Offs) - 1
		} else {
			return spos
		}
	} else {
		spos.Y += tv.Offs[pos.Ln] + mat32.FromFixed(tv.Style.Font.Face.Face.Metrics().Descent)
	}
	if len(tv.Renders[pos.Ln].Spans) > 0 {
		// note: Y from rune pos is baseline
		rrp, _, _, _ := tv.Renders[pos.Ln].RuneRelPos(pos.Ch)
		spos.X += rrp.X
		spos.Y += rrp.Y - tv.Renders[pos.Ln].Spans[0].RelPos.Y // relative
	}
	return spos
}

// CharEndPos returns the ending (bottom right) render coords for the given
// position -- makes no attempt to rationalize that pos (i.e., if not in
// visible range, position will be out of range too)
func (tv *TextView) CharEndPos(pos lex.Pos) mat32.Vec2 {
	spos := tv.RenderStartPos()
	pos.Ln = min(pos.Ln, tv.NLines-1)
	if pos.Ln < 0 {
		spos.Y += float32(tv.LinesSize.Y)
		spos.X += tv.LineNoOff
		return spos
	}
	// if pos.Ln >= tv.NLines {
	// 	spos.Y += float32(tv.LinesSize.Y)
	// 	spos.X += tv.LineNoOff
	// 	return spos
	// }
	spos.Y += tv.Offs[pos.Ln] + mat32.FromFixed(tv.Style.Font.Face.Face.Metrics().Descent)
	spos.X += tv.LineNoOff
	if len(tv.Renders[pos.Ln].Spans) > 0 {
		// note: Y from rune pos is baseline
		rrp, _, _, _ := tv.Renders[pos.Ln].RuneEndPos(pos.Ch)
		spos.X += rrp.X
		spos.Y += rrp.Y - tv.Renders[pos.Ln].Spans[0].RelPos.Y // relative
	}
	spos.Y += tv.LineHeight // end of that line
	return spos
}

// TextViewBlinkMu is mutex protecting TextViewBlink updating and access
var TextViewBlinkMu sync.Mutex

// TextViewBlinker is the time.Ticker for blinking cursors for text fields,
// only one of which can be active at at a time
var TextViewBlinker *time.Ticker

// BlinkingTextView is the text field that is blinking
var BlinkingTextView *TextView

// TextViewSpriteName is the name of the window sprite used for the cursor
var TextViewSpriteName = "giv.TextView.Cursor"

// TextViewBlink is function that blinks text field cursor
func TextViewBlink() {
	for {
		TextViewBlinkMu.Lock()
		if TextViewBlinker == nil {
			TextViewBlinkMu.Unlock()
			return // shutdown..
		}
		TextViewBlinkMu.Unlock()
		<-TextViewBlinker.C
		TextViewBlinkMu.Lock()
		if BlinkingTextView == nil || BlinkingTextView.This() == nil {
			TextViewBlinkMu.Unlock()
			continue
		}
		if BlinkingTextView.Is(ki.Destroyed) || BlinkingTextView.Is(ki.Deleted) {
			BlinkingTextView = nil
			TextViewBlinkMu.Unlock()
			continue
		}
		tv := BlinkingTextView
		if tv.Scene == nil || !tv.StateIs(states.Focused) || !tv.IsFocusActive() || !tv.This().(gi.Widget).IsVisible() {
			tv.RenderCursor(false)
			BlinkingTextView = nil
			TextViewBlinkMu.Unlock()
			continue
		}
		win := tv.ParentRenderWin()
		if win == nil || win.Is(WinResizing) || win.IsClosed() || !win.IsRenderWinInFocus() {
			TextViewBlinkMu.Unlock()
			continue
		}
		if win.Is(Updating) {
			TextViewBlinkMu.Unlock()
			continue
		}
		tv.BlinkOn = !tv.BlinkOn
		tv.RenderCursor(tv.BlinkOn)
		TextViewBlinkMu.Unlock()
	}
}

// StartCursor starts the cursor blinking and renders it
func (tv *TextView) StartCursor() {
	if tv == nil || tv.This() == nil {
		return
	}
	if !tv.This().(gi.Widget).IsVisible() {
		return
	}
	tv.BlinkOn = true
	if gi.CursorBlinkTime == 0 {
		tv.RenderCursor(true)
		return
	}
	TextViewBlinkMu.Lock()
	if TextViewBlinker == nil {
		TextViewBlinker = time.NewTicker(gi.CursorBlinkTime)
		go TextViewBlink()
	}
	tv.BlinkOn = true
	win := tv.ParentRenderWin()
	if win != nil && !win.Is(WinResizing) {
		tv.RenderCursor(true)
	}
	//	fmt.Printf("set blink tv: %v\n", tv.Path())
	BlinkingTextView = tv
	TextViewBlinkMu.Unlock()
}

// StopCursor stops the cursor from blinking
func (tv *TextView) StopCursor() {
	if tv == nil || tv.This() == nil {
		return
	}
	// if !tv.This().(gi.Widget).IsVisible() {
	// 	return
	// }
	tv.RenderCursor(false)
	TextViewBlinkMu.Lock()
	if BlinkingTextView == tv {
		BlinkingTextView = nil
	}
	TextViewBlinkMu.Unlock()
}

// CursorBBox returns a bounding-box for a cursor at given position
func (tv *TextView) CursorBBox(pos lex.Pos) image.Rectangle {
	cpos := tv.CharStartPos(pos)
	cbmin := cpos.SubScalar(tv.CursorWidth.Dots)
	cbmax := cpos.AddScalar(tv.CursorWidth.Dots)
	cbmax.Y += tv.FontHeight
	curBBox := image.Rectangle{cbmin.ToPointFloor(), cbmax.ToPointCeil()}
	return curBBox
}

// RenderCursor renders the cursor on or off, as a sprite that is either on or off
func (tv *TextView) RenderCursor(on bool) {
	if tv == nil || tv.This() == nil {
		return
	}
	if !tv.This().(gi.Widget).IsVisible() {
		return
	}
	if tv.Renders == nil {
		return
	}
	tv.CursorMu.Lock()
	defer tv.CursorMu.Unlock()

	win := tv.ParentRenderWin()
	sp := tv.CursorSprite()
	if on {
		win.ActivateSprite(sp.Name)
	} else {
		win.InactivateSprite(sp.Name)
	}
	sp.Geom.Pos = tv.CharStartPos(tv.CursorPos).ToPointFloor()
	win.UpdateSig()
}

// CursorSpriteName returns the name of the cursor sprite
func (tv *TextView) CursorSpriteName() string {
	spnm := fmt.Sprintf("%v-%v", TextViewSpriteName, tv.FontHeight)
	return spnm
}

// CursorSprite returns the sprite for the cursor, which is
// only rendered once with a vertical bar, and just activated and inactivated
// depending on render status.
func (tv *TextView) CursorSprite() *gi.Sprite {
	win := tv.ParentRenderWin()
	if win == nil {
		return nil
	}
	spnm := tv.CursorSpriteName()
	sp, ok := win.SpriteByName(spnm)
	if !ok {
		bbsz := image.Point{int(mat32.Ceil(tv.CursorWidth.Dots)), int(mat32.Ceil(tv.FontHeight))}
		if bbsz.X < 2 { // at least 2
			bbsz.X = 2
		}
		sp = gi.NewSprite(spnm, bbsz, image.Point{})
		ibox := sp.Pixels.Bounds()
		draw.Draw(sp.Pixels, ibox, &image.Uniform{tv.CursorColor.Color}, image.Point{}, draw.Src)
		win.AddSprite(sp)
	}
	return sp
}

// TextViewDepthOffsets are changes in color values from default background for different
// depths.  For dark mode, these are increments, for light mode they are decrements.
var TextViewDepthColors = []color.RGBA{
	{0, 0, 0, 0},
	{5, 5, 0, 0},
	{15, 15, 0, 0},
	{5, 15, 0, 0},
	{0, 15, 5, 0},
	{0, 15, 15, 0},
	{0, 5, 15, 0},
	{5, 0, 15, 0},
	{5, 0, 5, 0},
}

// RenderDepthBg renders the depth background color
func (tv *TextView) RenderDepthBg(stln, edln int) {
	if tv.Buf == nil {
		return
	}
	if !tv.Buf.Opts.DepthColor || tv.IsDisabled() || !tv.StateIs(states.Focused) || !tv.IsFocusActive() {
		return
	}
	tv.Buf.MarkupMu.RLock() // needed for HiTags access
	defer tv.Buf.MarkupMu.RUnlock()
	sty := &tv.Style
	cspec := sty.BackgroundColor
	bg := cspec.Color
	// STYTODO: fix textview colors
	// isDark := bg.IsDark()
	// nclrs := len(TextViewDepthColors)
	lstdp := 0
	for ln := stln; ln <= edln; ln++ {
		lst := tv.CharStartPos(lex.Pos{Ln: ln}).Y // note: charstart pos includes descent
		led := lst + mat32.Max(tv.Renders[ln].Size.Y, tv.LineHeight)
		if int(mat32.Ceil(led)) < tv.ScBBox.Min.Y {
			continue
		}
		if int(mat32.Floor(lst)) > tv.ScBBox.Max.Y {
			continue
		}
		if ln >= len(tv.Buf.HiTags) { // may be out of sync
			continue
		}
		ht := tv.Buf.HiTags[ln]
		lsted := 0
		for ti := range ht {
			lx := &ht[ti]
			if lx.Tok.Depth > 0 {
				cspec.Color = bg
				// if isDark {
				// 	// reverse order too
				// 	cspec.Color.Add(TextViewDepthColors[nclrs-1-lx.Tok.Depth%nclrs])
				// } else {
				// 	cspec.Color.Sub(TextViewDepthColors[lx.Tok.Depth%nclrs])
				// }
				st := min(lsted, lx.St)
				reg := textbuf.Region{Start: lex.Pos{Ln: ln, Ch: st}, End: lex.Pos{Ln: ln, Ch: lx.Ed}}
				lsted = lx.Ed
				lstdp = lx.Tok.Depth
				tv.RenderRegionBoxSty(reg, sty, &cspec)
			}
		}
		if lstdp > 0 {
			tv.RenderRegionToEnd(lex.Pos{Ln: ln, Ch: lsted}, sty, &cspec)
		}
	}
}

// RenderSelect renders the selection region as a selected background color
// -- always called within context of outer RenderLines or RenderAllLines
func (tv *TextView) RenderSelect() {
	if !tv.HasSelection() {
		return
	}
	tv.RenderRegionBox(tv.SelectReg, &tv.SelectColor)
}

// RenderHighlights renders the highlight regions as a highlighted background
// color -- always called within context of outer RenderLines or
// RenderAllLines
func (tv *TextView) RenderHighlights(stln, edln int) {
	for _, reg := range tv.Highlights {
		reg := tv.Buf.AdjustReg(reg)
		if reg.IsNil() || (stln >= 0 && (reg.Start.Ln > edln || reg.End.Ln < stln)) {
			continue
		}
		tv.RenderRegionBox(reg, &tv.HighlightColor)
	}
}

// RenderScopelights renders a highlight background color for regions
// in the Scopelights list
// -- always called within context of outer RenderLines or RenderAllLines
func (tv *TextView) RenderScopelights(stln, edln int) {
	for _, reg := range tv.Scopelights {
		reg := tv.Buf.AdjustReg(reg)
		if reg.IsNil() || (stln >= 0 && (reg.Start.Ln > edln || reg.End.Ln < stln)) {
			continue
		}
		tv.RenderRegionBox(reg, &tv.HighlightColor)
	}
}

// UpdateHighlights re-renders lines from previous highlights and current
// highlights -- assumed to be within a window update block
func (tv *TextView) UpdateHighlights(prev []textbuf.Region) {
	for _, ph := range prev {
		ph := tv.Buf.AdjustReg(ph)
		tv.RenderLines(ph.Start.Ln, ph.End.Ln)
	}
	for _, ch := range tv.Highlights {
		ch := tv.Buf.AdjustReg(ch)
		tv.RenderLines(ch.Start.Ln, ch.End.Ln)
	}
}

// ClearHighlights clears the Highlights slice of all regions
func (tv *TextView) ClearHighlights() {
	if len(tv.Highlights) == 0 {
		return
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	tv.Highlights = tv.Highlights[:0]
	tv.RenderAllLines()
}

// ClearScopelights clears the Highlights slice of all regions
func (tv *TextView) ClearScopelights() {
	if len(tv.Scopelights) == 0 {
		return
	}
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)
	sl := make([]textbuf.Region, len(tv.Scopelights))
	copy(sl, tv.Scopelights)
	tv.Scopelights = tv.Scopelights[:0]
	for _, si := range sl {
		ln := si.Start.Ln
		tv.RenderLines(ln, ln)
	}
}

// RenderRegionBox renders a region in background color according to given background color
func (tv *TextView) RenderRegionBox(reg textbuf.Region, bgclr *colors.Full) {
	tv.RenderRegionBoxSty(reg, &tv.Style, bgclr)
}

// RenderRegionBoxSty renders a region in given style and background color
func (tv *TextView) RenderRegionBoxSty(reg textbuf.Region, sty *styles.Style, bgclr *colors.Full) {
	st := reg.Start
	ed := reg.End
	spos := tv.CharStartPos(st)
	epos := tv.CharStartPos(ed)
	epos.Y += tv.LineHeight
	if int(mat32.Ceil(epos.Y)) < tv.ScBBox.Min.Y || int(mat32.Floor(spos.Y)) > tv.ScBBox.Max.Y {
		return
	}

	rs := tv.Render()
	pc := &rs.Paint
	spc := sty.BoxSpace()

	rst := tv.RenderStartPos()
	// SidesTODO: this is sketchy
	ex := float32(tv.ScBBox.Max.X) - spc.Right
	sx := rst.X + tv.LineNoOff

	// fmt.Printf("select: %v -- %v\n", st, ed)

	stsi, _, _ := tv.WrappedLineNo(st)
	edsi, _, _ := tv.WrappedLineNo(ed)
	if st.Ln == ed.Ln && stsi == edsi {
		pc.FillBox(rs, spos, epos.Sub(spos), bgclr) // same line, done
		return
	}
	// on diff lines: fill to end of stln
	seb := spos
	seb.Y += tv.LineHeight
	seb.X = ex
	pc.FillBox(rs, spos, seb.Sub(spos), bgclr)
	sfb := seb
	sfb.X = sx
	if sfb.Y < epos.Y { // has some full box
		efb := epos
		efb.Y -= tv.LineHeight
		efb.X = ex
		pc.FillBox(rs, sfb, efb.Sub(sfb), bgclr)
	}
	sed := epos
	sed.Y -= tv.LineHeight
	sed.X = sx
	pc.FillBox(rs, sed, epos.Sub(sed), bgclr)
}

// RenderRegionToEnd renders a region in given style and background color, to end of line from start
func (tv *TextView) RenderRegionToEnd(st lex.Pos, sty *styles.Style, bgclr *colors.Full) {
	spos := tv.CharStartPos(st)
	epos := spos
	epos.Y += tv.LineHeight
	epos.X = float32(tv.ScBBox.Max.X)
	if int(mat32.Ceil(epos.Y)) < tv.ScBBox.Min.Y || int(mat32.Floor(spos.Y)) > tv.ScBBox.Max.Y {
		return
	}

	rs := tv.Render()
	pc := &rs.Paint

	pc.FillBox(rs, spos, epos.Sub(spos), bgclr) // same line, done
}

// RenderStartPos is absolute rendering start position from our allocpos
func (tv *TextView) RenderStartPos() mat32.Vec2 {
	st := &tv.Style
	spc := st.BoxSpace()
	pos := tv.LayState.Alloc.Pos.Add(spc.Pos())
	return pos
}

// VisSizes computes the visible size of view given current parameters
func (tv *TextView) VisSizes() {
	if tv.Style.Font.Size.Val == 0 { // called under lock
		tv.StyleTextView()
	}
	sty := &tv.Style
	spc := sty.BoxSpace()
	sty.Font = paint.OpenFont(sty.FontRender(), &sty.UnContext)
	tv.FontHeight = sty.Font.Face.Metrics.Height
	tv.LineHeight = sty.Text.EffLineHeight(tv.FontHeight)
	sz := tv.ScBBox.Size()
	if sz == (image.Point{}) {
		tv.VisSize.Y = 40
		tv.VisSize.X = 80
	} else {
		tv.VisSize.Y = int(mat32.Floor(float32(sz.Y) / tv.LineHeight))
		tv.VisSize.X = int(mat32.Floor(float32(sz.X) / sty.Font.Face.Metrics.Ch))
	}
	tv.LineNoDigs = max(1+int(mat32.Log10(float32(tv.NLines))), 3)
	lno := true
	if tv.Buf != nil {
		lno = tv.Buf.Opts.LineNos
	}
	if lno {
		tv.SetFlag(int(TextViewHasLineNos))
		// SidesTODO: this is sketchy
		tv.LineNoOff = float32(tv.LineNoDigs+3)*sty.Font.Face.Metrics.Ch + spc.Left // space for icon
	} else {
		tv.ClearFlag(int(TextViewHasLineNos))
		tv.LineNoOff = 0
	}
	tv.RenderSize()
}

// RenderAllLines displays all the visible lines on the screen -- this is
// called outside of update process and has its own bounds check and updating
func (tv *TextView) RenderAllLines() {
	if tv == nil || tv.This() == nil {
		return
	}
	if !tv.This().(gi.Widget).IsVisible() {
		return
	}
	rs := tv.Render()
	rs.PushBounds(tv.ScBBox)
	wupdt := tv.TopUpdateStart()
	tv.RenderAllLinesInBounds()
	tv.PopBounds()
	tv.Scene.This().(gi.Scene).ScUploadRegion(tv.ScBBox, tv.WinBBox)
	tv.RenderScrolls()
	tv.TopUpdateEnd(wupdt)
}

// RenderAllLinesInBounds displays all the visible lines on the screen --
// after PushBounds has already been called
func (tv *TextView) RenderAllLinesInBounds() {
	// fmt.Printf("render all: %v\n", tv.Nm)
	rs := tv.Render()
	rs.Lock()
	pc := &rs.Paint
	sty := &tv.Style
	tv.VisSizes()
	pos := mat32.NewVec2FmPoint(tv.ScBBox.Min)
	epos := mat32.NewVec2FmPoint(tv.ScBBox.Max)
	pc.FillBox(rs, pos, epos.Sub(pos), &sty.BackgroundColor)
	pos = tv.RenderStartPos()
	stln := -1
	edln := -1
	for ln := 0; ln < tv.NLines; ln++ {
		lst := pos.Y + tv.Offs[ln]
		led := lst + mat32.Max(tv.Renders[ln].Size.Y, tv.LineHeight)
		if int(mat32.Ceil(led)) < tv.ScBBox.Min.Y {
			continue
		}
		if int(mat32.Floor(lst)) > tv.ScBBox.Max.Y {
			continue
		}
		if stln < 0 {
			stln = ln
		}
		edln = ln
	}

	if stln < 0 || edln < 0 { // shouldn't happen.
		rs.Unlock()
		return
	}

	if tv.HasLineNos() {
		tv.RenderLineNosBoxAll()
		for ln := stln; ln <= edln; ln++ {
			tv.RenderLineNo(ln, false, false) // don't re-render std fill boxes, no separate vp upload
		}
	}

	tv.RenderDepthBg(stln, edln)
	tv.RenderHighlights(stln, edln)
	tv.RenderScopelights(stln, edln)
	tv.RenderSelect()
	if tv.HasLineNos() {
		tbb := tv.ScBBox
		tbb.Min.X += int(tv.LineNoOff)
		rs.Unlock()
		rs.PushBounds(tbb)
		rs.Lock()
	}
	for ln := stln; ln <= edln; ln++ {
		lst := pos.Y + tv.Offs[ln]
		lp := pos
		lp.Y = lst
		lp.X += tv.LineNoOff
		tv.Renders[ln].Render(rs, lp) // not top pos -- already has baseline offset
	}
	rs.Unlock()
	if tv.HasLineNos() {
		rs.PopBounds()
	}
}

// RenderLineNosBoxAll renders the background for the line numbers in the LineNumberColor
func (tv *TextView) RenderLineNosBoxAll() {
	if !tv.HasLineNos() {
		return
	}
	rs := tv.Render()
	pc := &rs.Paint
	sty := &tv.Style
	spc := sty.BoxSpace()
	spos := mat32.NewVec2FmPoint(tv.ScBBox.Min)
	epos := mat32.NewVec2FmPoint(tv.ScBBox.Max)
	// SidesTODO: this is sketchy
	epos.X = spos.X + tv.LineNoOff - spc.Size().X/2
	pc.FillBoxColor(rs, spos, epos.Sub(spos), tv.LineNumberColor.Color)
}

// RenderLineNosBox renders the background for the line numbers in given range, in the LineNumberColor
func (tv *TextView) RenderLineNosBox(st, ed int) {
	if !tv.HasLineNos() {
		return
	}
	rs := tv.Render()
	pc := &rs.Paint
	sty := &tv.Style
	spc := sty.BoxSpace()
	spos := tv.CharStartPos(lex.Pos{Ln: st})
	spos.X = float32(tv.ScBBox.Min.X)
	epos := tv.CharEndPos(lex.Pos{Ln: ed + 1})
	epos.Y -= tv.LineHeight
	// SidesTODO: this is sketchy
	epos.X = spos.X + tv.LineNoOff - spc.Size().X/2
	// fmt.Printf("line box: st %v ed: %v spos %v  epos %v\n", st, ed, spos, epos)
	pc.FillBoxColor(rs, spos, epos.Sub(spos), tv.LineNumberColor.Color)
}

// RenderLineNo renders given line number -- called within context of other render
// if defFill is true, it fills box color for default background color (use false for batch mode)
// and if vpUpload is true it uploads the rendered region to scene directly
// (only if totally separate from other updates)
func (tv *TextView) RenderLineNo(ln int, defFill bool, vpUpload bool) {
	if !tv.HasLineNos() || tv.Buf == nil {
		return
	}

	vp := tv.Scene
	sty := &tv.Style
	spc := sty.BoxSpace()
	fst := sty.FontRender()
	rs := &vp.Render
	pc := &rs.Paint

	// render fillbox
	sbox := tv.CharStartPos(lex.Pos{Ln: ln})
	sbox.X = float32(tv.ScBBox.Min.X)
	ebox := tv.CharEndPos(lex.Pos{Ln: ln + 1})
	if ln < tv.NLines-1 {
		ebox.Y -= tv.LineHeight
	}
	// SidesTODO: this is sketchy
	ebox.X = sbox.X + tv.LineNoOff - spc.Size().X/2
	bsz := ebox.Sub(sbox)
	lclr, hasLClr := tv.Buf.LineColors[ln]
	if tv.CursorPos.Ln == ln {
		if hasLClr { // split the diff!
			bszhlf := bsz
			bszhlf.X /= 2
			pc.FillBoxColor(rs, sbox, bszhlf, lclr)
			nsp := sbox
			nsp.X += bszhlf.X
			pc.FillBoxColor(rs, nsp, bszhlf, tv.SelectColor.Color)
		} else {
			pc.FillBoxColor(rs, sbox, bsz, tv.SelectColor.Color)
		}
	} else if hasLClr {
		pc.FillBoxColor(rs, sbox, bsz, lclr)
	} else if defFill {
		pc.FillBoxColor(rs, sbox, bsz, tv.LineNumberColor.Color)
	}

	fst.BackgroundColor.SetColor(nil)
	lfmt := fmt.Sprintf("%d", tv.LineNoDigs)
	lfmt = "%" + lfmt + "d"
	lnstr := fmt.Sprintf(lfmt, ln+1)
	tv.LineNoRender.SetString(lnstr, fst, &sty.UnContext, &sty.Text, true, 0, 0)
	pos := mat32.Vec2{}
	lst := tv.CharStartPos(lex.Pos{Ln: ln}).Y // note: charstart pos includes descent
	pos.Y = lst + mat32.FromFixed(sty.Font.Face.Face.Metrics().Ascent) - mat32.FromFixed(sty.Font.Face.Face.Metrics().Descent)
	pos.X = float32(tv.ScBBox.Min.X) + spc.Pos().X

	tv.LineNoRender.Render(rs, pos)
	// todo: need an SvgRender interface that just takes an svg file or object
	// and renders it to a given bitmap, and then just keep that around.
	// if icnm, ok := tv.Buf.LineIcons[ln]; ok {
	// 	ic := tv.Buf.Icons[icnm]
	// 	ic.Par = tv
	// 	ic.Scene = tv.Scene
	// 	// pos.X += 20 // todo
	// 	sic := ic.SVGIcon()
	// 	sic.Resize(image.Point{20, 20})
	// 	sic.FullRenderTree()
	// 	ist := sbox.ToPointFloor()
	// 	ied := ebox.ToPointFloor()
	// 	ied.X += int(spc)
	// 	ist.X = ied.X - 20
	// 	r := image.Rectangle{Min: ist, Max: ied}
	// 	sic.Sty.BackgroundColor.SetName("black")
	// 	sic.FillScene()
	// 	draw.Draw(tv.Scene.Pixels, r, sic.Pixels, image.Point{}, draw.Over)
	// }
	if vpUpload {
		tBBox := image.Rectangle{sbox.ToPointFloor(), ebox.ToPointCeil()}
		winoff := tv.WinBBox.Min.Sub(tv.ScBBox.Min)
		tWinBBox := tBBox.Add(winoff)
		vp.This().(gi.Scene).ScUploadRegion(tBBox, tWinBBox)
	}
}

// RenderScrolls renders scrollbars if needed
func (tv *TextView) RenderScrolls() {
	if tv.HasFlag(int(TextViewRenderScrolls)) {
		ly := tv.ParentLayout()
		if ly != nil {
			ly.ReRenderScrolls()
		}
		tv.ClearFlag(int(TextViewRenderScrolls))
	}
}

// RenderLines displays a specific range of lines on the screen, also painting
// selection.  end is *inclusive* line.  returns false if nothing visible.
func (tv *TextView) RenderLines(st, ed int) bool {
	if tv == nil || tv.This() == nil || tv.Buf == nil {
		return false
	}
	if !tv.This().(gi.Widget).IsVisible() {
		return false
	}
	if st >= tv.NLines {
		st = tv.NLines - 1
	}
	if ed >= tv.NLines {
		ed = tv.NLines - 1
	}
	if st > ed {
		return false
	}
	vp := tv.Scene
	wupdt := tv.TopUpdateStart()
	sty := &tv.Style
	rs := &vp.Render
	pc := &rs.Paint
	pos := tv.RenderStartPos()
	var boxMin, boxMax mat32.Vec2
	rs.PushBounds(tv.ScBBox)
	// first get the box to fill
	visSt := -1
	visEd := -1
	for ln := st; ln <= ed; ln++ {
		lst := tv.CharStartPos(lex.Pos{Ln: ln}).Y // note: charstart pos includes descent
		led := lst + mat32.Max(tv.Renders[ln].Size.Y, tv.LineHeight)
		if int(mat32.Ceil(led)) < tv.ScBBox.Min.Y {
			continue
		}
		if int(mat32.Floor(lst)) > tv.ScBBox.Max.Y {
			continue
		}
		lp := pos
		if visSt < 0 {
			visSt = ln
			lp.Y = lst
			boxMin = lp
		}
		visEd = ln // just keep updating
		lp.Y = led
		boxMax = lp
	}
	if !(visSt < 0 && visEd < 0) {
		rs.Lock()
		boxMin.X = float32(tv.ScBBox.Min.X) // go all the way
		boxMax.X = float32(tv.ScBBox.Max.X) // go all the way
		pc.FillBox(rs, boxMin, boxMax.Sub(boxMin), &sty.BackgroundColor)
		// fmt.Printf("lns: st: %v ed: %v vis st: %v ed %v box: min %v max: %v\n", st, ed, visSt, visEd, boxMin, boxMax)

		tv.RenderDepthBg(visSt, visEd)
		tv.RenderHighlights(visSt, visEd)
		tv.RenderScopelights(visSt, visEd)
		tv.RenderSelect()
		tv.RenderLineNosBox(visSt, visEd)

		if tv.HasLineNos() {
			for ln := visSt; ln <= visEd; ln++ {
				tv.RenderLineNo(ln, true, false)
			}
			tbb := tv.ScBBox
			tbb.Min.X += int(tv.LineNoOff)
			rs.Unlock()
			rs.PushBounds(tbb)
			rs.Lock()
		}
		for ln := visSt; ln <= visEd; ln++ {
			lst := pos.Y + tv.Offs[ln]
			lp := pos
			lp.Y = lst
			lp.X += tv.LineNoOff
			tv.Renders[ln].Render(rs, lp) // not top pos -- already has baseline offset
		}
		rs.Unlock()
		if tv.HasLineNos() {
			rs.PopBounds()
		}

		tBBox := image.Rectangle{boxMin.ToPointFloor(), boxMax.ToPointCeil()}
		winoff := tv.WinBBox.Min.Sub(tv.ScBBox.Min)
		tWinBBox := tBBox.Add(winoff)
		// fmt.Printf("Render lines upload: tbbox: %v  twinbbox: %v\n", tBBox, tWinBBox)
		vp.This().(gi.Scene).ScUploadRegion(tBBox, tWinBBox)
	}
	tv.PopBounds()
	tv.RenderScrolls()
	tv.TopUpdateEnd(wupdt)
	return true
}

///////////////////////////////////////////////////////////////////////////////
//    View-specific helpers

// FirstVisibleLine finds the first visible line, starting at given line
// (typically cursor -- if zero, a visible line is first found) -- returns
// stln if nothing found above it.
func (tv *TextView) FirstVisibleLine(stln int) int {
	if stln == 0 {
		perln := float32(tv.LinesSize.Y) / float32(tv.NLines)
		stln = int(float32(tv.ScBBox.Min.Y-tv.ObjBBox.Min.Y)/perln) - 1
		if stln < 0 {
			stln = 0
		}
		for ln := stln; ln < tv.NLines; ln++ {
			cpos := tv.CharStartPos(lex.Pos{Ln: ln})
			if int(mat32.Floor(cpos.Y)) >= tv.ScBBox.Min.Y { // top definitely on screen
				stln = ln
				break
			}
		}
	}
	lastln := stln
	for ln := stln - 1; ln >= 0; ln-- {
		cpos := tv.CharStartPos(lex.Pos{Ln: ln})
		if int(mat32.Ceil(cpos.Y)) < tv.ScBBox.Min.Y { // top just offscreen
			break
		}
		lastln = ln
	}
	return lastln
}

// LastVisibleLine finds the last visible line, starting at given line
// (typically cursor) -- returns stln if nothing found beyond it.
func (tv *TextView) LastVisibleLine(stln int) int {
	lastln := stln
	for ln := stln + 1; ln < tv.NLines; ln++ {
		pos := lex.Pos{Ln: ln}
		cpos := tv.CharStartPos(pos)
		if int(mat32.Floor(cpos.Y)) > tv.ScBBox.Max.Y { // just offscreen
			break
		}
		lastln = ln
	}
	return lastln
}

// PixelToCursor finds the cursor position that corresponds to the given pixel
// location (e.g., from mouse click) which has had WinBBox.Min subtracted from
// it (i.e, relative to upper left of text area)
func (tv *TextView) PixelToCursor(pt image.Point) lex.Pos {
	if tv.NLines == 0 {
		return lex.PosZero
	}
	sty := &tv.Style
	yoff := float32(tv.WinBBox.Min.Y)
	stln := tv.FirstVisibleLine(0)
	cln := stln
	fls := tv.CharStartPos(lex.Pos{Ln: stln}).Y - yoff
	if pt.Y < int(mat32.Floor(fls)) {
		cln = stln
	} else if pt.Y > tv.WinBBox.Max.Y {
		cln = tv.NLines - 1
	} else {
		got := false
		for ln := stln; ln < tv.NLines; ln++ {
			ls := tv.CharStartPos(lex.Pos{Ln: ln}).Y - yoff
			es := ls
			es += mat32.Max(tv.Renders[ln].Size.Y, tv.LineHeight)
			if pt.Y >= int(mat32.Floor(ls)) && pt.Y < int(mat32.Ceil(es)) {
				got = true
				cln = ln
				break
			}
		}
		if !got {
			cln = tv.NLines - 1
		}
	}
	// fmt.Printf("cln: %v  pt: %v\n", cln, pt)
	lnsz := tv.Buf.LineLen(cln)
	if lnsz == 0 {
		return lex.Pos{Ln: cln, Ch: 0}
	}
	xoff := float32(tv.WinBBox.Min.X)
	scrl := tv.WinBBox.Min.X - tv.ObjBBox.Min.X
	nolno := pt.X - int(tv.LineNoOff)
	sc := int(float32(nolno+scrl) / sty.Font.Face.Metrics.Ch)
	sc -= sc / 4
	sc = max(0, sc)
	cch := sc

	si := 0
	spoff := 0
	nspan := len(tv.Renders[cln].Spans)
	lstY := tv.CharStartPos(lex.Pos{Ln: cln}).Y - yoff
	if nspan > 1 {
		si = int((float32(pt.Y) - lstY) / tv.LineHeight)
		si = min(si, nspan-1)
		si = max(si, 0)
		for i := 0; i < si; i++ {
			spoff += len(tv.Renders[cln].Spans[i].Text)
		}
		// fmt.Printf("si: %v  spoff: %v\n", si, spoff)
	}

	ri := sc
	rsz := len(tv.Renders[cln].Spans[si].Text)
	if rsz == 0 {
		return lex.Pos{Ln: cln, Ch: spoff}
	}
	// fmt.Printf("sc: %v  rsz: %v\n", sc, rsz)

	c, _ := tv.Renders[cln].SpanPosToRuneIdx(si, rsz-1) // end
	rsp := mat32.Floor(tv.CharStartPos(lex.Pos{Ln: cln, Ch: c}).X - xoff)
	rep := mat32.Ceil(tv.CharEndPos(lex.Pos{Ln: cln, Ch: c}).X - xoff)
	if int(rep) < pt.X { // end of line
		if si == nspan-1 {
			c++
		}
		return lex.Pos{Ln: cln, Ch: c}
	}

	tooBig := false
	got := false
	if ri < rsz {
		for rii := ri; rii < rsz; rii++ {
			c, _ := tv.Renders[cln].SpanPosToRuneIdx(si, rii)
			rsp = mat32.Floor(tv.CharStartPos(lex.Pos{Ln: cln, Ch: c}).X - xoff)
			rep = mat32.Ceil(tv.CharEndPos(lex.Pos{Ln: cln, Ch: c}).X - xoff)
			// fmt.Printf("trying c: %v for pt: %v xoff: %v rsp: %v, rep: %v\n", c, pt, xoff, rsp, rep)
			if pt.X >= int(rsp) && pt.X < int(rep) {
				cch = c
				got = true
				// fmt.Printf("got cch: %v for pt: %v rsp: %v, rep: %v\n", cch, pt, rsp, rep)
				break
			} else if int(rep) > pt.X {
				cch = c
				tooBig = true
				break
			}
		}
	} else {
		tooBig = true
	}
	if !got && tooBig {
		ri = rsz - 1
		// fmt.Printf("too big: %v\n", ri)
		for rii := ri; rii >= 0; rii-- {
			c, _ := tv.Renders[cln].SpanPosToRuneIdx(si, rii)
			rsp := mat32.Floor(tv.CharStartPos(lex.Pos{Ln: cln, Ch: c}).X - xoff)
			rep := mat32.Ceil(tv.CharEndPos(lex.Pos{Ln: cln, Ch: c}).X - xoff)
			// fmt.Printf("too big: trying c: %v for pt: %v rsp: %v, rep: %v\n", c, pt, rsp, rep)
			if pt.X >= int(rsp) && pt.X < int(rep) {
				got = true
				cch = c
				// fmt.Printf("got cch: %v for pt: %v rsp: %v, rep: %v\n", cch, pt, rsp, rep)
				break
			}
		}
	}
	return lex.Pos{Ln: cln, Ch: cch}
}

// SetCursorFromMouse sets cursor position from mouse mouse action -- handles
// the selection updating etc.
func (tv *TextView) SetCursorFromMouse(pt image.Point, newPos lex.Pos, selMode events.SelectModes) {
	oldPos := tv.CursorPos
	if newPos == oldPos {
		return
	}
	//	fmt.Printf("set cursor fm mouse: %v\n", newPos)
	wupdt := tv.TopUpdateStart()
	defer tv.TopUpdateEnd(wupdt)

	if !tv.SelectMode && selMode == events.ExtendContinuous {
		if tv.SelectReg == textbuf.RegionNil {
			tv.SelectStart = tv.CursorPos
		}
		tv.SetCursor(newPos)
		tv.SelectRegUpdate(tv.CursorPos)
		tv.RenderSelectLines()
		tv.RenderCursor(true)
		return
	}

	tv.SetCursor(newPos)
	if tv.SelectMode || selMode != events.SelectOne {
		if !tv.SelectMode && selMode != events.SelectOne {
			tv.SelectMode = true
			tv.SelectStart = newPos
			tv.SelectRegUpdate(tv.CursorPos)
		}
		if !tv.IsDragging() && selMode == events.SelectOne {
			ln := tv.CursorPos.Ln
			ch := tv.CursorPos.Ch
			if ln != tv.SelectReg.Start.Ln || ch < tv.SelectReg.Start.Ch || ch > tv.SelectReg.End.Ch {
				tv.SelectReset()
			}
		} else {
			tv.SelectRegUpdate(tv.CursorPos)
		}
		if tv.IsDragging() {
			tv.AutoScroll(pt.Add(tv.WinBBox.Min))
		} else {
			tv.ScrollCursorToCenterIfHidden()
		}
		tv.RenderSelectLines()
	} else if tv.HasSelection() {
		ln := tv.CursorPos.Ln
		ch := tv.CursorPos.Ch
		if ln != tv.SelectReg.Start.Ln || ch < tv.SelectReg.Start.Ch || ch > tv.SelectReg.End.Ch {
			tv.SelectReset()
		}
	}
	tv.RenderCursor(true)
}

///////////////////////////////////////////////////////////////////////////////
//    KeyInput handling

// ShiftSelect sets the selection start if the shift key is down but wasn't on the last key move.
// If the shift key has been released the select region is set to textbuf.RegionNil
func (tv *TextView) ShiftSelect(kt events.Event) {
	hasShift := kt.HasAnyModifier(goosi.Shift)
	if hasShift {
		if tv.SelectReg == textbuf.RegionNil {
			tv.SelectStart = tv.CursorPos
		}
	} else {
		tv.SelectReg = textbuf.RegionNil
	}
}

// ShiftSelectExtend updates the select region if the shift key is down and renders the selected text.
// If the shift key is not down the previously selected text is rerendered to clear the highlight
func (tv *TextView) ShiftSelectExtend(kt events.Event) {
	hasShift := kt.HasAnyModifier(goosi.Shift)
	if hasShift {
		tv.SelectRegUpdate(tv.CursorPos)
	}
	tv.RenderSelectLines()
}

// KeyInput handles keyboard input into the text field and from the completion menu
func (tv *TextView) KeyInput(kt events.Event) {
	if gi.KeyEventTrace {
		fmt.Printf("TextView KeyInput: %v\n", tv.Path())
	}
	kf := gi.KeyFun(kt.KeyChord())
	win := tv.ParentRenderWin()
	tv.ClearScopelights()

	tv.RefreshIfNeeded()

	cpop := win.CurPopup()
	if gi.PopupIsCompleter(cpop) {
		setprocessed := tv.Buf.Complete.KeyInput(kf)
		if setprocessed {
			kt.SetHandled()
		}
	}

	if gi.PopupIsCorrector(cpop) {
		setprocessed := tv.Buf.Spell.KeyInput(kf)
		if setprocessed {
			kt.SetHandled()
		}
	}

	if kt.IsHandled() {
		return
	}
	if tv.Buf == nil || tv.Buf.NumLines() == 0 {
		return
	}

	// cancelAll cancels search, completer, and..
	cancelAll := func() {
		tv.CancelComplete()
		tv.CancelCorrect()
		tv.ISearchCancel()
		tv.QReplaceCancel()
		tv.lastAutoInsert = 0
	}

	if kf != gi.KeyFunRecenter { // always start at centering
		tv.lastRecenter = 0
	}

	if kf != gi.KeyFunUndo && tv.HasFlag(int(TextViewLastWasUndo)) {
		tv.Buf.EmacsUndoSave()
		tv.ClearFlag(int(TextViewLastWasUndo))
	}

	gotTabAI := false // got auto-indent tab this time

	// first all the keys that work for both inactive and active
	switch kf {
	case gi.KeyFunMoveRight:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorForward(1)
		tv.ShiftSelectExtend(kt)
		tv.ISpellKeyInput(kt)
	case gi.KeyFunWordRight:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorForwardWord(1)
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunMoveLeft:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorBackward(1)
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunWordLeft:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorBackwardWord(1)
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunMoveUp:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorUp(1)
		tv.ShiftSelectExtend(kt)
		tv.ISpellKeyInput(kt)
	case gi.KeyFunMoveDown:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorDown(1)
		tv.ShiftSelectExtend(kt)
		tv.ISpellKeyInput(kt)
	case gi.KeyFunPageUp:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorPageUp(1)
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunPageDown:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorPageDown(1)
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunHome:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorStartLine()
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunEnd:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorEndLine()
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunDocHome:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorStartDoc()
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunDocEnd:
		cancelAll()
		kt.SetHandled()
		tv.ShiftSelect(kt)
		tv.CursorEndDoc()
		tv.ShiftSelectExtend(kt)
	case gi.KeyFunRecenter:
		cancelAll()
		kt.SetHandled()
		tv.ReMarkup()
		tv.CursorRecenter()
	case gi.KeyFunSelectMode:
		cancelAll()
		kt.SetHandled()
		tv.SelectModeToggle()
	case gi.KeyFunCancelSelect:
		tv.CancelComplete()
		kt.SetHandled()
		tv.EscPressed() // generic cancel
	case gi.KeyFunSelectAll:
		cancelAll()
		kt.SetHandled()
		tv.SelectAll()
	case gi.KeyFunCopy:
		cancelAll()
		kt.SetHandled()
		tv.Copy(true) // reset
	case gi.KeyFunSearch:
		kt.SetHandled()
		tv.QReplaceCancel()
		tv.CancelComplete()
		tv.ISearchStart()
	case gi.KeyFunAbort:
		cancelAll()
		kt.SetHandled()
		tv.EscPressed()
	case gi.KeyFunJump:
		cancelAll()
		kt.SetHandled()
		tv.JumpToLinePrompt()
	case gi.KeyFunHistPrev:
		cancelAll()
		kt.SetHandled()
		tv.CursorToHistPrev()
	case gi.KeyFunHistNext:
		cancelAll()
		kt.SetHandled()
		tv.CursorToHistNext()
	case gi.KeyFunLookup:
		cancelAll()
		kt.SetHandled()
		tv.Lookup()
	}
	if tv.IsDisabled() {
		switch {
		case kf == gi.KeyFunFocusNext: // tab
			kt.SetHandled()
			tv.CursorNextLink(true)
		case kf == gi.KeyFunFocusPrev: // tab
			kt.SetHandled()
			tv.CursorPrevLink(true)
		case kf == gi.KeyFunNil && tv.ISearch.On:
			if unicode.IsPrint(kt.Rune) && !kt.HasAnyModifier(key.Control, key.Meta) {
				tv.ISearchKeyInput(kt)
			}
		case kt.Rune == ' ' || kf == gi.KeyFunAccept || kf == gi.KeyFunEnter:
			kt.SetHandled()
			tv.CursorPos.Ch--
			tv.CursorNextLink(true) // todo: cursorcurlink
			tv.OpenLinkAt(tv.CursorPos)
		}
		return
	}
	if kt.IsHandled() {
		tv.SetFlagState(gotTabAI, int(TextViewLastWasTabAI))
		return
	}
	switch kf {
	case gi.KeyFunReplace:
		kt.SetHandled()
		tv.CancelComplete()
		tv.ISearchCancel()
		tv.QReplacePrompt()
	// case gi.KeyFunAccept: // ctrl+enter
	// 	tv.ISearchCancel()
	// 	tv.QReplaceCancel()
	// 	kt.SetHandled()
	// 	tv.FocusNext()
	case gi.KeyFunBackspace:
		// todo: previous item in qreplace
		if tv.ISearch.On {
			tv.ISearchBackspace()
		} else {
			kt.SetHandled()
			tv.CursorBackspace(1)
			tv.ISpellKeyInput(kt)
			tv.OfferComplete()
		}
	case gi.KeyFunKill:
		cancelAll()
		kt.SetHandled()
		tv.CursorKill()
	case gi.KeyFunDelete:
		cancelAll()
		kt.SetHandled()
		tv.CursorDelete(1)
		tv.ISpellKeyInput(kt)
	case gi.KeyFunBackspaceWord:
		cancelAll()
		kt.SetHandled()
		tv.CursorBackspaceWord(1)
	case gi.KeyFunDeleteWord:
		cancelAll()
		kt.SetHandled()
		tv.CursorDeleteWord(1)
	case gi.KeyFunCut:
		cancelAll()
		kt.SetHandled()
		tv.Cut()
	case gi.KeyFunPaste:
		cancelAll()
		kt.SetHandled()
		tv.Paste()
	case gi.KeyFunTranspose:
		cancelAll()
		kt.SetHandled()
		tv.CursorTranspose()
	case gi.KeyFunTransposeWord:
		cancelAll()
		kt.SetHandled()
		tv.CursorTransposeWord()
	case gi.KeyFunPasteHist:
		cancelAll()
		kt.SetHandled()
		tv.PasteHist()
	case gi.KeyFunUndo:
		cancelAll()
		kt.SetHandled()
		tv.Undo()
		tv.SetFlag(int(TextViewLastWasUndo))
	case gi.KeyFunRedo:
		cancelAll()
		kt.SetHandled()
		tv.Redo()
	case gi.KeyFunComplete:
		tv.ISearchCancel()
		kt.SetHandled()
		if tv.Buf.IsSpellEnabled(tv.CursorPos) {
			tv.OfferCorrect()
		} else {
			tv.ForceComplete = true
			tv.OfferComplete()
			tv.ForceComplete = false
		}
	case gi.KeyFunEnter:
		cancelAll()
		if !kt.HasAnyModifier(key.Control, key.Meta) {
			kt.SetHandled()
			if tv.Buf.Opts.AutoIndent {
				bufUpdt, winUpdt, autoSave := tv.Buf.BatchUpdateStart()
				lp, _ := pi.LangSupport.Props(tv.Buf.PiState.Sup)
				if lp != nil && lp.Lang != nil && lp.HasFlag(pi.ReAutoIndent) {
					// only re-indent current line for supported types
					tbe, _, _ := tv.Buf.AutoIndent(tv.CursorPos.Ln) // reindent current line
					if tbe != nil {
						// go back to end of line!
						npos := lex.Pos{Ln: tv.CursorPos.Ln, Ch: tv.Buf.LineLen(tv.CursorPos.Ln)}
						tv.SetCursor(npos)
					}
				}
				tv.InsertAtCursor([]byte("\n"))
				tbe, _, cpos := tv.Buf.AutoIndent(tv.CursorPos.Ln)
				if tbe != nil {
					tv.SetCursorShow(lex.Pos{Ln: tbe.Reg.End.Ln, Ch: cpos})
				}
				tv.Buf.BatchUpdateEnd(bufUpdt, winUpdt, autoSave)
			} else {
				tv.InsertAtCursor([]byte("\n"))
			}
			tv.ISpellKeyInput(kt)
		}
		// todo: KeFunFocusPrev -- unindent
	case gi.KeyFunFocusNext: // tab
		cancelAll()
		if !kt.HasAnyModifier(key.Control, key.Meta) {
			kt.SetHandled()
			wupdt := tv.TopUpdateStart()
			lasttab := tv.HasFlag(int(TextViewLastWasTabAI))
			if !lasttab && tv.CursorPos.Ch == 0 && tv.Buf.Opts.AutoIndent {
				_, _, cpos := tv.Buf.AutoIndent(tv.CursorPos.Ln)
				tv.CursorPos.Ch = cpos
				tv.RenderCursor(true)
				gotTabAI = true
			} else {
				tv.InsertAtCursor(indent.Bytes(tv.Buf.Opts.IndentChar(), 1, tv.Style.Text.TabSize))
			}
			tv.TopUpdateEnd(wupdt)
			tv.ISpellKeyInput(kt)
		}
	case gi.KeyFunFocusPrev: // shift-tab
		cancelAll()
		if !kt.HasAnyModifier(key.Control, key.Meta) {
			kt.SetHandled()
			if tv.CursorPos.Ch > 0 {
				ind, _ := lex.LineIndent(tv.Buf.Line(tv.CursorPos.Ln), tv.Style.Text.TabSize)
				if ind > 0 {
					tv.Buf.IndentLine(tv.CursorPos.Ln, ind-1)
					intxt := indent.Bytes(tv.Buf.Opts.IndentChar(), ind-1, tv.Style.Text.TabSize)
					npos := lex.Pos{Ln: tv.CursorPos.Ln, Ch: len(intxt)}
					tv.SetCursorShow(npos)
				}
			}
			tv.ISpellKeyInput(kt)
		}
	case gi.KeyFunNil:
		if unicode.IsPrint(kt.Rune) {
			if !kt.HasAnyModifier(key.Control, key.Meta) {
				tv.KeyInputInsertRune(kt)
			}
		}
		if unicode.IsSpace(kt.Rune) {
			tv.ForceComplete = false
		}
		tv.ISpellKeyInput(kt)
	}
	tv.SetFlagState(gotTabAI, int(TextViewLastWasTabAI))
}

// KeyInputInsertBra handle input of opening bracket-like entity (paren, brace, bracket)
func (tv *TextView) KeyInputInsertBra(kt events.Event) {
	bufUpdt, winUpdt, autoSave := tv.Buf.BatchUpdateStart()
	defer tv.Buf.BatchUpdateEnd(bufUpdt, winUpdt, autoSave)
	pos := tv.CursorPos
	match := true
	newLine := false
	curLn := tv.Buf.Line(pos.Ln)
	lnLen := len(curLn)
	lp, _ := pi.LangSupport.Props(tv.Buf.PiState.Sup)
	if lp != nil && lp.Lang != nil {
		match, newLine = lp.Lang.AutoBracket(&tv.Buf.PiState, kt.Rune, pos, curLn)
	} else {
		if kt.Rune == '{' {
			if pos.Ch == lnLen {
				if lnLen == 0 || unicode.IsSpace(curLn[pos.Ch-1]) {
					newLine = true
				}
				match = true
			} else {
				match = unicode.IsSpace(curLn[pos.Ch])
			}
		} else {
			match = pos.Ch == lnLen || unicode.IsSpace(curLn[pos.Ch]) // at end or if space after
		}
	}
	if match {
		ket, _ := lex.BracePair(kt.Rune)
		if newLine && tv.Buf.Opts.AutoIndent {
			tv.InsertAtCursor([]byte(string(kt.Rune) + "\n"))
			tbe, _, cpos := tv.Buf.AutoIndent(tv.CursorPos.Ln)
			if tbe != nil {
				pos = lex.Pos{Ln: tbe.Reg.End.Ln, Ch: cpos}
				tv.SetCursorShow(pos)
			}
			tv.InsertAtCursor([]byte("\n" + string(ket)))
			tv.Buf.AutoIndent(tv.CursorPos.Ln)
		} else {
			tv.InsertAtCursor([]byte(string(kt.Rune) + string(ket)))
			pos.Ch++
		}
		tv.lastAutoInsert = ket
	} else {
		tv.InsertAtCursor([]byte(string(kt.Rune)))
		pos.Ch++
	}
	tv.SetCursorShow(pos)
	tv.SetCursorCol(tv.CursorPos)
}

// KeyInputInsertRune handles the insertion of a typed character
func (tv *TextView) KeyInputInsertRune(kt events.Event) {
	kt.SetHandled()
	if tv.ISearch.On {
		tv.CancelComplete()
		tv.ISearchKeyInput(kt)
	} else if tv.QReplace.On {
		tv.CancelComplete()
		tv.QReplaceKeyInput(kt)
	} else {
		if kt.Rune == '{' || kt.Rune == '(' || kt.Rune == '[' {
			tv.KeyInputInsertBra(kt)
		} else if kt.Rune == '}' && tv.Buf.Opts.AutoIndent && tv.CursorPos.Ch == tv.Buf.LineLen(tv.CursorPos.Ln) {
			tv.CancelComplete()
			tv.lastAutoInsert = 0
			bufUpdt, winUpdt, autoSave := tv.Buf.BatchUpdateStart()
			tv.InsertAtCursor([]byte(string(kt.Rune)))
			tbe, _, cpos := tv.Buf.AutoIndent(tv.CursorPos.Ln)
			if tbe != nil {
				tv.SetCursorShow(lex.Pos{Ln: tbe.Reg.End.Ln, Ch: cpos})
			}
			tv.Buf.BatchUpdateEnd(bufUpdt, winUpdt, autoSave)
		} else if tv.lastAutoInsert == kt.Rune { // if we type what we just inserted, just move past
			tv.CursorPos.Ch++
			tv.SetCursorShow(tv.CursorPos)
			tv.lastAutoInsert = 0
		} else {
			tv.lastAutoInsert = 0
			tv.InsertAtCursor([]byte(string(kt.Rune)))
			if kt.Rune == ' ' {
				tv.CancelComplete()
			} else {
				tv.OfferComplete()
			}
		}
		if kt.Rune == '}' || kt.Rune == ')' || kt.Rune == ']' {
			cp := tv.CursorPos
			np := cp
			np.Ch--
			tp, found := tv.Buf.BraceMatch(kt.Rune, np)
			if found {
				tv.Scopelights = append(tv.Scopelights, textbuf.NewRegionPos(tp, lex.Pos{tp.Ln, tp.Ch + 1}))
				tv.Scopelights = append(tv.Scopelights, textbuf.NewRegionPos(np, lex.Pos{cp.Ln, cp.Ch}))
				if tv.CursorPos.Ln < tp.Ln {
					tv.RenderLines(cp.Ln, tp.Ln)
				} else {
					tv.RenderLines(tp.Ln, cp.Ln)
				}
			}
		}
	}
}

// OpenLink opens given link, either by sending LinkSig signal if there are
// receivers, or by calling the TextLinkHandler if non-nil, or URLHandler if
// non-nil (which by default opens user's default browser via
// oswin/App.OpenURL())
func (tv *TextView) OpenLink(tl *paint.TextLink) {
	tl.Widget = tv.This().(gi.Widget)
	// fmt.Printf("opening link: %v\n", tl.URL)
	if len(tv.LinkSig.Cons) == 0 {
		if paint.TextLinkHandler != nil {
			if paint.TextLinkHandler(*tl) {
				return
			}
			if paint.URLHandler != nil {
				paint.URLHandler(tl.URL)
			}
		}
		return
	}
	tv.LinkSig.Emit(tv.This(), 0, tl.URL) // todo: could potentially signal different target=_blank kinds of options here with the sig
}

// LinkAt returns link at given cursor position, if one exists there --
// returns true and the link if there is a link, and false otherwise
func (tv *TextView) LinkAt(pos lex.Pos) (*paint.TextLink, bool) {
	if !(pos.Ln < len(tv.Renders) && len(tv.Renders[pos.Ln].Links) > 0) {
		return nil, false
	}
	cpos := tv.CharStartPos(pos).ToPointCeil()
	cpos.Y += 2
	cpos.X += 2
	lpos := tv.CharStartPos(lex.Pos{Ln: pos.Ln})
	rend := &tv.Renders[pos.Ln]
	for ti := range rend.Links {
		tl := &rend.Links[ti]
		tlb := tl.Bounds(rend, lpos)
		if cpos.In(tlb) {
			return tl, true
		}
	}
	return nil, false
}

// OpenLinkAt opens a link at given cursor position, if one exists there --
// returns true and the link if there is a link, and false otherwise -- highlights selected link
func (tv *TextView) OpenLinkAt(pos lex.Pos) (*paint.TextLink, bool) {
	tl, ok := tv.LinkAt(pos)
	if ok {
		rend := &tv.Renders[pos.Ln]
		st, _ := rend.SpanPosToRuneIdx(tl.StartSpan, tl.StartIdx)
		ed, _ := rend.SpanPosToRuneIdx(tl.EndSpan, tl.EndIdx)
		reg := textbuf.NewRegion(pos.Ln, st, pos.Ln, ed)
		tv.HighlightRegion(reg)
		tv.SetCursorShow(pos)
		tv.SavePosHistory(tv.CursorPos)
		tv.OpenLink(tl)
	}
	return tl, ok
}

// MouseEvent handles the events.Event
func (tv *TextView) MouseEvent(me events.Event) {
	if !tv.StateIs(states.Focused) {
		tv.GrabFocus()
	}
	tv.SetFlag(int(TextViewFocusActive))
	me.SetHandled()
	if tv.Buf == nil || tv.Buf.NumLines() == 0 {
		return
	}
	pt := tv.PointToRelPos(me.Pos())
	newPos := tv.PixelToCursor(pt)
	switch me.Button {
	case events.Left:
		if me.Action == events.Press {
			me.SetHandled()
			if _, got := tv.OpenLinkAt(newPos); got {
			} else {
				tv.SetCursorFromMouse(pt, newPos, me.SelectMode())
				tv.SavePosHistory(tv.CursorPos)
			}
		} else if me.Action == events.DoubleClick {
			me.SetHandled()
			if tv.HasSelection() {
				if tv.SelectReg.Start.Ln == tv.SelectReg.End.Ln {
					sz := tv.Buf.LineLen(tv.SelectReg.Start.Ln)
					if tv.SelectReg.Start.Ch == 0 && tv.SelectReg.End.Ch == sz {
						tv.SelectReset()
					} else { // assume word, go line
						tv.SelectReg.Start.Ch = 0
						tv.SelectReg.End.Ch = sz
					}
				} else {
					tv.SelectReset()
				}
			} else {
				if tv.SelectWord() {
					tv.CursorPos = tv.SelectReg.Start
				}
			}
			tv.RenderLines(tv.CursorPos.Ln, tv.CursorPos.Ln)
		}
	case events.Middle:
		if !tv.IsDisabled() && me.Action == events.Press {
			me.SetHandled()
			tv.SetCursorFromMouse(pt, newPos, me.SelectMode())
			tv.SavePosHistory(tv.CursorPos)
			tv.Paste()
		}
	case events.Right:
		if me.Action == events.Press {
			me.SetHandled()
			tv.SetCursorFromMouse(pt, newPos, me.SelectMode())
			tv.EmitContextMenuSignal()
			tv.This().(gi.Widget).ContextMenu()
		}
	}
}

// todo: needs this in event filtering update!
// if !tv.HasLinks {
// 	return
// }

// MouseMoveEvent
func (tv *TextView) MouseMoveEvent() {
	we.AddFunc(events.MouseMove, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		me := d.(events.Event)
		me.SetHandled()
		tvv := recv.Embed(TypeTextView).(*TextView)
		pt := tv.PointToRelPos(me.Pos())
		mpos := tvv.PixelToCursor(pt)
		if mpos.Ln >= tvv.NLines {
			return
		}
		pos := tv.RenderStartPos()
		pos.Y += tv.Offs[mpos.Ln]
		pos.X += tv.LineNoOff
		rend := &tvv.Renders[mpos.Ln]
		inLink := false
		for _, tl := range rend.Links {
			tlb := tl.Bounds(rend, pos)
			if me.Pos().In(tlb) {
				inLink = true
				break
			}
		}
		// TODO: figure out how to handle links with new cursor setup
		if inLink {
			goosi.TheApp.Cursor(tv.ParentRenderWin().RenderWin).PushIfNot(cursors.Pointer)
		} else {
			goosi.TheApp.Cursor(tv.ParentRenderWin().RenderWin).PopIf(cursors.Pointer)
		}

	})
}

func (tv *TextView) MouseDragEvent() {
	we.AddFunc(events.MouseDrag, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		me := d.(events.Event)
		me.SetHandled()
		tv := recv.Embed(TypeTextView).(*TextView)
		if !tv.SelectMode {
			tv.SelectModeToggle()
		}
		pt := tv.PointToRelPos(me.Pos())
		newPos := tv.PixelToCursor(pt)
		tv.SetCursorFromMouse(pt, newPos, events.SelectOne)
	})
}

func (tv *TextView) MouseFocusEvent() {
	we.AddFunc(events.MouseEnter, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		tv := recv.Embed(TypeTextView).(*TextView)
		if tv.IsDisabled() {
			return
		}
		me := d.(events.Event)
		me.SetHandled()
		// TODO: is this needed?
		tv.RefreshIfNeeded()
	})
}

// TextViewEvents sets connections between mouse and key events and actions
func (tv *TextView) TextViewEvents() {
	tv.HoverTooltipEvent(we)
	tv.MouseMoveEvent(we)
	tv.MouseDragEvent(we)
	we.AddFunc(events.MouseUp, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		tv := recv.Embed(TypeTextView).(*TextView)
		me := d.(events.Event)
		tv.MouseEvent(me)
	})
	tv.MouseFocusEvent(we)
	we.AddFunc(events.KeyChord, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
		tv := recv.Embed(TypeTextView).(*TextView)
		kt := d.(*events.Key)
		tv.KeyInput(kt)
	})

	// todo: need to handle this separately!!
	if dlg, ok := tv.Scene.This().(*gi.Dialog); ok {
		dlg.DialogSig.Connect(tv.This(), func(recv, send ki.Ki, sig int64, data any) {
			tv, _ := recv.Embed(TypeTextView).(*TextView)
			if sig == int64(gi.DialogAccepted) {
				tv.EditDone()
			}
		})
	}
}

////////////////////////////////////////////////////
//  Widget Interface

// Config calls Init on widget
// func (tv *TextView) ConfigWidget(vp *gi.Scene) {
//
// }

// StyleTextView sets the style of widget
func (tv *TextView) StyleTextView() {
	tv.StyMu.Lock()
	defer tv.StyMu.Unlock()

	if tv.NeedsRebuild() {
		if tv.Buf != nil {
			tv.Buf.SetHiStyle(histyle.StyleDefault)
		}
		win := tv.ParentRenderWin()
		if win != nil {
			spnm := tv.CursorSpriteName()
			win.DeleteSprite(spnm)
		}
	}
	tv.ApplyStyleWidget()
	tv.CursorWidth.ToDots(&tv.Style.UnContext)
	if tv.Buf != nil {
		tv.Buf.Opts.StyleFromProps(tv.Props)
	}
	// if tv.IsDisabled() {
	// 	if tv.StateIs(states.Selected) {
	// 		tv.Style = tv.StateStyles[TextViewSel]
	// 	} else {
	// 		tv.Style = tv.StateStyles[TextViewInactive]
	// 	}
	// } else if tv.NLines == 0 {
	// 	tv.Style = tv.StateStyles[TextViewInactive]
	// } else if tv.StateIs(states.Focused) {
	// 	tv.Style = tv.StateStyles[TextViewFocus]
	// } else if tv.StateIs(states.Selected) {
	// 	tv.Style = tv.StateStyles[TextViewSel]
	// } else {
	// 	tv.Style = tv.StateStyles[TextViewActive]
	// }
}

// ApplyStyle calls StyleTextView and sets the style
func (tv *TextView) ApplyStyle(sc *gi.Scene) {
	tv.SetFlag(int(gi.CanFocus)) // always focusable
	tv.StyleTextView()
}

// GetSize
func (tv *TextView) GetSize(vp *gi.Scene, iter int) {
	if iter > 0 {
		return
	}
	tv.InitLayout(vp * Scene)
	if tv.LinesSize == (image.Point{}) {
		tv.LayoutAllLines(true)
	} else {
		tv.SetSize()
	}
}

// DoLayoutn
func (tv *TextView) DoLayout(vp *gi.Scene, parBBox image.Rectangle, iter int) bool {
	tv.DoLayoutBase(parBBox, iter)
	for i := 0; i < int(TextViewStatesN); i++ {
		tv.StateStyles[i].CopyUnitContext(&tv.Style.UnContext)
	}
	tv.DoLayoutChildren(iter)
	if tv.LinesSize == (image.Point{} || tv.NeedsRebuild() || tv.NeedsRefresh() ||
		tv.NLines != tv.Buf.NumLines()) {
		redo := tv.LayoutAllLines(true) // is our size now different?  if so iterate..
		return redo
	}
	tv.SetSize()
	return false
}

// Render does some preliminary work and then calls render on children
func (tv *TextView) Render(vp *gi.Scene) {
	// fmt.Printf("tv render: %v\n", tv.Nm)
	// if tv.NeedsFullReRender() {
	// 	tv.SetNeedsRefresh()
	// }
	// if tv.FullReRenderIfNeeded() {
	// 	return
	// }
	//
	// if tv.Buf != nil && (tv.NLines != tv.Buf.NumLines() || tv.NeedsRefresh()) {
	// 	tv.LayoutAllLines(false)
	// 	if tv.NeedsRefresh() {
	// 		tv.ClearNeedsRefresh()
	// 	}
	// }

	tv.VisSizes()
	if tv.NLines == 0 {
		ply := tv.ParentLayout()
		if ply != nil {
			tv.ScBBox = ply.ScBBox
			tv.WinBBox = ply.WinBBox
		}
	}

	if tv.PushBounds() {
		tv.RenderAllLinesInBounds()
		if tv.ScrollToCursorOnRender {
			tv.ScrollToCursorOnRender = false
			tv.CursorPos = tv.ScrollToCursorPos
			tv.ScrollCursorToTop()
		}
		if tv.StateIs(states.Focused) && tv.IsFocusActive() {
			// fmt.Printf("tv render: %v  start cursor\n", tv.Nm)
			tv.StartCursor()
		} else {
			// fmt.Printf("tv render: %v  stop cursor\n", tv.Nm)
			tv.StopCursor()
		}
		tv.RenderChildren()
		tv.PopBounds()
	} else {
		// fmt.Printf("tv render: %v  not vis stop cursor\n", tv.Nm)
		tv.StopCursor()
	}
}

// SetTypeHandlers indirectly sets connections between mouse and key events and actions
func (tv *TextView) SetTypeHandlers() {
}

// FocusChanged appropriate actions for various types of focus changes
// func (tv *TextView) FocusChanged(change gi.FocusChanges) {
// 	switch change {
// 	case gi.FocusLost:
// 		tv.ClearFlag(int(TextViewFocusActive))
// 		// tv.EditDone()
// 		tv.StopCursor() // make sure no cursor
// 		tv.UpdateSig()
// 		goosi.TheApp.HideVirtualKeyboard()
// 		// fmt.Printf("lost focus: %v\n", tv.Nm)
// 	case gi.FocusGot:
// 		tv.SetFlag(int(TextViewFocusActive))
// 		tv.EmitFocusedSignal()
// 		tv.UpdateSig()
// 		goosi.TheApp.ShowVirtualKeyboard(goosi.DefaultKeyboard)
// 		// fmt.Printf("got focus: %v\n", tv.Nm)
// 	case gi.FocusInactive:
// 		tv.ClearFlag(int(TextViewFocusActive))
// 		tv.StopCursor()
// 		// tv.EditDone()
// 		// tv.UpdateSig()
// 		goosi.TheApp.HideVirtualKeyboard()
// 		// fmt.Printf("focus inactive: %v\n", tv.Nm)
// 	case gi.FocusActive:
// 		// fmt.Printf("focus active: %v\n", tv.Nm)
// 		tv.SetFlag(int(TextViewFocusActive))
// 		// tv.UpdateSig()
// 		// todo: see about cursor
// 		tv.StartCursor()
// 		goosi.TheApp.ShowVirtualKeyboard(goosi.DefaultKeyboard)
// 	}
// }
