// Copyright (c) 2023, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"fmt"
	"log/slog"
	"reflect"

	"github.com/iancoleman/strcase"
	"goki.dev/gi/v2/gi"
	"goki.dev/glop/sentencecase"
	"goki.dev/grease"
	"goki.dev/gti"
	"goki.dev/icons"
)

// MethodConfig contains the configuration options for a method button in a toolbar or menubar.
// These are the configuration options passed to the `gi:toolbar` and `gi:menubar` comment directives.
//
//gti:add
type MethodConfig struct {
	// Name is the actual name in code of the function to call.
	Name string
	// Label is the label for the method button.
	// It defaults to the sentence case version of the
	// name of the function.
	Label string
	// Icon is the icon for the method button. If there
	// is an icon with the same name as the function, it
	// defaults to that icon.
	Icon icons.Icon
	// Tooltip is the tooltip for the method button.
	// It defaults to the documentation for the function.
	Tooltip string
	// SepBefore is whether to insert a separator before the method button.
	SepBefore bool
	// SepAfter is whether to insert a separator after the method button.
	SepAfter bool
	// Args are the arguments to the method
	Args *gti.Fields
}

// ToolbarView adds the method buttons for the given value to the given toolbar.
// It returns whether any method buttons were added.
func ToolbarView(val any, tb *gi.Toolbar) bool {
	typ := gti.TypeByValue(val)
	if typ == nil {
		return false
	}
	gotAny := false
	for _, kv := range typ.Methods.Order {
		met := kv.Val
		var tbDir *gti.Directive
		for _, dir := range met.Directives {
			if dir.Tool == "gi" && dir.Directive == "toolbar" {
				tbDir = dir
				break
			}
		}
		if tbDir == nil {
			continue
		}
		cfg := &MethodConfig{
			Label:   sentencecase.Of(met.Name),
			Tooltip: met.Doc,
			Args:    met.Args,
		}
		// we default to the icon with the same name as
		// the method, if it exists
		ic := icons.Icon(strcase.ToSnake(met.Name))
		if ic.IsValid() {
			cfg.Icon = ic
		}
		_, err := grease.SetFromArgs(cfg, tbDir.Args, grease.ErrNotFound)
		if err != nil {
			slog.Error("programmer error: error while parsing args to `gi:toolbar` comment directive", "err", err.Error())
			continue
		}
		gotAny = true
		if cfg.SepBefore {
			tb.AddSeparator()
		}
		tb.AddButton(gi.ActOpts{Label: cfg.Label, Icon: cfg.Icon, Tooltip: cfg.Tooltip}, func(bt *gi.Button) {
			fmt.Println("calling method", met.Name)
			CallMethod(tb, val, cfg)
		})
		if cfg.SepAfter {
			tb.AddSeparator()
		}
	}
	return gotAny
}

// ArgConfig contains the relevant configuration information for each arg,
// including the reflect.Value, name, optional description, and default value
type ArgConfig struct {
	Val     reflect.Value
	Name    string
	Desc    string
	View    Value
	Default any
}

// CallMethod calls the method with the given configuration information on the
// given object value, using a GUI interface to prompt for args. It uses the
// given context widget for context information for the GUI interface.
// gopy:interface=handle
func CallMethod(ctx gi.Widget, val any, met *MethodConfig) bool {
	ArgViewDialog(
		ctx,
		DlgOpts{Title: met.Label, Prompt: met.Tooltip, Ok: true, Cancel: true},
		ArgConfigsFromMethod(val, met), func(dlg *gi.Dialog) {
			fmt.Println("dialog closed")
		}).Run()
	return true
}

func ArgConfigsFromMethod(val any, met *MethodConfig) []ArgConfig {
	rval := reflect.ValueOf(val)
	rmet := rval.MethodByName(met.Name)
	mtyp := rmet.Type()

	res := make([]ArgConfig, met.Args.Len())
	for i, kv := range met.Args.Order {
		arg := kv.Val
		ra := ArgConfig{
			Name: arg.Name,
			Desc: arg.Doc,
		}

		atyp := mtyp.In(i)
		ra.Val = reflect.New(atyp)

		ra.View = ToValue(ra.Val.Interface(), "")
		ra.View.SetSoloValue(ra.Val)
		ra.View.SetName(ra.Name)
		res[i] = ra
	}
	return res
}
