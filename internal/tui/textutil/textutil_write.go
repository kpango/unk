package textutil

import (
	"bytes"
	"unsafe"
)

// SpaceBuf is a pre-initialized all-spaces slice used for bulk padding writes.
var SpaceBuf [512]byte

func init() {
	for i := range SpaceBuf {
		SpaceBuf[i] = ' '
	}
}

// WriteSpaces writes exactly n space bytes into sb.
func WriteSpaces(sb *bytes.Buffer, n int) {
	for n > len(SpaceBuf) {
		sb.Write(SpaceBuf[:])
		n -= len(SpaceBuf)
	}
	if n > 0 {
		sb.Write(SpaceBuf[:n])
	}
}

// WriteDecimalInt writes the decimal representation of n into sb. Zero allocations.
func WriteDecimalInt(sb *bytes.Buffer, n int) {
	if n <= 0 {
		sb.WriteByte('0')
		return
	}
	var buf [20]byte
	i := len(buf)
	for v := n; v > 0; v /= 10 {
		i--
		buf[i] = byte('0' + v%10)
	}
	sb.Write(buf[i:])
}

// WriteRightJustifiedInt writes n right-justified in a field of (w-1) columns
// followed by a trailing space. Writes all spaces for n <= 0. Zero allocations.
func WriteRightJustifiedInt(sb *bytes.Buffer, n, w int) {
	if w <= 0 {
		return
	}
	if n <= 0 {
		WriteSpaces(sb, w)
		return
	}
	if w == 4 {
		var b [4]byte
		b[3] = ' '
		switch {
		case n < 10:
			b[0] = ' '; b[1] = ' '; b[2] = byte('0' + n)
		case n < 100:
			b[0] = ' '; b[1] = byte('0' + n/10); b[2] = byte('0' + n%10)
		case n < 1000:
			b[0] = byte('0' + n/100); b[1] = byte('0' + (n/10)%10); b[2] = byte('0' + n%10)
		default:
			b[0] = byte('0' + (n/1000)%10); b[1] = byte('0' + (n/100)%10)
			b[2] = byte('0' + (n/10)%10); b[3] = byte('0' + n%10)
		}
		sb.Write(b[:])
		return
	}
	var buf [20]byte
	i := len(buf)
	for v := n; v > 0; v /= 10 {
		i--
		buf[i] = byte('0' + v%10)
	}
	nDigits := len(buf) - i
	if pad := w - 1 - nDigits; pad > 0 {
		WriteSpaces(sb, pad)
	}
	sb.Write(buf[i:])
	sb.WriteByte(' ')
}

// BuildCell constructs the prebuilt cell string: hl + pad spaces + ANSI reset.
func BuildCell(hl string, pad int) string {
	n := len(hl) + pad + 4 // 4 = len("\x1b[m")
	b := make([]byte, n)
	copy(b, hl)
	for i := range pad {
		b[len(hl)+i] = ' '
	}
	copy(b[len(hl)+pad:], "\x1b[m")
	return unsafe.String(unsafe.SliceData(b), n)
}
