// Code generated by "goki generate"; DO NOT EDIT.

package keyfun

import (
	"errors"
	"strconv"
	"strings"

	"goki.dev/enums"
)

var _FunsValues = []Funs{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65}

// FunsN is the highest valid value
// for type Funs, plus one.
const FunsN Funs = 66

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the enumgen command to generate them again.
func _FunsNoOp() {
	var x [1]struct{}
	_ = x[Nil-(0)]
	_ = x[MoveUp-(1)]
	_ = x[MoveDown-(2)]
	_ = x[MoveRight-(3)]
	_ = x[MoveLeft-(4)]
	_ = x[PageUp-(5)]
	_ = x[PageDown-(6)]
	_ = x[Home-(7)]
	_ = x[End-(8)]
	_ = x[DocHome-(9)]
	_ = x[DocEnd-(10)]
	_ = x[WordRight-(11)]
	_ = x[WordLeft-(12)]
	_ = x[FocusNext-(13)]
	_ = x[FocusPrev-(14)]
	_ = x[Enter-(15)]
	_ = x[Accept-(16)]
	_ = x[CancelSelect-(17)]
	_ = x[SelectMode-(18)]
	_ = x[SelectAll-(19)]
	_ = x[Abort-(20)]
	_ = x[Copy-(21)]
	_ = x[Cut-(22)]
	_ = x[Paste-(23)]
	_ = x[PasteHist-(24)]
	_ = x[Backspace-(25)]
	_ = x[BackspaceWord-(26)]
	_ = x[Delete-(27)]
	_ = x[DeleteWord-(28)]
	_ = x[Kill-(29)]
	_ = x[Duplicate-(30)]
	_ = x[Transpose-(31)]
	_ = x[TransposeWord-(32)]
	_ = x[Undo-(33)]
	_ = x[Redo-(34)]
	_ = x[Insert-(35)]
	_ = x[InsertAfter-(36)]
	_ = x[ZoomOut-(37)]
	_ = x[ZoomIn-(38)]
	_ = x[Prefs-(39)]
	_ = x[Refresh-(40)]
	_ = x[Recenter-(41)]
	_ = x[Complete-(42)]
	_ = x[Lookup-(43)]
	_ = x[Search-(44)]
	_ = x[Find-(45)]
	_ = x[Replace-(46)]
	_ = x[Jump-(47)]
	_ = x[HistPrev-(48)]
	_ = x[HistNext-(49)]
	_ = x[Menu-(50)]
	_ = x[WinFocusNext-(51)]
	_ = x[WinClose-(52)]
	_ = x[WinSnapshot-(53)]
	_ = x[GoGiEditor-(54)]
	_ = x[New-(55)]
	_ = x[NewAlt1-(56)]
	_ = x[NewAlt2-(57)]
	_ = x[Open-(58)]
	_ = x[OpenAlt1-(59)]
	_ = x[OpenAlt2-(60)]
	_ = x[Save-(61)]
	_ = x[SaveAs-(62)]
	_ = x[SaveAlt-(63)]
	_ = x[CloseAlt1-(64)]
	_ = x[CloseAlt2-(65)]
}

var _FunsNameToValueMap = map[string]Funs{
	`Nil`:           0,
	`nil`:           0,
	`MoveUp`:        1,
	`moveup`:        1,
	`MoveDown`:      2,
	`movedown`:      2,
	`MoveRight`:     3,
	`moveright`:     3,
	`MoveLeft`:      4,
	`moveleft`:      4,
	`PageUp`:        5,
	`pageup`:        5,
	`PageDown`:      6,
	`pagedown`:      6,
	`Home`:          7,
	`home`:          7,
	`End`:           8,
	`end`:           8,
	`DocHome`:       9,
	`dochome`:       9,
	`DocEnd`:        10,
	`docend`:        10,
	`WordRight`:     11,
	`wordright`:     11,
	`WordLeft`:      12,
	`wordleft`:      12,
	`FocusNext`:     13,
	`focusnext`:     13,
	`FocusPrev`:     14,
	`focusprev`:     14,
	`Enter`:         15,
	`enter`:         15,
	`Accept`:        16,
	`accept`:        16,
	`CancelSelect`:  17,
	`cancelselect`:  17,
	`SelectMode`:    18,
	`selectmode`:    18,
	`SelectAll`:     19,
	`selectall`:     19,
	`Abort`:         20,
	`abort`:         20,
	`Copy`:          21,
	`copy`:          21,
	`Cut`:           22,
	`cut`:           22,
	`Paste`:         23,
	`paste`:         23,
	`PasteHist`:     24,
	`pastehist`:     24,
	`Backspace`:     25,
	`backspace`:     25,
	`BackspaceWord`: 26,
	`backspaceword`: 26,
	`Delete`:        27,
	`delete`:        27,
	`DeleteWord`:    28,
	`deleteword`:    28,
	`Kill`:          29,
	`kill`:          29,
	`Duplicate`:     30,
	`duplicate`:     30,
	`Transpose`:     31,
	`transpose`:     31,
	`TransposeWord`: 32,
	`transposeword`: 32,
	`Undo`:          33,
	`undo`:          33,
	`Redo`:          34,
	`redo`:          34,
	`Insert`:        35,
	`insert`:        35,
	`InsertAfter`:   36,
	`insertafter`:   36,
	`ZoomOut`:       37,
	`zoomout`:       37,
	`ZoomIn`:        38,
	`zoomin`:        38,
	`Prefs`:         39,
	`prefs`:         39,
	`Refresh`:       40,
	`refresh`:       40,
	`Recenter`:      41,
	`recenter`:      41,
	`Complete`:      42,
	`complete`:      42,
	`Lookup`:        43,
	`lookup`:        43,
	`Search`:        44,
	`search`:        44,
	`Find`:          45,
	`find`:          45,
	`Replace`:       46,
	`replace`:       46,
	`Jump`:          47,
	`jump`:          47,
	`HistPrev`:      48,
	`histprev`:      48,
	`HistNext`:      49,
	`histnext`:      49,
	`Menu`:          50,
	`menu`:          50,
	`WinFocusNext`:  51,
	`winfocusnext`:  51,
	`WinClose`:      52,
	`winclose`:      52,
	`WinSnapshot`:   53,
	`winsnapshot`:   53,
	`GoGiEditor`:    54,
	`gogieditor`:    54,
	`New`:           55,
	`new`:           55,
	`NewAlt1`:       56,
	`newalt1`:       56,
	`NewAlt2`:       57,
	`newalt2`:       57,
	`Open`:          58,
	`open`:          58,
	`OpenAlt1`:      59,
	`openalt1`:      59,
	`OpenAlt2`:      60,
	`openalt2`:      60,
	`Save`:          61,
	`save`:          61,
	`SaveAs`:        62,
	`saveas`:        62,
	`SaveAlt`:       63,
	`savealt`:       63,
	`CloseAlt1`:     64,
	`closealt1`:     64,
	`CloseAlt2`:     65,
	`closealt2`:     65,
}

var _FunsDescMap = map[Funs]string{
	0:  ``,
	1:  ``,
	2:  ``,
	3:  ``,
	4:  ``,
	5:  ``,
	6:  ``,
	7:  `PageRight PageLeft`,
	8:  ``,
	9:  ``,
	10: ``,
	11: ``,
	12: ``,
	13: ``,
	14: ``,
	15: ``,
	16: ``,
	17: ``,
	18: ``,
	19: ``,
	20: ``,
	21: `EditItem`,
	22: ``,
	23: ``,
	24: ``,
	25: ``,
	26: ``,
	27: ``,
	28: ``,
	29: ``,
	30: ``,
	31: ``,
	32: ``,
	33: ``,
	34: ``,
	35: ``,
	36: ``,
	37: ``,
	38: ``,
	39: ``,
	40: ``,
	41: ``,
	42: ``,
	43: ``,
	44: ``,
	45: ``,
	46: ``,
	47: ``,
	48: ``,
	49: ``,
	50: ``,
	51: ``,
	52: ``,
	53: ``,
	54: ``,
	55: `Below are menu specific functions -- use these as shortcuts for menu buttons allows uniqueness of mapping and easy customization of all key buttons`,
	56: ``,
	57: ``,
	58: ``,
	59: ``,
	60: ``,
	61: ``,
	62: ``,
	63: ``,
	64: ``,
	65: ``,
}

var _FunsMap = map[Funs]string{
	0:  `Nil`,
	1:  `MoveUp`,
	2:  `MoveDown`,
	3:  `MoveRight`,
	4:  `MoveLeft`,
	5:  `PageUp`,
	6:  `PageDown`,
	7:  `Home`,
	8:  `End`,
	9:  `DocHome`,
	10: `DocEnd`,
	11: `WordRight`,
	12: `WordLeft`,
	13: `FocusNext`,
	14: `FocusPrev`,
	15: `Enter`,
	16: `Accept`,
	17: `CancelSelect`,
	18: `SelectMode`,
	19: `SelectAll`,
	20: `Abort`,
	21: `Copy`,
	22: `Cut`,
	23: `Paste`,
	24: `PasteHist`,
	25: `Backspace`,
	26: `BackspaceWord`,
	27: `Delete`,
	28: `DeleteWord`,
	29: `Kill`,
	30: `Duplicate`,
	31: `Transpose`,
	32: `TransposeWord`,
	33: `Undo`,
	34: `Redo`,
	35: `Insert`,
	36: `InsertAfter`,
	37: `ZoomOut`,
	38: `ZoomIn`,
	39: `Prefs`,
	40: `Refresh`,
	41: `Recenter`,
	42: `Complete`,
	43: `Lookup`,
	44: `Search`,
	45: `Find`,
	46: `Replace`,
	47: `Jump`,
	48: `HistPrev`,
	49: `HistNext`,
	50: `Menu`,
	51: `WinFocusNext`,
	52: `WinClose`,
	53: `WinSnapshot`,
	54: `GoGiEditor`,
	55: `New`,
	56: `NewAlt1`,
	57: `NewAlt2`,
	58: `Open`,
	59: `OpenAlt1`,
	60: `OpenAlt2`,
	61: `Save`,
	62: `SaveAs`,
	63: `SaveAlt`,
	64: `CloseAlt1`,
	65: `CloseAlt2`,
}

// String returns the string representation
// of this Funs value.
func (i Funs) String() string {
	if str, ok := _FunsMap[i]; ok {
		return str
	}
	return strconv.FormatInt(int64(i), 10)
}

// SetString sets the Funs value from its
// string representation, and returns an
// error if the string is invalid.
func (i *Funs) SetString(s string) error {
	if val, ok := _FunsNameToValueMap[s]; ok {
		*i = val
		return nil
	}
	if val, ok := _FunsNameToValueMap[strings.ToLower(s)]; ok {
		*i = val
		return nil
	}
	return errors.New(s + " is not a valid value for type Funs")
}

// Int64 returns the Funs value as an int64.
func (i Funs) Int64() int64 {
	return int64(i)
}

// SetInt64 sets the Funs value from an int64.
func (i *Funs) SetInt64(in int64) {
	*i = Funs(in)
}

// Desc returns the description of the Funs value.
func (i Funs) Desc() string {
	if str, ok := _FunsDescMap[i]; ok {
		return str
	}
	return i.String()
}

// FunsValues returns all possible values
// for the type Funs.
func FunsValues() []Funs {
	return _FunsValues
}

// Values returns all possible values
// for the type Funs.
func (i Funs) Values() []enums.Enum {
	res := make([]enums.Enum, len(_FunsValues))
	for i, d := range _FunsValues {
		res[i] = d
	}
	return res
}

// IsValid returns whether the value is a
// valid option for type Funs.
func (i Funs) IsValid() bool {
	_, ok := _FunsMap[i]
	return ok
}

// MarshalText implements the [encoding.TextMarshaler] interface.
func (i Funs) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (i *Funs) UnmarshalText(text []byte) error {
	return i.SetString(string(text))
}
