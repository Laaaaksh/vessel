package containers

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
)

// column returns the rune offset of sub within s, ignoring styling escapes.
func column(t *testing.T, s, sub string) int {
	t.Helper()
	plain := ansi.Strip(s)
	i := strings.Index(plain, sub)
	if i < 0 {
		t.Fatalf("%q not found in %q", sub, plain)
	}
	return utf8.RuneCountInString(plain[:i])
}

// Names shorter than colName must still fill the column, otherwise every data
// cell after NAME slides left and stops lining up with its header.
func TestRenderRowAlignsWithHeader(t *testing.T) {
	const width = 100
	m := New().SetItems([]backend.Container{
		{ID: "a", Name: "short", Status: "running"},
		{ID: "b", Name: "a-much-longer-container-name-that-overflows", Status: "stopped"},
	})

	header := m.renderHeader(width)
	wantStatus := column(t, header, "STATUS")

	for i, c := range m.filtered {
		row := m.renderRow(c, false, width, nil)
		if got := column(t, row, c.Status); got != wantStatus {
			t.Errorf("row %d (%s): status at column %d, header STATUS at %d", i, c.Name, got, wantStatus)
		}
	}
}
