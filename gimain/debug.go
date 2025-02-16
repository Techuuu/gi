// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debug
// +build debug

package gimain

import (
	"fmt"

	"goki.dev/gi/v2/gi"
	"goki.dev/ki/v2"
)

// DebugEnumSizes is a startup function that reports current sizes of some big
// enums, just to make sure everything is well below 64..
func DebugEnumSizes() {
	fmt.Printf("ki.WidgetFlagsN: %d\n", ki.FlagsN)
	fmt.Printf("gi.WidgetFlagsN: %d\n", gi.WidgetFlagsN)
	fmt.Printf("gi.WinFlagN: %d\n", gi.WinFlagsN)
	fmt.Printf("gi.ScFlagN: %d\n", gi.ScFlagsN)
}
