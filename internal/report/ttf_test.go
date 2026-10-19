// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import "testing"

func TestPDFFontCoversLatinCyrillicVietnamese(t *testing.T) {
	for _, name := range []string{"Geist-Regular.ttf", "Geist-SemiBold.ttf"} {
		f, err := loadPDFFont(name)
		if err != nil {
			t.Fatal(err)
		}
		for label, s := range map[string]string{
			"latin":      "AaZz09 .,:;!?()[]/%&-_ŠšČčĆćŽžĐđ éèêëàâçñüöäß ğşıİ",
			"cyrillic":   "АБВГДЂЕЖЗИЈКЛЉМНЊОПРСТЋУФХЦЧЏШабвгдђежзијклљмнњопрстћуфхцчџшЁё",
			"vietnamese": "ăâđêôơưĂÂĐÊÔƠƯàáảãạằắẳẵặầấẩẫậèéẻẽẹềếểễệìíỉĩịòóỏõọồốổỗộờớởỡợùúủũụừứửữựỳýỷỹỵ",
			"punct":      "•–—…©™®",
		} {
			for _, r := range s {
				if !f.has(r) {
					t.Errorf("%s: %s lacks %s glyph U+%04X %c", name, label, label, r, r)
				}
			}
		}
	}
}
