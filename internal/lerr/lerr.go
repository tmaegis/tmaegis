// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package lerr builds errors whose message comes from the i18n catalog while still
// supporting errors.Is / errors.As on the wrapped cause.
package lerr

import (
	"github.com/tmaegis/tmaegis/internal/i18n"
)

type localized struct {
	msg string
	err error
}

func (e *localized) Error() string { return e.msg }
func (e *localized) Unwrap() error { return e.err }

// New returns an error with the translated message for key.
func New(key string, args ...any) error {
	return &localized{msg: i18n.T(key, args...)}
}

// Wrap returns an error with the translated message for key (the cause is available to
// the template as {error}) that unwraps to err.
func Wrap(key string, err error, args ...any) error {
	args = append(args[:len(args):len(args)], "error", err.Error())
	return &localized{msg: i18n.T(key, args...), err: err}
}
