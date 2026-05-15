package layout

import (
	"bytes"
	"strings"
)

// JoinColumnsInto assembles diffPane + scrollbar (no sidebar) into sb, line by line.
func JoinColumnsInto(sb *bytes.Buffer, _ string, diffPane, scrollbar string, bodyH int) {
	sb.Grow(len(diffPane) + len(scrollbar) + bodyH)
	di, sci := 0, 0
	for i := range bodyH {
		if nl := strings.IndexByte(diffPane[di:], '\n'); nl >= 0 {
			sb.WriteString(diffPane[di : di+nl])
			di += nl + 1
		} else {
			sb.WriteString(diffPane[di:])
			di = len(diffPane)
		}
		if nl := strings.IndexByte(scrollbar[sci:], '\n'); nl >= 0 {
			sb.WriteString(scrollbar[sci : sci+nl])
			sci += nl + 1
		} else {
			sb.WriteString(scrollbar[sci:])
			sci = len(scrollbar)
		}
		if i < bodyH-1 {
			sb.WriteByte('\n')
		}
	}
}

// JoinColumnsAllInto assembles sidebar + divChar + diffPane + scrollbar into sb
// in a single pass. Avoids any intermediate alloc by writing directly into the
// caller-provided buffer (typically a model-local bytes.Buffer).
func JoinColumnsAllInto(sb *bytes.Buffer, sidebar, divChar, diffPane, scrollbar string, bodyH int) {
	sb.Grow(len(sidebar) + len(divChar)*bodyH + len(diffPane) + len(scrollbar) + bodyH)
	si, di, sci := 0, 0, 0
	for i := range bodyH {
		if nl := strings.IndexByte(sidebar[si:], '\n'); nl >= 0 {
			sb.WriteString(sidebar[si : si+nl])
			si += nl + 1
		} else {
			sb.WriteString(sidebar[si:])
			si = len(sidebar)
		}
		sb.WriteString(divChar)
		if nl := strings.IndexByte(diffPane[di:], '\n'); nl >= 0 {
			sb.WriteString(diffPane[di : di+nl])
			di += nl + 1
		} else {
			sb.WriteString(diffPane[di:])
			di = len(diffPane)
		}
		if nl := strings.IndexByte(scrollbar[sci:], '\n'); nl >= 0 {
			sb.WriteString(scrollbar[sci : sci+nl])
			sci += nl + 1
		} else {
			sb.WriteString(scrollbar[sci:])
			sci = len(scrollbar)
		}
		if i < bodyH-1 {
			sb.WriteByte('\n')
		}
	}
}
