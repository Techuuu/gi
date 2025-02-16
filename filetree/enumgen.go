// Code generated by "goki generate"; DO NOT EDIT.

package filetree

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"

	"goki.dev/enums"
	"goki.dev/gi/v2/giv"
)

var _DirFlagsValues = []DirFlags{0, 1, 2, 3}

// DirFlagsN is the highest valid value
// for type DirFlags, plus one.
const DirFlagsN DirFlags = 4

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the enumgen command to generate them again.
func _DirFlagsNoOp() {
	var x [1]struct{}
	_ = x[DirMark-(0)]
	_ = x[DirIsOpen-(1)]
	_ = x[DirSortByName-(2)]
	_ = x[DirSortByModTime-(3)]
}

var _DirFlagsNameToValueMap = map[string]DirFlags{
	`Mark`:          0,
	`mark`:          0,
	`IsOpen`:        1,
	`isopen`:        1,
	`SortByName`:    2,
	`sortbyname`:    2,
	`SortByModTime`: 3,
	`sortbymodtime`: 3,
}

var _DirFlagsDescMap = map[DirFlags]string{
	0: `DirMark means directory is marked -- unmarked entries are deleted post-update`,
	1: `DirIsOpen means directory is open -- else closed`,
	2: `DirSortByName means sort the directory entries by name. this is mutex with other sorts -- keeping option open for non-binary sort choices.`,
	3: `DirSortByModTime means sort the directory entries by modification time`,
}

var _DirFlagsMap = map[DirFlags]string{
	0: `Mark`,
	1: `IsOpen`,
	2: `SortByName`,
	3: `SortByModTime`,
}

// String returns the string representation
// of this DirFlags value.
func (i DirFlags) String() string {
	str := ""
	for _, ie := range _DirFlagsValues {
		if i.HasFlag(ie) {
			ies := ie.BitIndexString()
			if str == "" {
				str = ies
			} else {
				str += "|" + ies
			}
		}
	}
	return str
}

// BitIndexString returns the string
// representation of this DirFlags value
// if it is a bit index value
// (typically an enum constant), and
// not an actual bit flag value.
func (i DirFlags) BitIndexString() string {
	if str, ok := _DirFlagsMap[i]; ok {
		return str
	}
	return strconv.FormatInt(int64(i), 10)
}

// SetString sets the DirFlags value from its
// string representation, and returns an
// error if the string is invalid.
func (i *DirFlags) SetString(s string) error {
	*i = 0
	return i.SetStringOr(s)
}

// SetStringOr sets the DirFlags value from its
// string representation while preserving any
// bit flags already set, and returns an
// error if the string is invalid.
func (i *DirFlags) SetStringOr(s string) error {
	flgs := strings.Split(s, "|")
	for _, flg := range flgs {
		if val, ok := _DirFlagsNameToValueMap[flg]; ok {
			i.SetFlag(true, &val)
		} else if val, ok := _DirFlagsNameToValueMap[strings.ToLower(flg)]; ok {
			i.SetFlag(true, &val)
		} else {
			return errors.New(flg + " is not a valid value for type DirFlags")
		}
	}
	return nil
}

// Int64 returns the DirFlags value as an int64.
func (i DirFlags) Int64() int64 {
	return int64(i)
}

// SetInt64 sets the DirFlags value from an int64.
func (i *DirFlags) SetInt64(in int64) {
	*i = DirFlags(in)
}

// Desc returns the description of the DirFlags value.
func (i DirFlags) Desc() string {
	if str, ok := _DirFlagsDescMap[i]; ok {
		return str
	}
	return i.String()
}

// DirFlagsValues returns all possible values
// for the type DirFlags.
func DirFlagsValues() []DirFlags {
	return _DirFlagsValues
}

// Values returns all possible values
// for the type DirFlags.
func (i DirFlags) Values() []enums.Enum {
	res := make([]enums.Enum, len(_DirFlagsValues))
	for i, d := range _DirFlagsValues {
		res[i] = d
	}
	return res
}

// IsValid returns whether the value is a
// valid option for type DirFlags.
func (i DirFlags) IsValid() bool {
	_, ok := _DirFlagsMap[i]
	return ok
}

// HasFlag returns whether these
// bit flags have the given bit flag set.
func (i DirFlags) HasFlag(f enums.BitFlag) bool {
	return atomic.LoadInt64((*int64)(&i))&(1<<uint32(f.Int64())) != 0
}

// SetFlag sets the value of the given
// flags in these flags to the given value.
func (i *DirFlags) SetFlag(on bool, f ...enums.BitFlag) {
	var mask int64
	for _, v := range f {
		mask |= 1 << v.Int64()
	}
	in := int64(*i)
	if on {
		in |= mask
		atomic.StoreInt64((*int64)(i), in)
	} else {
		in &^= mask
		atomic.StoreInt64((*int64)(i), in)
	}
}

// MarshalText implements the [encoding.TextMarshaler] interface.
func (i DirFlags) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (i *DirFlags) UnmarshalText(text []byte) error {
	return i.SetString(string(text))
}

var _NodeFlagsValues = []NodeFlags{11, 12}

// NodeFlagsN is the highest valid value
// for type NodeFlags, plus one.
const NodeFlagsN NodeFlags = 13

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the enumgen command to generate them again.
func _NodeFlagsNoOp() {
	var x [1]struct{}
	_ = x[NodeOpen-(11)]
	_ = x[NodeSymLink-(12)]
}

var _NodeFlagsNameToValueMap = map[string]NodeFlags{
	`Open`:    11,
	`open`:    11,
	`SymLink`: 12,
	`symlink`: 12,
}

var _NodeFlagsDescMap = map[NodeFlags]string{
	11: `NodeOpen means file is open -- for directories, this means that sub-files should be / have been loaded -- for files, means that they have been opened e.g., for editing`,
	12: `NodeSymLink indicates that file is a symbolic link -- file info is all for the target of the symlink`,
}

var _NodeFlagsMap = map[NodeFlags]string{
	11: `Open`,
	12: `SymLink`,
}

// String returns the string representation
// of this NodeFlags value.
func (i NodeFlags) String() string {
	str := ""
	for _, ie := range giv.TreeViewFlagsValues() {
		if i.HasFlag(ie) {
			ies := ie.BitIndexString()
			if str == "" {
				str = ies
			} else {
				str += "|" + ies
			}
		}
	}
	for _, ie := range _NodeFlagsValues {
		if i.HasFlag(ie) {
			ies := ie.BitIndexString()
			if str == "" {
				str = ies
			} else {
				str += "|" + ies
			}
		}
	}
	return str
}

// BitIndexString returns the string
// representation of this NodeFlags value
// if it is a bit index value
// (typically an enum constant), and
// not an actual bit flag value.
func (i NodeFlags) BitIndexString() string {
	if str, ok := _NodeFlagsMap[i]; ok {
		return str
	}
	return giv.TreeViewFlags(i).BitIndexString()
}

// SetString sets the NodeFlags value from its
// string representation, and returns an
// error if the string is invalid.
func (i *NodeFlags) SetString(s string) error {
	*i = 0
	return i.SetStringOr(s)
}

// SetStringOr sets the NodeFlags value from its
// string representation while preserving any
// bit flags already set, and returns an
// error if the string is invalid.
func (i *NodeFlags) SetStringOr(s string) error {
	flgs := strings.Split(s, "|")
	for _, flg := range flgs {
		if val, ok := _NodeFlagsNameToValueMap[flg]; ok {
			i.SetFlag(true, &val)
		} else if val, ok := _NodeFlagsNameToValueMap[strings.ToLower(flg)]; ok {
			i.SetFlag(true, &val)
		} else {
			err := (*giv.TreeViewFlags)(i).SetStringOr(flg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Int64 returns the NodeFlags value as an int64.
func (i NodeFlags) Int64() int64 {
	return int64(i)
}

// SetInt64 sets the NodeFlags value from an int64.
func (i *NodeFlags) SetInt64(in int64) {
	*i = NodeFlags(in)
}

// Desc returns the description of the NodeFlags value.
func (i NodeFlags) Desc() string {
	if str, ok := _NodeFlagsDescMap[i]; ok {
		return str
	}
	return giv.TreeViewFlags(i).Desc()
}

// NodeFlagsValues returns all possible values
// for the type NodeFlags.
func NodeFlagsValues() []NodeFlags {
	es := giv.TreeViewFlagsValues()
	res := make([]NodeFlags, len(es))
	for i, e := range es {
		res[i] = NodeFlags(e)
	}
	res = append(res, _NodeFlagsValues...)
	return res
}

// Values returns all possible values
// for the type NodeFlags.
func (i NodeFlags) Values() []enums.Enum {
	es := giv.TreeViewFlagsValues()
	les := len(es)
	res := make([]enums.Enum, les+len(_NodeFlagsValues))
	for i, d := range es {
		res[i] = d
	}
	for i, d := range _NodeFlagsValues {
		res[i+les] = d
	}
	return res
}

// IsValid returns whether the value is a
// valid option for type NodeFlags.
func (i NodeFlags) IsValid() bool {
	_, ok := _NodeFlagsMap[i]
	if !ok {
		return giv.TreeViewFlags(i).IsValid()
	}
	return ok
}

// HasFlag returns whether these
// bit flags have the given bit flag set.
func (i NodeFlags) HasFlag(f enums.BitFlag) bool {
	return atomic.LoadInt64((*int64)(&i))&(1<<uint32(f.Int64())) != 0
}

// SetFlag sets the value of the given
// flags in these flags to the given value.
func (i *NodeFlags) SetFlag(on bool, f ...enums.BitFlag) {
	var mask int64
	for _, v := range f {
		mask |= 1 << v.Int64()
	}
	in := int64(*i)
	if on {
		in |= mask
		atomic.StoreInt64((*int64)(i), in)
	} else {
		in &^= mask
		atomic.StoreInt64((*int64)(i), in)
	}
}

// MarshalText implements the [encoding.TextMarshaler] interface.
func (i NodeFlags) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (i *NodeFlags) UnmarshalText(text []byte) error {
	return i.SetString(string(text))
}
