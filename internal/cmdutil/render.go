package cmdutil

import (
	"context"
	"iter"

	"github.com/tdeschamps/modjo-cli/internal/output"
)

// CollectAndRender drains a paginating iterator into a slice, then renders it
// through the factory's Printer using the supplied table fields. It returns the
// first error encountered. This centralizes the "list → render" flow every
// resource command shares.
func CollectAndRender[T any](ctx context.Context, f *Factory, seq iter.Seq2[T, error], fields []output.Field) error {
	items := make([]T, 0)
	for item, err := range seq {
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

// RenderOne renders a single object (not wrapped in an array).
func RenderOne[T any](f *Factory, item T, fields []output.Field) error {
	p, err := f.Printer()
	if err != nil {
		return err
	}
	return p.Output(item, []any{item}, fields)
}
