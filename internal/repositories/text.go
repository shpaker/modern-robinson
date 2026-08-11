package repositories

import "strings"

// decodeCP1251 converts Windows-1251 bytes to UTF-8. The game's texts use the
// Cyrillic block (0xC0-0xFF maps linearly to U+0410) plus Ё/ё at 0xA8/0xB8 and
// the «»–„“ punctuation used in dialogue.
func decodeCP1251(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		switch {
		case c < 0x80:
			sb.WriteByte(c)
		case c >= 0xC0:
			sb.WriteRune(rune(0x0410 + int(c) - 0xC0))
		case c == 0xA8:
			sb.WriteRune('Ё')
		case c == 0xB8:
			sb.WriteRune('ё')
		case c == 0xAB:
			sb.WriteRune('«')
		case c == 0xBB:
			sb.WriteRune('»')
		case c == 0x96:
			sb.WriteRune('–')
		case c == 0x97:
			sb.WriteRune('—')
		case c == 0x84:
			sb.WriteRune('„')
		case c == 0x93:
			sb.WriteRune('“')
		case c == 0x94:
			sb.WriteRune('”')
		case c == 0x85:
			sb.WriteRune('…')
		default:
			sb.WriteRune(' ')
		}
	}
	return sb.String()
}

// Texts returns the global string table (STARTUP.DAN:TEXT.DAT): one quoted
// CP1251 line per string; Text ids are 0-based line numbers. Quotes are kept —
// the original UI shows dialogue lines with them.
func (r *Resources) Texts() []string {
	c := r.SceneContainer("STARTUP")
	if c == nil {
		return nil
	}
	d, err := c.ExtractName("TEXT.DAT")
	if err != nil {
		return nil
	}
	raw := strings.ReplaceAll(decodeCP1251(d), "\r\n", "\n")
	return strings.Split(raw, "\n")
}
