package iostreams

import (
	"fmt"
	"strings"
)

// BannerKind selects a banner's icon and headline color.
type BannerKind int

// Banner kinds.
const (
	BannerSuccess BannerKind = iota
	BannerInfo
	BannerWarn
	BannerError
)

// Banner is a structured block of user feedback (success/info/warning/error)
// with a bold headline, an optional body, optional "Next steps", and optional
// links. It renders to stderr because it is chrome, not data, so it never
// pollutes piped stdout. With color/TTY off it degrades to clean plain text.
type Banner struct {
	Kind      BannerKind
	Headline  string
	Body      string
	NextSteps []string
	Links     []string
}

// RenderBanner writes b to stderr.
func (s *IOStreams) RenderBanner(b Banner) {
	var icon, headline string
	switch b.Kind {
	case BannerSuccess:
		icon, headline = s.SuccessIcon(), s.colorize(codeGreen, s.Bold(b.Headline))
	case BannerWarn:
		icon, headline = s.WarnIcon(), s.colorize(codeYellow, s.Bold(b.Headline))
	case BannerError:
		icon, headline = s.ErrorIcon(), s.colorize(codeRed, s.Bold(b.Headline))
	default:
		icon, headline = s.InfoIcon(), s.Bold(b.Headline)
	}

	w := s.ErrOut
	fmt.Fprintf(w, "%s %s\n", icon, headline)
	if b.Body != "" {
		for _, line := range strings.Split(b.Body, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
	if len(b.NextSteps) > 0 {
		fmt.Fprintf(w, "\n  %s\n", s.Bold("Next steps:"))
		for _, step := range b.NextSteps {
			fmt.Fprintf(w, "  %s %s\n", s.Gray("›"), step)
		}
	}
	if len(b.Links) > 0 {
		fmt.Fprintf(w, "\n  %s\n", s.Bold("Links:"))
		for _, link := range b.Links {
			fmt.Fprintf(w, "  %s %s\n", s.Gray("›"), s.Cyan(link))
		}
	}
}
