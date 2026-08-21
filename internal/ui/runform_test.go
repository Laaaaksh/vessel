package ui

import "testing"

func TestRunFormValidate_onlyImageRequired(t *testing.T) {
	f := newRunForm("alpine")
	image, opts, errMsg := f.validate()
	if errMsg != "" {
		t.Fatalf("errMsg = %q, want none", errMsg)
	}
	if image != "alpine" {
		t.Fatalf("image = %q, want alpine", image)
	}
	if opts.Name != "" || len(opts.Ports) != 0 || len(opts.Env) != 0 || len(opts.Volumes) != 0 {
		t.Fatalf("opts = %+v, want all flags empty", opts)
	}
}

func TestRunFormValidate_buildsFullOptions(t *testing.T) {
	f := newRunForm("nginx:latest")
	f.text[runFieldName] = "web"
	f.text[runFieldPorts] = "8080:80, 8443:443"
	f.text[runFieldEnv] = "FOO=bar, GREETING=hello there"
	f.text[runFieldVolumes] = "data:/var/data"
	f.text[runFieldMemory] = "512M"
	f.text[runFieldCPUs] = "2"
	f.text[runFieldArch] = "arm64"
	f.bools[runFieldDetached] = true
	f.bools[runFieldTTY] = true

	image, opts, errMsg := f.validate()
	if errMsg != "" {
		t.Fatalf("errMsg = %q, want none", errMsg)
	}
	if image != "nginx:latest" {
		t.Fatalf("image = %q", image)
	}
	wantPorts := []string{"8080:80", "8443:443"}
	if !equalStrings(opts.Ports, wantPorts) {
		t.Fatalf("Ports = %v, want %v", opts.Ports, wantPorts)
	}
	wantEnv := []string{"FOO=bar", "GREETING=hello there"}
	if !equalStrings(opts.Env, wantEnv) {
		t.Fatalf("Env = %v, want %v", opts.Env, wantEnv)
	}
	if opts.Name != "web" || opts.Memory != "512M" || opts.CPUs != "2" || opts.Arch != "arm64" {
		t.Fatalf("opts = %+v", opts)
	}
	if !opts.Detached || !opts.TTY || opts.Interactive {
		t.Fatalf("bool flags = detached=%v tty=%v interactive=%v", opts.Detached, opts.TTY, opts.Interactive)
	}
}

func TestRunFormValidate_emptyImage_reportsError(t *testing.T) {
	f := newRunForm("")
	if _, _, errMsg := f.validate(); errMsg != "image is required" {
		t.Fatalf("errMsg = %q, want %q", errMsg, "image is required")
	}
}

func TestRunFormValidate_malformedPort_namesTheBadEntry(t *testing.T) {
	f := newRunForm("alpine")
	f.text[runFieldPorts] = "8080:80, bogus"
	_, _, errMsg := f.validate()
	if errMsg != `invalid port mapping "bogus" (want host:container)` {
		t.Fatalf("errMsg = %q", errMsg)
	}
}

func TestRunFormValidate_malformedEnv_namesTheBadEntry(t *testing.T) {
	f := newRunForm("alpine")
	f.text[runFieldEnv] = "=novalue"
	_, _, errMsg := f.validate()
	if errMsg != `invalid env entry "=novalue" (want KEY=VALUE)` {
		t.Fatalf("errMsg = %q", errMsg)
	}
}

func TestRunFormValidate_malformedVolume_namesTheBadEntry(t *testing.T) {
	f := newRunForm("alpine")
	f.text[runFieldVolumes] = "novolume"
	_, _, errMsg := f.validate()
	if errMsg != `invalid volume mount "novolume" (want host:container)` {
		t.Fatalf("errMsg = %q", errMsg)
	}
}

// The run form must survive the same space/multi-byte bug as the single-token
// prompt: an env value or volume path with a space is exactly what this form
// exists for.
func TestRunFormInsert_spaceOnTextFieldAppendsLiteralSpace(t *testing.T) {
	f := newRunForm("")
	f.focus = runFieldEnv
	for _, k := range []string{"F", "O", "O", "=", "h", "i", keySpace, "t", "h", "e", "r", "e"} {
		f.insert(k)
	}
	if f.text[runFieldEnv] != "FOO=hi there" {
		t.Fatalf("text = %q, want %q", f.text[runFieldEnv], "FOO=hi there")
	}
}

func TestRunFormInsert_spaceOnBoolFieldTogglesInsteadOfAppending(t *testing.T) {
	f := newRunForm("")
	f.focus = runFieldDetached
	f.insert(keySpace)
	if !f.bools[runFieldDetached] {
		t.Fatal("space on a bool field must toggle it")
	}
	f.insert(keySpace)
	if f.bools[runFieldDetached] {
		t.Fatal("a second space must toggle it back off")
	}
	if f.text[runFieldDetached] != "" {
		t.Fatalf("bool field must not accumulate text, got %q", f.text[runFieldDetached])
	}
}

func TestRunFormInsert_multibyteRuneAppendsWholeRune(t *testing.T) {
	f := newRunForm("")
	f.focus = runFieldName
	for _, r := range "café" {
		f.insert(string(r))
	}
	if f.text[runFieldName] != "café" {
		t.Fatalf("text = %q, want %q", f.text[runFieldName], "café")
	}
}

func TestRunFormBackspace_removesWholeTrailingRune(t *testing.T) {
	f := newRunForm("")
	f.focus = runFieldName
	f.text[runFieldName] = "café"
	f.backspace()
	if f.text[runFieldName] != "caf" {
		t.Fatalf("text = %q, want %q", f.text[runFieldName], "caf")
	}
}

func TestRunFormBackspace_onBoolFieldIsNoop(t *testing.T) {
	f := newRunForm("")
	f.focus = runFieldDetached
	f.bools[runFieldDetached] = true
	f.backspace()
	if !f.bools[runFieldDetached] {
		t.Fatal("backspace must not affect a bool field")
	}
}

func TestRunFormMove_clampsAtBothEnds(t *testing.T) {
	f := newRunForm("")
	f.move(-1)
	if f.focus != 0 {
		t.Fatalf("focus = %d, want clamped to 0", f.focus)
	}
	f.focus = runFieldCount - 1
	f.move(1)
	if f.focus != runFieldCount-1 {
		t.Fatalf("focus = %d, want clamped to %d", f.focus, runFieldCount-1)
	}
}

func TestRunFormWindow_showsEverythingWhenItFits(t *testing.T) {
	start, end := runFormWindow(0, runFieldCount)
	if start != 0 || end != runFieldCount {
		t.Fatalf("window = [%d,%d), want the full range when visible >= total", start, end)
	}
}

// At the smallest supported terminal (60x12) the field list cannot all fit;
// the window must still keep the focused field inside its bounds instead of
// letting the cursor scroll off screen.
func TestRunFormWindow_keepsFocusInsideTheVisibleRange(t *testing.T) {
	visible := 4
	for focus := 0; focus < runFieldCount; focus++ {
		start, end := runFormWindow(focus, visible)
		if focus < start || focus >= end {
			t.Fatalf("focus %d outside window [%d,%d)", focus, start, end)
		}
		if end-start != visible {
			t.Fatalf("window size = %d, want %d", end-start, visible)
		}
	}
}

func TestRunFormVisibleRows_reservesForTheErrorLine(t *testing.T) {
	withoutErr := runFormVisibleRows(10, false)
	withErr := runFormVisibleRows(10, true)
	if withErr != withoutErr-1 {
		t.Fatalf("visible rows with error = %d, want one less than without (%d)", withErr, withoutErr)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
