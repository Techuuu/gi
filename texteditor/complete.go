// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package texteditor

import (
	"fmt"

	"goki.dev/gi/v2/gi"
	"goki.dev/gi/v2/texteditor/textbuf"
	"goki.dev/pi/v2/complete"
	"goki.dev/pi/v2/lex"
	"goki.dev/pi/v2/parse"
	"goki.dev/pi/v2/pi"
)

// CompletePi uses GoPi symbols and language -- the string is a line of text
// up to point where user has typed.
// The data must be the *FileState from which the language type is obtained.
func CompletePi(data any, text string, posLn, posCh int) (md complete.Matches) {
	sfs := data.(*pi.FileStates)
	if sfs == nil {
		// log.Printf("CompletePi: data is nil not FileStates or is nil - can't complete\n")
		return md
	}
	lp, err := pi.LangSupport.Props(sfs.Sup)
	if err != nil {
		// log.Printf("CompletePi: %v\n", err)
		return md
	}
	if lp.Lang == nil {
		return md
	}

	// note: must have this set to ture to allow viewing of AST
	// must set it in pi/parse directly -- so it is changed in the fileparse too
	parse.GuiActive = true // note: this is key for debugging -- runs slower but makes the tree unique

	md = lp.Lang.CompleteLine(sfs, text, lex.Pos{posLn, posCh})
	return md
}

// CompleteEditPi uses the selected completion to edit the text
func CompleteEditPi(data any, text string, cursorPos int, comp complete.Completion, seed string) (ed complete.Edit) {
	sfs := data.(*pi.FileStates)
	if sfs == nil {
		// log.Printf("CompleteEditPi: data is nil not FileStates or is nil - can't complete\n")
		return ed
	}
	lp, err := pi.LangSupport.Props(sfs.Sup)
	if err != nil {
		// log.Printf("CompleteEditPi: %v\n", err)
		return ed
	}
	if lp.Lang == nil {
		return ed
	}
	return lp.Lang.CompleteEdit(sfs, text, cursorPos, comp, seed)
}

// LookupPi uses GoPi symbols and language -- the string is a line of text
// up to point where user has typed.
// The data must be the *FileState from which the language type is obtained.
func LookupPi(data any, text string, posLn, posCh int) (ld complete.Lookup) {
	sfs := data.(*pi.FileStates)
	if sfs == nil {
		// log.Printf("LookupPi: data is nil not FileStates or is nil - can't lookup\n")
		return ld
	}
	lp, err := pi.LangSupport.Props(sfs.Sup)
	if err != nil {
		// log.Printf("LookupPi: %v\n", err)
		return ld
	}
	if lp.Lang == nil {
		return ld
	}

	// note: must have this set to ture to allow viewing of AST
	// must set it in pi/parse directly -- so it is changed in the fileparse too
	parse.GuiActive = true // note: this is key for debugging -- runs slower but makes the tree unique

	ld = lp.Lang.Lookup(sfs, text, lex.Pos{posLn, posCh})
	if len(ld.Text) > 0 {
		// TextViewDialog(nil, DlgOpts{Title: "Lookup: " + text}, ld.Text, nil)
		return ld
	}
	if ld.Filename != "" {
		txt := textbuf.FileRegionBytes(ld.Filename, ld.StLine, ld.EdLine, true, 10) // comments, 10 lines back max
		_ = txt
		prmpt := fmt.Sprintf("%v [%d:%d]", ld.Filename, ld.StLine, ld.EdLine)
		_ = prmpt
		// TextViewDialog(nil, DlgOpts{Title: "Lookup: " + text, Prompt: prmpt, Filename: ld.Filename, LineNos: true, Data: prmpt}, txt, nil)
		return ld
	}

	return ld
}

// CompleteText does completion for text files
func CompleteText(data any, text string, posLn, posCh int) (md complete.Matches) {
	// todo:
	// err := gi.InitSpell() // text completion uses the spell code to generate completions and suggestions
	// if err != nil {
	// 	fmt.Printf("Could not initialize spelling model: Spelling model needed for text completion: %v", err)
	// 	return md
	// }

	md.Seed = complete.SeedWhiteSpace(text)
	if md.Seed == "" {
		return md
	}
	result := gi.CompleteText(md.Seed)
	possibles := complete.MatchSeedString(result, md.Seed)
	for _, p := range possibles {
		m := complete.Completion{Text: p, Icon: ""}
		md.Matches = append(md.Matches, m)
	}
	return md
}

// CompleteTextEdit uses the selected completion to edit the text
func CompleteTextEdit(data any, text string, cursorPos int, completion complete.Completion, seed string) (ed complete.Edit) {
	ed = gi.CompleteEditText(text, cursorPos, completion.Text, seed)
	return ed
}
