package uiutil

// AppendLines appends as many of more as the budget allows and drops the rest,
// so a pane never renders taller than the height it was given.
func AppendLines(lines []string, budget int, more ...string) []string {
	for _, l := range more {
		if len(lines) >= budget {
			return lines
		}
		lines = append(lines, l)
	}
	return lines
}

// Section appends a blank spacer, a header and as many items as the budget
// allows. The whole section is skipped when the spacer, the header and at
// least one item do not fit, so a full pane never shows a dangling heading.
func Section(lines []string, budget int, header string, items []string) []string {
	if len(items) == 0 || len(lines)+3 > budget {
		return lines
	}
	lines = append(lines, "", header)
	return AppendLines(lines, budget, items...)
}
