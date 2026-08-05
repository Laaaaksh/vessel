package uiutil

// MoveCursor applies a navigation delta with clamping.
func MoveCursor(cursor, n, delta int) int {
	if n <= 0 {
		return 0
	}
	cursor += delta
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// PageDelta returns ±page size for page/half-page keys.
func PageDelta(page int, half bool, down bool) int {
	if page < 1 {
		page = 1
	}
	d := page
	if half {
		d = max(1, page/2)
	}
	if !down {
		return -d
	}
	return d
}
