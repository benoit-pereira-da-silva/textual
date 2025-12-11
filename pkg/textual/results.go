package textual

import "strings"

type Results []Result

// Aggregated concatenates the Text fields of results and rebases
// all fragment positions into the coordinate space of the aggregated
// Text. Errors are merged by taking the first non‑nil error.
func (r Results) Aggregated() Result {
	var aggregated Result
	if len(r) == 0 {
		return aggregated
	}
	var builder strings.Builder

	// Precompute capacity and first error, if any.
	totalFragments := 0
	for _, res := range r {
		totalFragments += len(res.Fragments)
		if aggregated.Error == nil && res.Error != nil {
			aggregated.Error = res.Error
		}
	}
	aggregated.Fragments = make([]Fragment, 0, totalFragments)
	aggregated.Index = -1
	offset := 0 // rune offset in the aggregated Text

	for _, res := range r {
		textStr := res.Text
		builder.WriteString(textStr)

		// Compute the rune length for offset rebasing.
		runeLen := len([]rune(textStr))

		for _, f := range res.Fragments {
			adjusted := f
			adjusted.Pos += offset
			aggregated.Fragments = append(aggregated.Fragments, adjusted)
		}
		offset += runeLen
	}
	aggregated.Text = builder.String()
	return aggregated
}
