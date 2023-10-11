// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

// This file contains all the Name types that drive chooser menus when they
// show up as fields or args, using the giv Value system.

// ColorName provides a value-view GUI lookup of valid color names
type ColorName string

// FontName is used to specify a font, as the unique name of the font family.
// This automatically provides a chooser menu for fonts using giv Value.
type FontName string

// FileName is used to specify an filename (including path) -- automatically
// opens the FileView dialog using Value system.  Use this for any method
// args that are filenames to trigger use of FileViewDialog under MethView
// automatic method calling.
type FileName string

// KeyMapName has an associated Value for selecting from the list of
// available key map names, for use in preferences etc.
type KeyMapName string

// HiStyleName is a highlighting style name
type HiStyleName string
