// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package histyle provides syntax highlighting styles; it is based on
// github.com/alecthomas/chroma, which in turn was based on the python
// pygments package.  Note that this package depends on goki/gi and goki/pi
// and cannot be imported there; is imported into goki/gi/giv.
package histyle

//go:generate goki generate -add-types

import (
	"encoding/json"
	"image/color"
	"log/slog"
	"os"
	"strings"

	"goki.dev/colors"
	"goki.dev/gi/v2/gi"
	"goki.dev/gi/v2/keyfun"
	"goki.dev/girl/styles"
	"goki.dev/icons"
	"goki.dev/ki/v2"
	"goki.dev/pi/v2/token"
)

// Trilean value for StyleEntry value inheritance.
type Trilean int32 //enums:enum

const (
	Pass Trilean = iota
	Yes
	No
)

func (t Trilean) Prefix(s string) string {
	if t == Yes {
		return s
	} else if t == No {
		return "no" + s
	}
	return ""
}

// StyleEntry is one value in the map of highlight style values
type StyleEntry struct {

	// text color
	Color color.RGBA

	// background color
	Background color.RGBA

	// border color? not sure what this is -- not really used
	Border color.RGBA `view:"-"`

	// bold font
	Bold Trilean

	// italic font
	Italic Trilean

	// underline
	Underline Trilean

	// don't inherit these settings from sub-category or category levels -- otherwise everything with a Pass is inherited
	NoInherit bool
}

// // FromChroma copies styles from chroma
//
//	func (he *StyleEntry) FromChroma(ce chroma.StyleEntry) {
//		if ce.Colour.IsSet() {
//			he.Color.SetString(ce.Colour.String(), nil)
//		} else {
//			he.Color.SetToNil()
//		}
//		if ce.Background.IsSet() {
//			he.Background.SetString(ce.Background.String(), nil)
//		} else {
//			he.Background.SetToNil()
//		}
//		if ce.Border.IsSet() {
//			he.Border.SetString(ce.Border.String(), nil)
//		} else {
//			he.Border.SetToNil()
//		}
//		he.Bold = Trilean(ce.Bold)
//		he.Italic = Trilean(ce.Italic)
//		he.Underline = Trilean(ce.Underline)
//		he.NoInherit = ce.NoInherit
//	}
//
// // StyleEntryFromChroma returns a new style entry from corresponding chroma version
//
//	func StyleEntryFromChroma(ce chroma.StyleEntry) StyleEntry {
//		he := StyleEntry{}
//		he.FromChroma(ce)
//		return he
//	}
func (se StyleEntry) String() string {
	out := []string{}
	if se.Bold != Pass {
		out = append(out, se.Bold.Prefix("bold"))
	}
	if se.Italic != Pass {
		out = append(out, se.Italic.Prefix("italic"))
	}
	if se.Underline != Pass {
		out = append(out, se.Underline.Prefix("underline"))
	}
	if se.NoInherit {
		out = append(out, "noinherit")
	}
	if !colors.IsNil(se.Color) {
		out = append(out, colors.AsString(se.Color))
	}
	if !colors.IsNil(se.Background) {
		out = append(out, "bg:"+colors.AsString(se.Background))
	}
	if !colors.IsNil(se.Border) {
		out = append(out, "border:"+colors.AsString(se.Border))
	}
	return strings.Join(out, " ")
}

// ToCSS converts StyleEntry to CSS attributes.
func (se StyleEntry) ToCSS() string {
	styles := []string{}
	if !colors.IsNil(se.Color) {
		styles = append(styles, "color: "+colors.AsString(se.Color))
	}
	if !colors.IsNil(se.Background) {
		styles = append(styles, "background-color: "+colors.AsString(se.Background))
	}
	if se.Bold == Yes {
		styles = append(styles, "font-weight: bold")
	}
	if se.Italic == Yes {
		styles = append(styles, "font-style: italic")
	}
	if se.Underline == Yes {
		styles = append(styles, "text-decoration: underline")
	}
	return strings.Join(styles, "; ")
}

// ToProps converts StyleEntry to ki.Props attributes.
func (se StyleEntry) ToProps() ki.Props {
	pr := ki.Props{}
	if !colors.IsNil(se.Color) {
		pr["color"] = se.Color
	}
	if !colors.IsNil(se.Background) {
		pr["background-color"] = se.Background
	}
	if se.Bold == Yes {
		pr["font-weight"] = styles.WeightBold
	}
	if se.Italic == Yes {
		pr["font-style"] = styles.FontItalic
	}
	if se.Underline == Yes {
		pr["text-decoration"] = 1 << uint32(styles.DecoUnderline)
	}
	return pr
}

// Sub subtracts two style entries, returning an entry with only the differences set
func (s StyleEntry) Sub(e StyleEntry) StyleEntry {
	out := StyleEntry{}
	if e.Color != s.Color {
		out.Color = s.Color
	}
	if e.Background != s.Background {
		out.Background = s.Background
	}
	if e.Border != s.Border {
		out.Border = s.Border
	}
	if e.Bold != s.Bold {
		out.Bold = s.Bold
	}
	if e.Italic != s.Italic {
		out.Italic = s.Italic
	}
	if e.Underline != s.Underline {
		out.Underline = s.Underline
	}
	return out
}

// Inherit styles from ancestors.
//
// Ancestors should be provided from oldest, furthest away to newest, closest.
func (s StyleEntry) Inherit(ancestors ...StyleEntry) StyleEntry {
	out := s
	for i := len(ancestors) - 1; i >= 0; i-- {
		if out.NoInherit {
			return out
		}
		ancestor := ancestors[i]
		if colors.IsNil(out.Color) {
			out.Color = ancestor.Color
		}
		if colors.IsNil(out.Background) {
			out.Background = ancestor.Background
		}
		if colors.IsNil(out.Border) {
			out.Border = ancestor.Border
		}
		if out.Bold == Pass {
			out.Bold = ancestor.Bold
		}
		if out.Italic == Pass {
			out.Italic = ancestor.Italic
		}
		if out.Underline == Pass {
			out.Underline = ancestor.Underline
		}
	}
	return out
}

func (s StyleEntry) IsZero() bool {
	return colors.IsNil(s.Color) && colors.IsNil(s.Background) && colors.IsNil(s.Border) && s.Bold == Pass && s.Italic == Pass &&
		s.Underline == Pass && !s.NoInherit
}

///////////////////////////////////////////////////////////////////////////////////
//  Style

// Style is a full style map of styles for different token.Tokens tag values
type Style map[token.Tokens]*StyleEntry

// CopyFrom copies a style from source style
func (hs *Style) CopyFrom(ss *Style) {
	if ss == nil {
		return
	}
	*hs = make(Style, len(*ss))
	for k, v := range *ss {
		(*hs)[k] = v
	}
}

// TagRaw returns a StyleEntry for given tag without any inheritance of anything
// will be IsZero if not defined for this style
func (hs Style) TagRaw(tag token.Tokens) StyleEntry {
	if len(hs) == 0 {
		return StyleEntry{}
	}
	if se, has := hs[tag]; has {
		return *se
	}
	return StyleEntry{}
}

// Tag returns a StyleEntry for given Tag.
// Will try sub-category or category if an exact match is not found.
// does NOT add the background properties -- those are always kept separate.
func (hs Style) Tag(tag token.Tokens) StyleEntry {
	se := hs.TagRaw(tag).Inherit(
		hs.TagRaw(token.Text),
		hs.TagRaw(tag.Cat()),
		hs.TagRaw(tag.SubCat()))
	return se
}

// ToCSS generates a CSS style sheet for this style, by token.Tokens tag
func (hs Style) ToCSS() map[token.Tokens]string {
	css := map[token.Tokens]string{}
	for ht := range token.Names {
		entry := hs.Tag(ht)
		if entry.IsZero() {
			continue
		}
		css[ht] = entry.ToCSS()
	}
	return css
}

// ToProps generates list of ki.Props for this style
func (hs Style) ToProps() ki.Props {
	pr := ki.Props{}
	for ht, nm := range token.Names {
		entry := hs.Tag(ht)
		if entry.IsZero() {
			if tp, ok := Props[ht]; ok {
				pr["."+nm] = tp
			}
			continue
		}
		pr["."+nm] = entry.ToProps()
	}
	return pr
}

// Open hi style from a JSON-formatted file.
func (hs Style) OpenJSON(filename gi.FileName) error {
	b, err := os.ReadFile(string(filename))
	if err != nil {
		// PromptDialog(nil, "File Not Found", err.Error(), true, false, nil, nil, nil)
		slog.Error(err.Error())
		return err
	}
	return json.Unmarshal(b, &hs)
}

// Save hi style to a JSON-formatted file.
func (hs Style) SaveJSON(filename gi.FileName) error {
	b, err := json.MarshalIndent(hs, "", "  ")
	if err != nil {
		slog.Error(err.Error()) // unlikely
		return err
	}
	err = os.WriteFile(string(filename), b, 0644)
	if err != nil {
		// PromptDialog(nil, "Could not Save to File", err.Error(), true, false, nil, nil, nil)
		slog.Error(err.Error())
	}
	return err
}

// TagsProps are default properties for custom tags (tokens) -- if set in style then used
// there but otherwise we use these as a fallback -- typically not overridden
var Props = map[token.Tokens]ki.Props{
	token.TextSpellErr: {
		"text-decoration": 1 << uint32(styles.DecoDottedUnderline), // bitflag!
	},
}

// StyleProps define the Toolbar and MenuBar for view
var StyleProps = ki.Props{
	"MainMenu": ki.PropSlice{
		{"AppMenu", ki.BlankProp{}},
		{"File", ki.PropSlice{
			{"OpenJSON", ki.Props{
				"label":    "Open from file",
				"desc":     "You can save and open styles to / from files to share, experiment, transfer, etc",
				"shortcut": keyfun.Open,
				"Args": ki.PropSlice{
					{"File Name", ki.Props{
						"ext": ".histy",
					}},
				},
			}},
			{"SaveJSON", ki.Props{
				"label":    "Save to file",
				"desc":     "You can save and open styles to / from files to share, experiment, transfer, etc",
				"shortcut": keyfun.SaveAs,
				"Args": ki.PropSlice{
					{"File Name", ki.Props{
						"ext": ".histy",
					}},
				},
			}},
		}},
		{"Edit", "Copy Cut Paste Dupe"},
		{"RenderWin", "RenderWins"},
	},
	"Toolbar": ki.PropSlice{
		{"OpenJSON", ki.Props{
			"label": "Open from file",
			"icon":  icons.Open,
			"desc":  "You can save and open styles to / from files to share, experiment, transfer, etc -- save from standard ones and load into custom ones for example",
			"Args": ki.PropSlice{
				{"File Name", ki.Props{
					"ext": ".histy",
				}},
			},
		}},
		{"SaveJSON", ki.Props{
			"label": "Save to file",
			"icon":  icons.SaveAs,
			"desc":  "You can save and open styles to / from files to share, experiment, transfer, etc -- save from standard ones and load into custom ones for example",
			"Args": ki.PropSlice{
				{"File Name", ki.Props{
					"ext": ".histy",
				}},
			},
		}},
	},
}
