package cmdutil

import (
	"context"
	"fmt"
	"iter"

	"github.com/tdeschamps/modjo-cli/internal/output"
)

// CollectAndRender drains a paginating iterator into a slice, then renders it
// through the factory's Printer using the supplied table fields. noun names the
// resource ("deals", "calls", …) for the progress indicator. When --all is set
// and progress is enabled (interactive stderr, not --quiet/--hide-spinner), a
// spinner reports a live count on stderr; it never touches stdout. It returns
// the first error encountered.
func CollectAndRender[T any](ctx context.Context, f *Factory, seq iter.Seq2[T, error], fields []output.Field, noun string) error {
	items := make([]T, 0)

	// The spinner is a no-op unless started, and Start itself is gated on an
	// interactive stderr — so this only animates for a real `--all` sweep.
	sp := f.IOStreams.NewSpinner("Fetching " + noun + "…")
	if f.Flags.All {
		sp.Start()
	}

	for item, err := range seq {
		if err != nil {
			sp.Stop()
			return err
		}
		items = append(items, item)
		sp.Update(fmt.Sprintf("Fetched %d %s…", len(items), noun))
	}
	sp.Stop()
	return RenderSlice(f, items, fields)
}

// GetAndRender fetches one object per id (in order), then renders the lot. It
// centralizes the "get one-or-more IDs → render" flow shared by every resource
// command's `get` subcommand.
func GetAndRender[T any](ctx context.Context, f *Factory, args []string, get func(context.Context, string) (T, error), fields []output.Field) error {
	items := make([]T, 0, len(args))
	for _, id := range args {
		item, err := get(ctx, id)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	return RenderSlice(f, items, fields)
}

// RenderSlice renders an already-collected slice of items.
func RenderSlice[T any](f *Factory, items []T, fields []output.Field) error {
	p, err := f.Printer()
	if err != nil {
		return err
	}
	return output.Render(p, items, fields)
}
