// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"fmt"
	"image"

	"goki.dev/colors"
	"goki.dev/cursors"
	"goki.dev/girl/paint"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/goosi"
	"goki.dev/goosi/events"
	"goki.dev/goosi/mimedata"
	"goki.dev/mat32/v2"
)

// Label is a widget for rendering text labels -- supports full widget model
// including box rendering, and full HTML styling, including links -- LinkSig
// emits link with data of URL -- opens default browser if nobody receiving
// signal.  The default white-space option is 'pre' -- set to 'normal' or
// other options to get word-wrapping etc.
//
//goki:embedder
type Label struct {
	WidgetBase

	// label to display
	Text string `xml:"text" desc:"label to display"`

	// the type of label
	Type LabelTypes `desc:"the type of label"`

	// [view: -] signal for clicking on a link -- data is a string of the URL -- if nobody receiving this signal, calls TextLinkHandler then URLHandler
	// LinkSig ki.Signal `copy:"-" json:"-" xml:"-" view:"-" desc:"signal for clicking on a link -- data is a string of the URL -- if nobody receiving this signal, calls TextLinkHandler then URLHandler"`

	// render data for text label
	TextRender paint.Text `copy:"-" xml:"-" json:"-" desc:"render data for text label"`

	// position offset of start of text rendering, from last render -- AllocPos plus alignment factors for center, right etc.
	RenderPos mat32.Vec2 `copy:"-" xml:"-" json:"-" desc:"position offset of start of text rendering, from last render -- AllocPos plus alignment factors for center, right etc."`
}

func (lb *Label) CopyFieldsFrom(frm any) {
	fr := frm.(*Label)
	lb.WidgetBase.CopyFieldsFrom(&fr.WidgetBase)
	lb.Text = fr.Text
}

// LabelTypes is an enum containing the different
// possible types of labels
type LabelTypes int //enums:enum

const (
	// LabelDisplayLarge is a large, short, and important
	// display label with a default font size of 57dp.
	LabelDisplayLarge LabelTypes = iota
	// LabelDisplayMedium is a medium-sized, short, and important
	// display label with a default font size of 45dp.
	LabelDisplayMedium
	// LabelDisplaySmall is a small, short, and important
	// display label with a default font size of 36dp.
	LabelDisplaySmall

	// LabelHeadlineLarge is a large, high-emphasis
	// headline label with a default font size of 32dp.
	LabelHeadlineLarge
	// LabelHeadlineMedium is a medium-sized, high-emphasis
	// headline label with a default font size of 28dp.
	LabelHeadlineMedium
	// LabelHeadlineSmall is a small, high-emphasis
	// headline label with a default font size of 24dp.
	LabelHeadlineSmall

	// LabelTitleLarge is a large, medium-emphasis
	// title label with a default font size of 22dp.
	LabelTitleLarge
	// LabelTitleMedium is a medium-sized, medium-emphasis
	// title label with a default font size of 16dp.
	LabelTitleMedium
	// LabelTitleSmall is a small, medium-emphasis
	// title label with a default font size of 14dp.
	LabelTitleSmall

	// LabelBodyLarge is a large body label used for longer
	// passages of text with a default font size of 16dp.
	LabelBodyLarge
	// LabelBodyMedium is a medium-sized body label used for longer
	// passages of text with a default font size of 14dp.
	LabelBodyMedium
	// LabelBodySmall is a small body label used for longer
	// passages of text with a default font size of 12dp.
	LabelBodySmall

	// LabelLabelLarge is a large label used for label text (like a caption
	// or the text inside a button) with a default font size of 14dp.
	LabelLabelLarge
	// LabelLabelMedium is a medium-sized label used for label text (like a caption
	// or the text inside a button) with a default font size of 12dp.
	LabelLabelMedium
	// LabelLabelSmall is a small label used for label text (like a caption
	// or the text inside a button) with a default font size of 11dp.
	LabelLabelSmall
)

func (lb *Label) OnInit() {
	lb.WidgetHandlers()
	lb.LabelHandlers()
	lb.LabelStyles()
}

func (lb *Label) LabelStyles() {
	lb.Type = LabelBodyLarge
	lb.AddStyles(func(s *styles.Style) {
		s.SetAbilities(true, states.Selectable, states.DoubleClickable)
		s.Cursor = cursors.Text

		if s.Is(states.Selected) {
			s.BackgroundColor.SetSolid(colors.Scheme.Select.Container)
		}

		s.Text.WhiteSpace = styles.WhiteSpaceNormal
		s.AlignV = styles.AlignMiddle
		// Label styles based on https://m3.material.io/styles/typography/type-scale-tokens
		// TODO: maybe support brand and plain global fonts with larger labels defaulting to brand and smaller to plain
		switch lb.Type {
		case LabelLabelLarge:
			s.Text.LineHeight.SetDp(20)
			s.Font.Size.SetDp(14)
			s.Text.LetterSpacing.SetDp(0.1)
			s.Font.Weight = styles.WeightMedium
		case LabelLabelMedium:
			s.Text.LineHeight.SetDp(16)
			s.Font.Size.SetDp(12)
			s.Text.LetterSpacing.SetDp(0.5)
			s.Font.Weight = styles.WeightMedium
		case LabelLabelSmall:
			s.Text.LineHeight.SetDp(16)
			s.Font.Size.SetDp(11)
			s.Text.LetterSpacing.SetDp(0.5)
			s.Font.Weight = styles.WeightMedium
		case LabelBodyLarge:
			s.Text.LineHeight.SetDp(24)
			s.Font.Size.SetDp(16)
			s.Text.LetterSpacing.SetDp(0.5)
			s.Font.Weight = styles.WeightNormal
		case LabelBodyMedium:
			s.Text.LineHeight.SetDp(20)
			s.Font.Size.SetDp(14)
			s.Text.LetterSpacing.SetDp(0.25)
			s.Font.Weight = styles.WeightNormal
		case LabelBodySmall:
			s.Text.LineHeight.SetDp(16)
			s.Font.Size.SetDp(12)
			s.Text.LetterSpacing.SetDp(0.4)
			s.Font.Weight = styles.WeightNormal
		case LabelTitleLarge:
			s.Text.LineHeight.SetDp(28)
			s.Font.Size.SetDp(22)
			s.Text.LetterSpacing.SetDp(0)
			s.Font.Weight = styles.WeightNormal
		case LabelTitleMedium:
			s.Text.LineHeight.SetDp(24)
			s.Font.Size.SetDp(16)
			s.Text.LetterSpacing.SetDp(0.15)
			s.Font.Weight = styles.WeightMedium
		case LabelTitleSmall:
			s.Text.LineHeight.SetDp(20)
			s.Font.Size.SetDp(14)
			s.Text.LetterSpacing.SetDp(0.1)
			s.Font.Weight = styles.WeightMedium
		case LabelHeadlineLarge:
			s.Text.LineHeight.SetDp(40)
			s.Font.Size.SetDp(32)
			s.Text.LetterSpacing.SetDp(0)
			s.Font.Weight = styles.WeightNormal
		case LabelHeadlineMedium:
			s.Text.LineHeight.SetDp(36)
			s.Font.Size.SetDp(28)
			s.Text.LetterSpacing.SetDp(0)
			s.Font.Weight = styles.WeightNormal
		case LabelHeadlineSmall:
			s.Text.LineHeight.SetDp(32)
			s.Font.Size.SetDp(24)
			s.Text.LetterSpacing.SetDp(0)
			s.Font.Weight = styles.WeightNormal
		case LabelDisplayLarge:
			s.Text.LineHeight.SetDp(64)
			s.Font.Size.SetDp(57)
			s.Text.LetterSpacing.SetDp(-0.25)
			s.Font.Weight = styles.WeightNormal
		case LabelDisplayMedium:
			s.Text.LineHeight.SetDp(52)
			s.Font.Size.SetDp(45)
			s.Text.LetterSpacing.SetDp(0)
			s.Font.Weight = styles.WeightNormal
		case LabelDisplaySmall:
			s.Text.LineHeight.SetDp(44)
			s.Font.Size.SetDp(36)
			s.Text.LetterSpacing.SetDp(0)
			s.Font.Weight = styles.WeightNormal
		}
	})
}

// SetText sets the text and updates the rendered version.
// Note: if there is already a label set, and no other
// larger updates are taking place, the new label may just
// illegibly overlay on top of the old one.
// Set Redrawable = true to fix this issue (it will redraw
// the background -- sampling from actual if none is set).
func (lb *Label) SetText(txt string) *Label {
	updt := lb.UpdateStart()
	// if lb.Text != "" { // not good to automate this -- better to use docs -- bg can be bad
	// 	lb.Redrawable = true
	// }

	lb.StyMu.RLock()
	lb.Text = txt
	if lb.Text == "" {
		lb.TextRender.SetHTML(" ", lb.Style.FontRender(), &lb.Style.Text, &lb.Style.UnContext, lb.CSSAgg)
	} else {
		lb.TextRender.SetHTML(lb.Text, lb.Style.FontRender(), &lb.Style.Text, &lb.Style.UnContext, lb.CSSAgg)
	}
	spc := lb.BoxSpace()
	sz := lb.LayState.Alloc.Size
	if sz.IsNil() {
		sz = lb.LayState.SizePrefOrMax()
	}
	if !sz.IsNil() {
		sz.SetSub(spc.Size())
	}
	lb.TextRender.LayoutStdLR(&lb.Style.Text, lb.Style.FontRender(), &lb.Style.UnContext, sz)
	lb.StyMu.RUnlock()
	lb.UpdateEnd(updt)
	return lb
}

// SetType sets the formatting type of the label
func (lb *Label) SetType(typ LabelTypes) *Label {
	updt := lb.UpdateStart()
	lb.Type = typ
	lb.UpdateEnd(updt)
	return lb
}

// OpenLink opens given link, either by sending LinkSig signal if there are
// receivers, or by calling the TextLinkHandler if non-nil, or URLHandler if
// non-nil (which by default opens user's default browser via
// oswin/App.OpenURL())
func (lb *Label) OpenLink(tl *paint.TextLink) {
	// tl.Widget = lb.This() // todo: needs this
	// if len(lb.LinkSig.Cons) == 0 {
	// 	if paint.TextLinkHandler != nil {
	// 		if paint.TextLinkHandler(*tl) {
	// 			return
	// 		}
	// 	}
	// 	if paint.URLHandler != nil {
	// 		paint.URLHandler(tl.URL)
	// 	}
	// 	return
	// }
	// lb.LinkSig.Emit(lb.This(), 0, tl.URL) // todo: could potentially signal different target=_blank kinds of options here with the sig
}

// func (lb *Label) HandleEvent(ev events.Event) {
// 	// hasLinks := len(lb.TextRender.Links) > 0
// 	// if !hasLinks {
// 	// 	lb.Events.Ex(events.MouseMove)
// 	// }
// }

func (lb *Label) LabelHandlers() {
	lb.LabelLongHover()
	lb.LabelClick()
	lb.LabelMouseMove()
	lb.LabelKeys()
}

func (lb *Label) LabelLongHover() {
	lb.On(events.LongHoverStart, func(e events.Event) {
		if lb.StateIs(states.Disabled) {
			return
		}
		// hasLinks := len(lb.TextRender.Links) > 0
		// if hasLinks {
		// 	pos := llb.RenderPos
		// 	for ti := range llb.TextRender.Links {
		// 		tl := &llb.TextRender.Links[ti]
		// 		tlb := tl.Bounds(&llb.TextRender, pos)
		// 		if me.Pos().In(tlb) {
		// 			PopupTooltip(tl.URL, tlb.Max.X, tlb.Max.Y, llb.Sc, llb.Nm)
		// 			me.SetHandled()
		// 			return
		// 		}
		// 	}
		// }
		/*
			todo:
			if llb.Tooltip != "" {
				me.SetHandled()
				llb.BBoxMu.RLock()
				pos := llb.WinBBox.Max
				llb.BBoxMu.RUnlock()
				pos.X -= 20
				PopupTooltip(llb.Tooltip, pos.X, pos.Y, llb.Sc, llb.Nm)
			}
		*/
	})
}

func (lb *Label) LabelClick() {
	lb.On(events.Click, func(e events.Event) {
		fmt.Println("click")
		if lb.StateIs(states.Disabled) {
			return
		}
		hasLinks := len(lb.TextRender.Links) > 0
		if !hasLinks {
			return
		}
		pos := lb.RenderPos
		for ti := range lb.TextRender.Links {
			tl := &lb.TextRender.Links[ti]
			tlb := tl.Bounds(&lb.TextRender, pos)
			if e.Pos().In(tlb) {
				lb.OpenLink(tl)
				e.SetHandled()
				return
			}
		}
	})
	lb.On(events.DoubleClick, func(e events.Event) {
		fmt.Println("dbl click")
		if !lb.AbilityIs(states.Selectable) || lb.StateIs(states.Disabled) {
			return
		}
		updt := lb.UpdateStart()
		lb.SetState(!lb.StateIs(states.Selected), states.Selected)
		lb.UpdateEnd(updt)
	})
}

func (lb *Label) LabelMouseMove() {
	lb.On(events.MouseMove, func(e events.Event) {
		pos := lb.RenderPos
		inLink := false
		for _, tl := range lb.TextRender.Links {
			tlb := tl.Bounds(&lb.TextRender, pos)
			if e.Pos().In(tlb) {
				inLink = true
				break
			}
		}
		_ = inLink
		/*
			// TODO: figure out how to get links to work with new cursor setup
			if inLink {
				goosi.TheApp.Cursor(lb.ParentRenderWin().RenderWin).PushIfNot(cursors.Pointer)
			} else {
				goosi.TheApp.Cursor(lb.ParentRenderWin().RenderWin).PopIf(cursors.Pointer)
			}
		*/
	})
}

func (lb *Label) LabelKeys() {
	lb.On(events.KeyChord, func(e events.Event) {
		if !lb.StateIs(states.Selected) {
			return
		}
		kf := KeyFun(e.KeyChord())
		if kf == KeyFunCopy {
			e.SetHandled()
			goosi.TheApp.ClipBoard(lb.EventMgr().RenderWin().GoosiWin).Write(mimedata.NewText(lb.Text))
		}
	})
}

// StyleLabel does label styling -- it sets the StyMu Lock
func (lb *Label) StyleLabel(sc *Scene) {
	lb.StyMu.Lock()
	defer lb.StyMu.Unlock()

	lb.ApplyStyleWidget(sc)
}

func (lb *Label) LayoutLabel(sc *Scene) {
	lb.StyMu.RLock()
	defer lb.StyMu.RUnlock()

	lb.TextRender.SetHTML(lb.Text, lb.Style.FontRender(), &lb.Style.Text, &lb.Style.UnContext, lb.CSSAgg)
	spc := lb.BoxSpace()
	sz := lb.LayState.SizePrefOrMax()
	if !sz.IsNil() {
		sz.SetSub(spc.Size())
	}
	lb.TextRender.LayoutStdLR(&lb.Style.Text, lb.Style.FontRender(), &lb.Style.UnContext, sz)
}

func (lb *Label) ApplyStyle(sc *Scene) {
	lb.StyleLabel(sc)
	lb.LayoutLabel(sc)
}

func (lb *Label) GetSize(sc *Scene, iter int) {
	if iter > 0 && lb.Style.Text.HasWordWrap() {
		return // already updated in previous iter, don't redo!
	} else {
		lb.InitLayout(sc)
		sz := lb.LayState.Size.Pref // SizePrefOrMax()
		sz = sz.Max(lb.TextRender.Size)
		lb.GetSizeFromWH(sz.X, sz.Y)
	}
}

func (lb *Label) DoLayout(sc *Scene, parBBox image.Rectangle, iter int) bool {
	lb.DoLayoutBase(sc, parBBox, iter)
	lb.DoLayoutChildren(sc, iter) // todo: maybe shouldn't call this on known terminals?
	sz := lb.GetSizeSubSpace()
	lb.TextRender.SetHTML(lb.Text, lb.Style.FontRender(), &lb.Style.Text, &lb.Style.UnContext, lb.CSSAgg)
	lb.TextRender.LayoutStdLR(&lb.Style.Text, lb.Style.FontRender(), &lb.Style.UnContext, sz)
	if lb.Style.Text.HasWordWrap() {
		if lb.TextRender.Size.Y < (sz.Y - 1) { // allow for numerical issues
			lb.LayState.SetFromStyle(&lb.Style) // todo: revisit!!
			lb.GetSizeFromWH(lb.TextRender.Size.X, lb.TextRender.Size.Y)
			return true // needs a redo!
		}
	}
	return false
}

func (lb *Label) TextPos() mat32.Vec2 {
	lb.StyMu.RLock()
	pos := lb.LayState.Alloc.Pos.Add(lb.Style.BoxSpace().Pos())
	lb.StyMu.RUnlock()
	return pos
}

func (lb *Label) RenderLabel(sc *Scene) {
	rs, _, st := lb.RenderLock(sc)
	defer lb.RenderUnlock(rs)
	lb.RenderPos = lb.TextPos()
	lb.RenderStdBox(sc, st)
	lb.TextRender.Render(rs, lb.RenderPos)
}

func (lb *Label) Render(sc *Scene) {
	if lb.PushBounds(sc) {
		lb.RenderLabel(sc)
		lb.RenderChildren(sc)
		lb.PopBounds(sc)
	}
}
