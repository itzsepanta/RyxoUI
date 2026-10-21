// Package diff tells two texts apart.
//
// The panel shows the difference between the configuration a node runs now
// and the configuration it would run, so an operator sees what a change does
// before it is written.
package diff

// Line is one line of a difference.
type Line struct {
	// Kind is "same", "added" or "removed".
	Kind string `json:"kind"`
	// Text is the line itself.
	Text string `json:"text"`
}

// The kinds of a line.
const (
	Same    = "same"
	Added   = "added"
	Removed = "removed"
)

// Lines tells two texts apart, line by line.
//
// The lines that both texts share are kept, the lines that only one of them
// has are marked.
func Lines(before, after string) []Line {
	old := split(before)
	fresh := split(after)

	// The table of the lengths of the common subsequences, so the lines that
	// did not change are found.
	table := make([][]int, len(old)+1)
	for i := range table {
		table[i] = make([]int, len(fresh)+1)
	}
	for i := len(old) - 1; i >= 0; i-- {
		for j := len(fresh) - 1; j >= 0; j-- {
			switch {
			case old[i] == fresh[j]:
				table[i][j] = table[i+1][j+1] + 1
			case table[i+1][j] >= table[i][j+1]:
				table[i][j] = table[i+1][j]
			default:
				table[i][j] = table[i][j+1]
			}
		}
	}

	out := []Line{}
	i, j := 0, 0
	for i < len(old) && j < len(fresh) {
		switch {
		case old[i] == fresh[j]:
			out = append(out, Line{Kind: Same, Text: old[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			out = append(out, Line{Kind: Removed, Text: old[i]})
			i++
		default:
			out = append(out, Line{Kind: Added, Text: fresh[j]})
			j++
		}
	}
	for ; i < len(old); i++ {
		out = append(out, Line{Kind: Removed, Text: old[i]})
	}
	for ; j < len(fresh); j++ {
		out = append(out, Line{Kind: Added, Text: fresh[j]})
	}
	return out
}

// Changed reports whether the two texts differ.
func Changed(before, after string) bool {
	return before != after
}

// split reads the lines of a text.
func split(text string) []string {
	if text == "" {
		return nil
	}
	lines := []string{}
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
