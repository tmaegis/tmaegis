// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package i18n

// Reviewed lists the locales whose catalogs a native speaker has reviewed. Every other locale
// is machine-translated and is labelled "Beta" in language pickers until it is added here.
// Add a tag only after a native speaker signed off the cli, report, gui and web catalogs.
var Reviewed = map[string]bool{
	"en-US": true, "en-GB": true, // source language
}

// IsBeta reports whether a locale is still awaiting native review.
func IsBeta(tag string) bool { return !Reviewed[Resolve(tag)] }

// Label is the endonym shown in language pickers, with a Beta marker for unreviewed locales.
func Label(tag string) string {
	tag = Resolve(tag)
	if IsBeta(tag) {
		return Names[tag] + " · Beta"
	}
	return Names[tag]
}

// Labels returns Label for every supported tag.
func Labels() map[string]string {
	out := make(map[string]string, len(Tags))
	for _, t := range Tags {
		out[t] = Label(t)
	}
	return out
}
