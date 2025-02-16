// Code generated by "goki generate -add-types"; DO NOT EDIT.

package histyle

import (
	"goki.dev/gti"
	"goki.dev/ordmap"
)

var _ = gti.AddType(&gti.Type{
	Name:      "goki.dev/gi/v2/texteditor/histyle.Trilean",
	ShortName: "histyle.Trilean",
	IDName:    "trilean",
	Doc:       "Trilean value for StyleEntry value inheritance.",
	Directives: gti.Directives{
		&gti.Directive{Tool: "enums", Directive: "enum", Args: []string{}},
	},

	Methods: ordmap.Make([]ordmap.KeyVal[string, *gti.Method]{}),
})

var _ = gti.AddType(&gti.Type{
	Name:       "goki.dev/gi/v2/texteditor/histyle.StyleEntry",
	ShortName:  "histyle.StyleEntry",
	IDName:     "style-entry",
	Doc:        "StyleEntry is one value in the map of highlight style values",
	Directives: gti.Directives{},
	Fields: ordmap.Make([]ordmap.KeyVal[string, *gti.Field]{
		{"Color", &gti.Field{Name: "Color", Type: "image/color.RGBA", LocalType: "color.RGBA", Doc: "text color", Directives: gti.Directives{}, Tag: ""}},
		{"Background", &gti.Field{Name: "Background", Type: "image/color.RGBA", LocalType: "color.RGBA", Doc: "background color", Directives: gti.Directives{}, Tag: ""}},
		{"Border", &gti.Field{Name: "Border", Type: "image/color.RGBA", LocalType: "color.RGBA", Doc: "border color? not sure what this is -- not really used", Directives: gti.Directives{}, Tag: "view:\"-\""}},
		{"Bold", &gti.Field{Name: "Bold", Type: "goki.dev/gi/v2/texteditor/histyle.Trilean", LocalType: "Trilean", Doc: "bold font", Directives: gti.Directives{}, Tag: ""}},
		{"Italic", &gti.Field{Name: "Italic", Type: "goki.dev/gi/v2/texteditor/histyle.Trilean", LocalType: "Trilean", Doc: "italic font", Directives: gti.Directives{}, Tag: ""}},
		{"Underline", &gti.Field{Name: "Underline", Type: "goki.dev/gi/v2/texteditor/histyle.Trilean", LocalType: "Trilean", Doc: "underline", Directives: gti.Directives{}, Tag: ""}},
		{"NoInherit", &gti.Field{Name: "NoInherit", Type: "bool", LocalType: "bool", Doc: "don't inherit these settings from sub-category or category levels -- otherwise everything with a Pass is inherited", Directives: gti.Directives{}, Tag: ""}},
	}),
	Embeds:  ordmap.Make([]ordmap.KeyVal[string, *gti.Field]{}),
	Methods: ordmap.Make([]ordmap.KeyVal[string, *gti.Method]{}),
})

var _ = gti.AddType(&gti.Type{
	Name:       "goki.dev/gi/v2/texteditor/histyle.Style",
	ShortName:  "histyle.Style",
	IDName:     "style",
	Doc:        "Style is a full style map of styles for different token.Tokens tag values",
	Directives: gti.Directives{},

	Methods: ordmap.Make([]ordmap.KeyVal[string, *gti.Method]{}),
})

var _ = gti.AddType(&gti.Type{
	Name:       "goki.dev/gi/v2/texteditor/histyle.Styles",
	ShortName:  "histyle.Styles",
	IDName:     "styles",
	Doc:        "Styles is a collection of styles",
	Directives: gti.Directives{},

	Methods: ordmap.Make([]ordmap.KeyVal[string, *gti.Method]{
		{"OpenJSON", &gti.Method{Name: "OpenJSON", Doc: "Open hi styles from a JSON-formatted file. You can save and open\nstyles to / from files to share, experiment, transfer, etc.", Directives: gti.Directives{
			&gti.Directive{Tool: "gti", Directive: "add", Args: []string{}},
		}, Args: ordmap.Make([]ordmap.KeyVal[string, *gti.Field]{
			{"filename", &gti.Field{Name: "filename", Type: "goki.dev/gi/v2/gi.FileName", LocalType: "gi.FileName", Doc: "", Directives: gti.Directives{}, Tag: ""}},
		}), Returns: ordmap.Make([]ordmap.KeyVal[string, *gti.Field]{
			{"error", &gti.Field{Name: "error", Type: "error", LocalType: "error", Doc: "", Directives: gti.Directives{}, Tag: ""}},
		})}},
		{"SaveJSON", &gti.Method{Name: "SaveJSON", Doc: "Save hi styles to a JSON-formatted file. You can save and open\nstyles to / from files to share, experiment, transfer, etc.", Directives: gti.Directives{
			&gti.Directive{Tool: "gti", Directive: "add", Args: []string{}},
		}, Args: ordmap.Make([]ordmap.KeyVal[string, *gti.Field]{
			{"filename", &gti.Field{Name: "filename", Type: "goki.dev/gi/v2/gi.FileName", LocalType: "gi.FileName", Doc: "", Directives: gti.Directives{}, Tag: ""}},
		}), Returns: ordmap.Make([]ordmap.KeyVal[string, *gti.Field]{
			{"error", &gti.Field{Name: "error", Type: "error", LocalType: "error", Doc: "", Directives: gti.Directives{}, Tag: ""}},
		})}},
	}),
})
