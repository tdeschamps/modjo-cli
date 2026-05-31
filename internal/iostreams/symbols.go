package iostreams

import "strings"

// A small, consistent status vocabulary used across the CLI's human output.
// Each renders a colorized glyph when color is enabled and a plain ASCII
// fallback otherwise, so piped/no-color output stays readable.

// SuccessIcon returns the success glyph (green ✓).
func (s *IOStreams) SuccessIcon() string { return s.icon("✓", "v", codeGreen) }

// ErrorIcon returns the error glyph (red ✗).
func (s *IOStreams) ErrorIcon() string { return s.icon("✗", "x", codeRed) }

// WarnIcon returns the warning glyph (yellow !).
func (s *IOStreams) WarnIcon() string { return s.icon("!", "!", codeYellow) }

// InfoIcon returns the info glyph (cyan •).
func (s *IOStreams) InfoIcon() string { return s.icon("•", "-", codeCyan) }

func (s *IOStreams) icon(glyph, asciiFallback, code string) string {
	if !s.colorEnabled {
		return asciiFallback
	}
	return code + glyph + codeReset
}

// StatusColor colorizes a deal/call status word for tables and detail views:
// open → cyan, won → green, lost → red, closed → gray. Unknown values and
// no-color output pass through unchanged.
func (s *IOStreams) StatusColor(status string) string {
	if !s.colorEnabled || status == "" {
		return status
	}
	switch {
	case strings.EqualFold(status, "open"):
		return s.colorize(codeCyan, status)
	case containsFold(status, "won"):
		return s.colorize(codeGreen, status)
	case containsFold(status, "lost"):
		return s.colorize(codeRed, status)
	case containsFold(status, "closed"), containsFold(status, "deleted"):
		return s.colorize(codeGray, status)
	default:
		return status
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), sub)
}
