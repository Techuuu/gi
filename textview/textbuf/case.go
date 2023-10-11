// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textbuf

//go:generate enumgen

import (
	"strings"

	"github.com/iancoleman/strcase"
)

// Cases are different cases -- Lower, Upper, Camel, etc
type Cases int32 //enums:enum

const (
	LowerCase Cases = iota
	UpperCase

	// CamelCase is init-caps
	CamelCase

	// LowerCamelCase has first letter lower-case
	LowerCamelCase

	// SnakeCase is snake_case -- lower with underbars
	SnakeCase

	// UpperSnakeCase is SNAKE_CASE -- upper with underbars
	UpperSnakeCase

	// KebabCase is kebab-case -- lower with -'s
	KebabCase
)

// ReCaseString changes the case of the string according to the given case type.
func ReCaseString(str string, c Cases) string {
	switch c {
	case LowerCase:
		return strings.ToLower(str)
	case UpperCase:
		return strings.ToUpper(str)
	case CamelCase:
		return strcase.ToCamel(str)
	case LowerCamelCase:
		return strcase.ToLowerCamel(str)
	case SnakeCase:
		return strcase.ToSnake(str)
	case UpperSnakeCase:
		return strcase.ToScreamingSnake(str)
	case KebabCase:
		return strcase.ToKebab(str)
	}
	return str
}
