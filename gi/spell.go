// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"image"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"goki.dev/gi/v2/keyfun"
	"goki.dev/glop/dirs"
	"goki.dev/goosi"
	"goki.dev/goosi/events"
	"goki.dev/ki/v2"
	"goki.dev/pi/v2/spell"
)

// InitSpell tries to load the saved fuzzy.spell model.
// If unsuccessful tries to create a new model from a text file used as input
func InitSpell() error {
	if spell.Initialized() {
		return nil
	}
	err := OpenSpellModel()
	if err != nil {
		err = spell.OpenDefault()
		if err != nil {
			slog.Error(err.Error())
		}
	}
	return nil
}

// OpenSpellModel loads a saved spelling model
func OpenSpellModel() error {
	pdir := goosi.TheApp.GoGiPrefsDir()
	openpath := filepath.Join(pdir, "spell_en_us.json")
	err := spell.Open(openpath)
	if err != nil {
		log.Printf("ERROR opening spelling dictionary: %s  error: %s\n", openpath, err)
	}
	return err
}

// NewSpellModelFromText builds a NEW spelling model from text
func NewSpellModelFromText() error {
	bigdatapath, err := dirs.GoSrcDir("goki.dev/pi/v2/spell")
	if err != nil {
		log.Printf("Error getting path to corpus directory: %v.\n", err)
		return err
	}

	bigdatafile := filepath.Join(bigdatapath, "big.txt")
	file, err := os.Open(bigdatafile)
	if err != nil {
		slog.Error("Could not open corpus file. This file is used to create the spelling model", "file", bigdatafile, "err", err)
		NewDialog(nil).Title("Corpus File Not Found").Prompt("You can build a spelling model to check against by clicking the \"Train\" button and selecting text files to train on.").Ok().Run()
		return err
	}

	err = spell.Train(*file, true) // true - create a NEW spelling model
	if err != nil {
		log.Printf("Failed building model from corpus file: %v.\n", err)
		return err
	}

	SaveSpellModel()

	return nil
}

// AddToSpellModel trains on additional text - extends model
func AddToSpellModel(filepath string) error {
	InitSpell() // make sure model is initialized
	file, err := os.Open(filepath)
	if err != nil {
		log.Printf("Could not open text file selected for training: %v.\n", err)
		return err
	}

	err = spell.Train(*file, false) // false - append rather than create new
	if err != nil {
		log.Printf("Failed appending to spell model: %v.\n", err)
		return err
	}
	return nil
}

// SaveSpellModel saves the spelling model which includes the data and parameters
func SaveSpellModel() error {
	pdir := goosi.TheApp.GoGiPrefsDir()
	path := filepath.Join(pdir, "spell_en_us.json")
	err := spell.Save(path)
	if err != nil {
		log.Printf("Could not save spelling model to file: %v.\n", err)
	}
	return err
}

////////////////////////////////////////////////////////////////////////////////////////
// Spell

// Spell
type Spell struct {
	ki.Node

	// line number in source that spelling is operating on, if relevant
	SrcLn int

	// character position in source that spelling is operating on (start of word to be corrected)
	SrcCh int

	// list of suggested corrections
	Suggest []string

	// word being checked
	Word string `set:"-"`

	// last word learned -- can be undone -- stored in lowercase format
	LastLearned string `set:"-"`

	// [view: -] signal for Spell -- see SpellSignals for the types
	// SpellSig ki.Signal `json:"-" xml:"-" view:"-" desc:"signal for Spell -- see SpellSignals for the types"`

	// the user's correction selection
	Correction string `set:"-"`

	// the scene where the current popup menu is presented
	Sc *Scene ` set:"-"`
}

func (sp *Spell) Disconnect() {
	// sp.Node.Disconnect()
	// sp.SpellSig.DisconnectAll()
}

// SpellSignals are signals that are sent by Spell
type SpellSignals int32 //enums:enum -trim-prefix Spell

const (
	// SpellSelect means the user chose one of the possible corrections
	SpellSelect SpellSignals = iota

	// SpellIgnore signals the user chose ignore so clear the tag
	SpellIgnore
)

// CheckWord checks the model to determine if the word is known.
// automatically checks the Ignore list first.
func (sp *Spell) CheckWord(word string) ([]string, bool) {
	return spell.CheckWord(word)
}

// SetWord sets the word to spell and other associated info
func (sp *Spell) SetWord(word string, sugs []string, srcLn, srcCh int) *Spell {
	sp.Word = word
	sp.Suggest = sugs
	sp.SrcLn = srcLn
	sp.SrcCh = srcCh
	return sp
}

// Show is the main call for listing spelling corrections.
// Calls ShowNow which builds the correction popup menu
// Similar to completion.Show but does not use a timer
// Displays popup immediately for any unknown word
func (sp *Spell) Show(text string, sc *Scene, pt image.Point) {
	// TODO(kai/menu): what should we do here?
	// if sc == nil || sc.Win == nil {
	// 	return
	// }
	// cpop := sc.Win.CurPopup()
	// if PopupIsCorrector(cpop) {
	// 	sc.Win.SetDelPopup(cpop)
	// }
	// sp.ShowNow(text, sc, pt)
}

// ShowNow actually builds the correction popup menu
func (sp *Spell) ShowNow(word string, m *Scene, pt image.Point) {
	// TODO(kai/menu): what should we do here?
	// if sc == nil || sc.Win == nil {
	// 	return
	// }
	// cpop := sc.Win.CurPopup()
	// if PopupIsCorrector(cpop) {
	// 	sc.Win.SetDelPopup(cpop)
	// }

	var text string
	if sp.IsLastLearned(word) {
		text = "unlearn"
		NewButton(m, text).SetText(text).SetData(text).OnClick(func(e events.Event) {
			sp.UnLearnLast()
		})
	} else {
		count := len(sp.Suggest)
		if count == 1 && sp.Suggest[0] == word {
			return
		}
		if count == 0 {
			text = "no suggestion"
			NewButton(m, text).SetText(text).SetData(text)
		} else {
			for i := 0; i < count; i++ {
				text = sp.Suggest[i]
				NewButton(m, text).SetText(text).SetData(text).OnClick(func(e events.Event) {
					sp.Spell(text)
				})
			}
		}
		NewSeparator(m)
		text = "learn"
		NewButton(m, text).SetText(text).SetData(text).OnClick(func(e events.Event) {
			sp.LearnWord()
		})
		text = "ignore"
		NewButton(m, text).SetText(text).SetData(text).OnClick(func(e events.Event) {
			sp.IgnoreWord()
		})
	}
	// TODO(kai/menu): what should we do here?
	// scp := PopupMenu(m, pt.X, pt.Y, sc, "tf-spellcheck-menu")
	// scp.Type = ScCorrector
	// psc.Child(0).SetProp("no-focus-name", true) // disable name focusing -- grabs key events in popup instead of in textfield!
}

// Spell emits a signal to let subscribers know that the user has made a
// selection from the list of possible corrections
func (sp *Spell) Spell(s string) {
	sp.Cancel()
	sp.Correction = s
	// sp.SpellSig.Emit(sp.This(), int64(SpellSelect), s)
}

// KeyInput is the opportunity for the spelling correction popup to act on specific key inputs
func (sp *Spell) KeyInput(kf keyfun.Funs) bool { // true - caller should set key processed
	switch kf {
	case keyfun.MoveDown:
		return true
	case keyfun.MoveUp:
		return true
	}
	return false
}

// LearnWord gets the misspelled/unknown word and passes to LearnWord
func (sp *Spell) LearnWord() {
	sp.LastLearned = strings.ToLower(sp.Word)
	spell.LearnWord(sp.Word)
	// sp.SpellSig.Emit(sp.This(), int64(SpellSelect), sp.Word)
}

// IsLastLearned returns true if given word was the last one learned
func (sp *Spell) IsLastLearned(wrd string) bool {
	lword := strings.ToLower(wrd)
	return lword == sp.LastLearned
}

// UnLearnLast unlearns the last learned word -- in case accidental
func (sp *Spell) UnLearnLast() {
	if sp.LastLearned == "" {
		slog.Error("spell.UnLearnLast: no last learned word")
		return
	}
	lword := sp.LastLearned
	sp.LastLearned = ""
	spell.UnLearnWord(lword)
}

// IgnoreWord adds the word to the ignore list
func (sp *Spell) IgnoreWord() {
	spell.IgnoreWord(sp.Word)
	// sp.SpellSig.Emit(sp.This(), int64(SpellIgnore), sp.Word)
}

// Cancel cancels any pending spell correction -- call when new events nullify prior correction
// returns true if canceled
func (sp *Spell) Cancel() bool {
	// TODO(kai/menu): what should we do here?
	// if sp.Sc == nil || sp.Sc.Win == nil {
	// 	return false
	// }
	// cpop := sp.Sc.Win.CurPopup()
	// did := false
	// if PopupIsCorrector(cpop) {
	// 	did = true
	// 	sp.Sc.Win.SetDelPopup(cpop)
	// }
	// sp.Sc = nil
	// return did
	return false
}
