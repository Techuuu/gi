// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"fmt"
	"image"
	"log"

	"log/slog"

	"goki.dev/colors"
	"goki.dev/cursors"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/girl/units"
	"goki.dev/goosi/events"
	"goki.dev/goosi/events/key"
	"goki.dev/icons"
	"goki.dev/ki/v2"
)

// todo: autoRepeat, autoRepeatInterval, autoRepeatDelay

// ButtonBase has common button functionality for all buttons, including
// Button, Action, MenuButton, CheckBox, etc
type ButtonBase struct {
	WidgetBase

	// label for the button -- if blank then no label is presented
	Text string `xml:"text" desc:"label for the button -- if blank then no label is presented"`

	// [view: show-name] optional icon for the button -- different buttons can configure this in different ways relative to the text if both are present
	Icon icons.Icon `xml:"icon" view:"show-name" desc:"optional icon for the button -- different buttons can configure this in different ways relative to the text if both are present"`

	// [view: show-name] name of the menu indicator icon to present, or blank or 'nil' or 'none' -- shown automatically when there are Menu elements present unless 'none' is set
	Indicator icons.Icon `xml:"indicator" view:"show-name" desc:"name of the menu indicator icon to present, or blank or 'nil' or 'none' -- shown automatically when there are Menu elements present unless 'none' is set"`

	// optional shortcut keyboard chord to trigger this action -- always window-wide in scope, and should generally not conflict other shortcuts (a log message will be emitted if so).  Shortcuts are processed after all other processing of keyboard input.  Use Command for Control / Meta (Mac Command key) per platform.  These are only set automatically for Menu items, NOT for items in ToolBar or buttons somewhere, but the tooltip for buttons will show the shortcut if set.
	Shortcut key.Chord `xml:"shortcut" desc:"optional shortcut keyboard chord to trigger this action -- always window-wide in scope, and should generally not conflict other shortcuts (a log message will be emitted if so).  Shortcuts are processed after all other processing of keyboard input.  Use Command for Control / Meta (Mac Command key) per platform.  These are only set automatically for Menu items, NOT for items in ToolBar or buttons somewhere, but the tooltip for buttons will show the shortcut if set."`

	// the menu items for this menu -- typically add Action elements for menus, along with separators
	Menu MenuActions `desc:"the menu items for this menu -- typically add Action elements for menus, along with separators"`

	// [view: -] set this to make a menu on demand -- if set then this button acts like a menu button
	MakeMenuFunc MakeMenuFunc `copy:"-" json:"-" xml:"-" view:"-" desc:"set this to make a menu on demand -- if set then this button acts like a menu button"`
}

func (bb *ButtonBase) CopyFieldsFrom(frm any) {
	fr, ok := frm.(*ButtonBase)
	if !ok {
		log.Printf("GoGi node of type: %v needs a CopyFieldsFrom method defined -- currently falling back on earlier ButtonBase one\n", bb.KiType().Name)
		return
	}
	bb.WidgetBase.CopyFieldsFrom(&fr.WidgetBase)
	bb.Text = fr.Text
	bb.Icon = fr.Icon
	bb.Indicator = fr.Indicator
	bb.Shortcut = fr.Shortcut
	bb.Menu = fr.Menu // note: can't use CopyFrom: need closure funcs in actions; todo: could do more elaborate copy etc but is it worth it?
	bb.MakeMenuFunc = fr.MakeMenuFunc
}

// ButtonFlags extend WidgetFlags to hold button state
type ButtonFlags WidgetFlags //enums:bitflag

const (
	// Menu flag means that the button is a menu item
	ButtonFlagMenu ButtonFlags = ButtonFlags(WidgetFlagsN) + iota
)

// see menus.go for MakeMenuFunc, etc

// SetCheckable sets whether this button is checkable
func (bb *ButtonBase) SetCheckable(checkable bool) {
	bb.Style.SetAbilities(checkable, states.Checkable)
}

// SetAsMenu ensures that this functions as a menu even before menu items are added
func (bb *ButtonBase) SetAsMenu() {
	bb.SetFlag(true, ButtonFlagMenu)
}

// SetAsButton clears the explicit ButtonFlagMenu -- if there are menu items
// or a menu function then it will still behave as a menu
func (bb *ButtonBase) SetAsButton() {
	bb.SetFlag(false, ButtonFlagMenu)
}

// LabelWidget returns the label widget if present
func (bb *ButtonBase) LabelWidget() *Label {
	lbi := bb.Parts.ChildByName("label")
	if lbi == nil {
		return nil
	}
	return lbi.(*Label)
}

// IconWidget returns the iconl widget if present
func (bb *ButtonBase) IconWidget() *Icon {
	ici := bb.Parts.ChildByName("icon")
	if ici == nil {
		return nil
	}
	return ici.(*Icon)
}

// SetText sets the text and updates the button.
// Use this for optimized auto-updating based on nature of changes made.
// Otherwise, can set Text directly followed by ReConfig()
func (bb *ButtonBase) SetText(txt string) ButtonWidget {
	if bb.Text == txt {
		return bb.This().(ButtonWidget)
	}
	updt := bb.UpdateStart()
	recfg := bb.Parts == nil || (bb.Text == "" && txt != "") || (bb.Text != "" && txt == "")
	bb.Text = txt
	if recfg {
		bb.This().(ButtonWidget).ConfigParts(bb.Sc)
	} else {
		lbl := bb.LabelWidget()
		if lbl != nil {
			lbl.SetText(bb.Text)
		}
	}
	bb.UpdateEndLayout(updt) // todo: could optimize to not re-layout every time but..
	return bb.This().(ButtonWidget)
}

// SetIcon sets the Icon to given icon name (could be empty or 'none') and
// updates the button.
// Use this for optimized auto-updating based on nature of changes made.
// Otherwise, can set Icon directly followed by ReConfig()
func (bb *ButtonBase) SetIcon(iconName icons.Icon) ButtonWidget {
	if bb.Icon == iconName {
		return bb.This().(ButtonWidget)
	}
	updt := bb.UpdateStart()
	recfg := (bb.Icon == "" && iconName != "") || (bb.Icon != "" && iconName == "")
	bb.Icon = iconName
	if recfg {
		bb.This().(ButtonWidget).ConfigParts(bb.Sc)
	} else {
		ic := bb.IconWidget()
		if ic != nil {
			ic.SetIcon(bb.Icon)
		}
	}
	bb.UpdateEndLayout(updt)
	return bb.This().(ButtonWidget)
}

// HasMenu returns true if there is a menu or menu-making function set, or the
// explicit ButtonFlagMenu has been set
func (bb *ButtonBase) HasMenu() bool {
	return bb.MakeMenuFunc != nil || len(bb.Menu) > 0
}

// OpenMenu will open any menu associated with this element -- returns true if
// menu opened, false if not
func (bb *ButtonBase) OpenMenu() bool {
	if !bb.HasMenu() {
		return false
	}
	if bb.MakeMenuFunc != nil {
		bb.MakeMenuFunc(bb.This().(Widget), &bb.Menu)
	}
	pos := bb.ContextMenuPos()
	if bb.Parts != nil {
		if indic := bb.Parts.ChildByName("indicator", 3); indic != nil {
			pos = indic.(Widget).ContextMenuPos()
		}
	} else {
		slog.Error("ButtonBase: parts nil", "button", bb)
	}
	NewMenu(bb.Menu, bb.This().(Widget), pos).Run()
	return true
}

// ResetMenu removes all items in the menu
func (bb *ButtonBase) ResetMenu() {
	bb.Menu = make(MenuActions, 0, 10)
}

// ConfigPartsAddIndicator adds a menu indicator if the Indicator field is set to an icon;
// if defOn is true, an indicator is added even if the Indicator field is unset
// (as long as it is not explicitly set to [icons.None]);
// returns the index in Parts of the indicator object, which is named "indicator";
// an "ind-stretch" is added as well to put on the right by default.
func (bb *ButtonBase) ConfigPartsAddIndicator(config *ki.Config, defOn bool) int {
	needInd := !bb.Indicator.IsNil() || (defOn && bb.Indicator != icons.None)
	if !needInd {
		return -1
	}
	indIdx := -1
	config.Add(StretchType, "ind-stretch")
	indIdx = len(*config)
	config.Add(IconType, "indicator")
	return indIdx
}

func (bb *ButtonBase) ConfigPartsIndicator(indIdx int) {
	if indIdx < 0 {
		return
	}
	ic := bb.Parts.Child(indIdx).(*Icon)
	icnm := bb.Indicator
	if icnm.IsNil() {
		icnm = icons.KeyboardArrowDown
	}
	ic.SetIcon(icnm)
}

//////////////////////////////////////////////////////////////////
//		Events

func (bb *ButtonBase) ClickMenu() {
	bb.On(events.Click, func(e events.Event) {
		if bb.StateIs(states.Disabled) {
			return
		}
		bb.OpenMenu()
	})
}

// ClickOnEnterSpace adds key event handler for Enter or Space
// to generate a Click action
func (bb *ButtonBase) ClickOnEnterSpace() {
	bb.On(events.KeyChord, func(e events.Event) {
		if bb.StateIs(states.Disabled) {
			return
		}
		if KeyEventTrace {
			fmt.Printf("Button KeyChordEvent: %v\n", bb.Path())
		}
		kf := KeyFun(e.KeyChord())
		if kf == KeyFunEnter || e.KeyRune() == ' ' {
			// if !(kt.Rune == ' ' && bbb.Sc.Type == ScCompleter) {
			e.SetHandled()
			bb.Send(events.Click, e)
			// }
		}
	})
}

// ShortcutTooltip returns the effective tooltip of the button
// with any keyboard shortcut included.
func (bb *ButtonBase) ShortcutTooltip() string {
	if bb.Tooltip == "" && bb.Shortcut == "" {
		return ""
	}
	res := bb.Tooltip
	if bb.Shortcut != "" {
		res = "[ " + bb.Shortcut.Shortcut() + " ]"
		if bb.Tooltip != "" {
			res += ": " + bb.Tooltip
		}
	}
	return res
}

func (bb *ButtonBase) LongHoverTooltip() {
	bb.On(events.LongHoverStart, func(e events.Event) {
		if bb.StateIs(states.Disabled) {
			return
		}
		tt := bb.ShortcutTooltip()
		if tt == "" {
			return
		}
		e.SetHandled()
		NewTooltipText(bb, tt, e.Pos()).Run()
	})
}

func (bb *ButtonBase) ButtonBaseHandlers() {
	bb.WidgetHandlers()
	bb.LongHoverTooltip()
	bb.ClickMenu()
	bb.ClickOnEnterSpace()
}

///////////////////////////////////////////////////////////
//   ButtonWidget

// ButtonWidget is an interface for button widgets allowing ButtonBase
// defaults to handle most cases.
type ButtonWidget interface {
	Widget

	// AsButtonBase gets the button base for most basic functions -- reduces
	// interface size.
	AsButtonBase() *ButtonBase

	// ConfigParts configures the parts of the button -- called during init
	// and style.
	ConfigParts(sc *Scene)

	// SetText sets the text and updates the button.
	// Use this for optimized auto-updating based on nature of changes made.
	// Otherwise, can set Text directly followed by ReConfig()
	SetText(txt string) ButtonWidget

	// SetIcon sets the Icon to given icon name (could be empty or 'none') and
	// updates the button.
	// Use this for optimized auto-updating based on nature of changes made.
	// Otherwise, can set Icon directly followed by ReConfig()
	SetIcon(iconName icons.Icon) ButtonWidget
}

///////////////////////////////////////////////////////////
// ButtonBase Widget and ButtonwWidget interface

func AsButtonBase(k ki.Ki) *ButtonBase {
	if ac, ok := k.(ButtonWidget); ok {
		return ac.AsButtonBase()
	}
	return nil
}

func (bb *ButtonBase) AsButtonBase() *ButtonBase {
	return bb
}

func (bb *ButtonBase) ConfigWidget(sc *Scene) {
	bb.This().(ButtonWidget).ConfigParts(sc)
}

func (bb *ButtonBase) ConfigParts(sc *Scene) {
	parts := bb.NewParts(LayoutHoriz)
	if bb.HasMenu() && bb.Icon.IsNil() {
		bb.Icon = icons.Menu
	}
	config := ki.Config{}
	icIdx, lbIdx := bb.ConfigPartsIconLabel(&config, bb.Icon, bb.Text)
	indIdx := bb.ConfigPartsAddIndicator(&config, false) // default off

	mods, updt := parts.ConfigChildren(config)
	bb.ConfigPartsSetIconLabel(bb.Icon, bb.Text, icIdx, lbIdx)
	bb.ConfigPartsIndicator(indIdx)
	if mods {
		parts.UpdateEnd(updt)
		bb.SetNeedsLayout(sc, updt)
	}
}

// ConfigPartsIconLabel adds to config to create parts, of icon
// and label left-to right in a row, based on whether items are nil or empty
func (bb *ButtonBase) ConfigPartsIconLabel(config *ki.Config, icnm icons.Icon, txt string) (icIdx, lbIdx int) {
	icIdx = -1
	lbIdx = -1
	if icnm.IsValid() {
		icIdx = len(*config)
		config.Add(IconType, "icon")
		if txt != "" {
			config.Add(SpaceType, "space")
		}
	}
	if txt != "" {
		lbIdx = len(*config)
		config.Add(LabelType, "label")
	}
	return
}

// ConfigPartsSetIconLabel sets the icon and text values in parts, and get
// part style props, using given props if not set in object props
func (bb *ButtonBase) ConfigPartsSetIconLabel(icnm icons.Icon, txt string, icIdx, lbIdx int) {
	if icIdx >= 0 {
		ic := bb.Parts.Child(icIdx).(*Icon)
		ic.SetIcon(icnm)
	}
	if lbIdx >= 0 {
		lbl := bb.Parts.Child(lbIdx).(*Label)
		if lbl.Text != txt {
			lbl.SetText(txt)
			lbl.Config(bb.Sc) // this is essential
		}
	}
}

func (bb *ButtonBase) ApplyStyle(sc *Scene) {
	bb.ApplyStyleWidget(sc)
	if bb.Menu != nil {
		bb.Menu.SetShortcuts(bb.EventMgr())
	}
}

func (bb *ButtonBase) DoLayout(sc *Scene, parBBox image.Rectangle, iter int) bool {
	bb.DoLayoutBase(sc, parBBox, iter)
	bb.DoLayoutParts(sc, parBBox, iter)
	return bb.DoLayoutChildren(sc, iter)
}

func (bb *ButtonBase) RenderButton(sc *Scene) {
	rs, _, st := bb.RenderLock(sc)
	bb.RenderStdBox(sc, st)
	bb.RenderUnlock(rs)
}

func (bb *ButtonBase) Render(sc *Scene) {
	if bb.PushBounds(sc) {
		bb.RenderButton(sc)
		bb.RenderParts(sc)
		bb.RenderChildren(sc)
		bb.PopBounds(sc)
	}
}

func (bb *ButtonBase) Destroy() {
	if bb.Menu != nil {
		bb.Menu.DeleteShortcuts(bb.EventMgr())
	}
}

///////////////////////////////////////////////////////////
// Button

// Button is a standard standalone button.
// Do On(events.Click) to register a function to execute when pressed.
//
//goki:embedder
type Button struct {
	ButtonBase

	// the type of button (default, primary, secondary, etc)
	Type ButtonTypes `desc:"the type of button (default, primary, secondary, etc)"`
}

// ButtonTypes is an enum containing the
// different possible types of buttons
type ButtonTypes int //enums:enum

const (
	// ButtonFilled is a filled button with a
	// contrasting background color. It should be
	// used for prominent actions, typically those
	// that are the final in a sequence. It is equivalent
	// to Material Design's filled button.
	ButtonFilled ButtonTypes = iota
	// ButtonTonal is a filled button, similar
	// to [ButtonFilled]. It is used for the same purposes,
	// but it has a lighter background color and less emphasis.
	// It is equivalent to Material Design's filled tonal button.
	ButtonTonal
	// ButtonElevated is an elevated button with
	// a light background color and a shadow.
	// It is equivalent to Material Design's elevated button.
	ButtonElevated
	// ButtonOutlined is an outlined button that is
	// used for secondary actions that are still important.
	// It is equivalent to Material Design's outlined button.
	ButtonOutlined
	// ButtonText is a low-importance button with only
	// text and/or an icon and no border, background color,
	// or shadow. They should only be used for low emphasis
	// actions, and you must ensure they stand out from the
	// surrounding context sufficiently. It is equivalent
	// to Material Design's text and icon buttons.
	ButtonText
)

func (bt *Button) OnInit() {
	bt.ButtonBaseHandlers()
	bt.ButtonStyles()
}

func (bt *Button) ButtonStyles() {
	bt.AddStyles(func(s *styles.Style) {
		s.SetAbilities(true, states.Activatable, states.Focusable, states.Hoverable)
		s.SetAbilities(bt.ShortcutTooltip() != "", states.LongHoverable)
		s.Cursor = cursors.Pointer
		s.Border.Radius = styles.BorderRadiusFull
		s.Padding.Set(units.Em(0.625*Prefs.DensityMul()), units.Em(1.5*Prefs.DensityMul()))
		if !bt.Icon.IsNil() {
			s.Padding.Left.SetEm(1 * Prefs.DensityMul())
		}
		if bt.Text == "" {
			s.Padding.Right.SetEm(1 * Prefs.DensityMul())
		}
		s.Text.Align = styles.AlignCenter
		s.MaxBoxShadow = styles.BoxShadow1()
		switch bt.Type {
		case ButtonFilled:
			s.BackgroundColor.SetSolid(colors.Scheme.Primary.Base)
			s.Color = colors.Scheme.Primary.On
			if s.Is(states.Focused) {
				s.Border.Color.Set(colors.Scheme.OnSurface) // primary is too hard to see
			}
		case ButtonTonal:
			s.BackgroundColor.SetSolid(colors.Scheme.Secondary.Container)
			s.Color = colors.Scheme.Secondary.OnContainer
		case ButtonElevated:
			s.BackgroundColor.SetSolid(colors.Scheme.SurfaceContainerLow)
			s.Color = colors.Scheme.Primary.Base
			s.MaxBoxShadow = styles.BoxShadow2()
			s.BoxShadow = styles.BoxShadow1()
		case ButtonOutlined:
			s.BackgroundColor.SetSolid(colors.Scheme.Surface)
			s.Color = colors.Scheme.Primary.Base
			s.Border.Style.Set(styles.BorderSolid)
			s.Border.Color.Set(colors.Scheme.Outline)
			s.Border.Width.Set(units.Dp(1))
		case ButtonText:
			s.Color = colors.Scheme.Primary.Base
		}
		if s.Is(states.Hovered) {
			s.BoxShadow = s.MaxBoxShadow
		}
	})
}

func (bt *Button) OnChildAdded(child ki.Ki) {
	w, _ := AsWidget(child)
	switch w.Name() {
	case "icon":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(1.125)
			s.Height.SetEm(1.125)
			s.Margin.Set()
			s.Padding.Set()
		})
	case "space":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(0.5)
			s.MinWidth.SetEm(0.5)
		})
	case "label":
		label := w.(*Label)
		label.Type = LabelLabelLarge
		w.AddStyles(func(s *styles.Style) {
			s.SetAbilities(false, states.Selectable, states.DoubleClickable)
			s.Cursor = cursors.None
			s.Text.WhiteSpace = styles.WhiteSpaceNowrap
			s.Margin.Set()
			s.Padding.Set()
			s.AlignV = styles.AlignMiddle
		})
	case "ind-stretch":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(0.5)
		})
	case "indicator":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(1.125)
			s.Height.SetEm(1.125)
			s.Margin.Set()
			s.Padding.Set()
			s.AlignV = styles.AlignBottom
		})
	}
}

func (bt *Button) CopyFieldsFrom(frm any) {
	fr := frm.(*Button)
	bt.ButtonBase.CopyFieldsFrom(&fr.ButtonBase)
}

// SetType sets the styling type of the button
func (bt *Button) SetType(typ ButtonTypes) *Button {
	updt := bt.UpdateStart()
	bt.Type = typ
	bt.UpdateEndLayout(updt)
	return bt
}

///////////////////////////////////////////////////////////
// CheckBox

// CheckBox toggles between a checked and unchecked state
type CheckBox struct {
	ButtonBase

	// [view: show-name] icon to use for the off, unchecked state of the icon -- plain Icon holds the On state -- can be set with icon-off property
	IconOff icons.Icon `xml:"icon-off" view:"show-name" desc:"icon to use for the off, unchecked state of the icon -- plain Icon holds the On state -- can be set with icon-off property"`
}

func (cb *CheckBox) CopyFieldsFrom(frm any) {
	fr := frm.(*CheckBox)
	cb.ButtonBase.CopyFieldsFrom(&fr.ButtonBase)
	cb.IconOff = fr.IconOff
}

func (cb *CheckBox) OnInit() {
	cb.ButtonBaseHandlers()
	cb.CheckBoxStyles()
}

func (cb *CheckBox) CheckBoxStyles() {
	cb.AddStyles(func(s *styles.Style) {
		s.SetAbilities(true, states.Activatable, states.Focusable, states.Hoverable, states.Checkable)
		s.SetAbilities(cb.ShortcutTooltip() != "", states.LongHoverable)
		s.Cursor = cursors.Pointer
		s.Text.Align = styles.AlignLeft
		s.Color = colors.Scheme.OnBackground
		s.Margin.Set(units.Dp(1 * Prefs.DensityMul()))
		s.Padding.Set(units.Dp(1 * Prefs.DensityMul()))
		s.Border.Style.Set(styles.BorderNone)

		if cb.Parts != nil && cb.Parts.HasChildren() {
			ist := cb.Parts.ChildByName("stack", 0).(*Layout)
			if cb.StateIs(states.Checked) {
				ist.StackTop = 0
			} else {
				ist.StackTop = 1
			}
		}
		if s.Is(states.Checked) {
			s.Color = colors.Scheme.Primary.Base
		}
		if s.Is(states.Selected) {
			s.BackgroundColor.SetSolid(colors.Scheme.Select.Container)
		}
		if s.Is(states.Disabled) {
			s.Color = colors.Scheme.SurfaceContainer
		}
	})
}

func (cb *CheckBox) OnChildAdded(child ki.Ki) {
	w, _ := AsWidget(child)
	switch w.Name() {
	case "icon0": // on
		w.AddStyles(func(s *styles.Style) {
			s.Color = colors.Scheme.Primary.Base
			s.Width.SetEm(1.5)
			s.Height.SetEm(1.5)
		})
	case "icon1": // off
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(1.5)
			s.Height.SetEm(1.5)
		})
	case "space":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetCh(0.1)
		})
	case "label":
		w.AddStyles(func(s *styles.Style) {
			s.SetAbilities(false, states.Selectable, states.DoubleClickable)
			s.Cursor = cursors.None
			s.Margin.Set()
			s.Padding.Set()
			s.AlignV = styles.AlignMiddle
		})
	}
}

// CheckBoxWidget interface

// todo: base widget will set checked state automatically, setstyle, updaterender

// // OnClicked calls the given function when the button is clicked,
// // which is the default / standard way of activating the button
// func (cb *CheckBox) OnClicked(fun func()) ButtonWidget {
// 	// cb.ButtonSig.Connect(cb.This(), func(recv, send ki.Ki, sig int64, data any) {
// 	// 	if sig == int64(ButtonToggled) {
// 	// 		fun()
// 	// 	}
// 	// })
// 	return cb.This().(ButtonWidget)
// }

// SetIcons sets the Icons (by name) for the On (checked) and Off (unchecked)
// states, and updates button
func (cb *CheckBox) SetIcons(icOn, icOff icons.Icon) {
	updt := cb.UpdateStart()
	cb.Icon = icOn
	cb.IconOff = icOff
	cb.This().(ButtonWidget).ConfigParts(cb.Sc)
	// todo: better config logic -- do layout
	cb.UpdateEnd(updt)
}

func (cb *CheckBox) ConfigWidget(sc *Scene) {
	cb.SetCheckable(true)
	cb.This().(ButtonWidget).ConfigParts(sc)
}

func (cb *CheckBox) ConfigParts(sc *Scene) {
	parts := cb.NewParts(LayoutHoriz)
	cb.SetCheckable(true)
	if !cb.Icon.IsValid() {
		cb.Icon = icons.CheckBox // fallback
	}
	if !cb.IconOff.IsValid() {
		cb.IconOff = icons.CheckBoxOutlineBlank
	}
	config := ki.Config{}
	icIdx := 0 // always there
	lbIdx := -1
	config.Add(LayoutType, "stack")
	if cb.Text != "" {
		config.Add(SpaceType, "space")
		lbIdx = len(config)
		config.Add(LabelType, "label")
	}
	mods, updt := parts.ConfigChildren(config)
	ist := parts.Child(icIdx).(*Layout)
	if mods || cb.NeedsRebuild() {
		ist.Lay = LayoutStacked
		ist.SetNChildren(2, IconType, "icon") // covered by above config update
		icon := ist.Child(0).(*Icon)
		icon.SetIcon(cb.Icon)
		icoff := ist.Child(1).(*Icon)
		icoff.SetIcon(cb.IconOff)
	}
	if cb.StateIs(states.Checked) {
		ist.StackTop = 0
	} else {
		ist.StackTop = 1
	}
	if lbIdx >= 0 {
		lbl := parts.Child(lbIdx).(*Label)
		if lbl.Text != cb.Text {
			lbl.SetText(cb.Text)
		}
	}
	if mods {
		parts.UpdateEnd(updt)
		cb.SetNeedsLayout(sc, updt)
	}
}
