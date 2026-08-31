package main

import (
	"bytes"
	"encoding/binary"
	"image/gif"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The README's Demo section is the first thing a visitor sees, and its assets
// are binaries no diff review can read. This file guards the three things that
// have to stay true for that section to keep working: the links resolve, the
// GIF is a real multi-second screen recording rather than a placeholder or a
// stray still, and it stays small enough that GitHub actually plays it inline.
//
// scripts/record-demo/record.sh enforces the same 10 MiB budget at record time;
// this is the committed-state half of that check.

const (
	demoGIF   = "docs/assets/demo.gif"
	demoMP4   = "docs/assets/demo.mp4"
	gifBudget = 10 << 20

	// The recorded loop is meant to walk every view plus a live start/stop, so
	// anything materially shorter means the tape was truncated, not retimed.
	minDemoSeconds = 30

	// Below this the TUI's text is unreadable when GitHub scales the GIF down.
	minDemoWidth = 600
)

var (
	demoImageRef = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	demoLinkRef  = regexp.MustCompile(`[^!]\[[^\]]*\]\(([^)]+)\)`)
)

// demoSection returns the body of README.md's "## Demo" section.
func demoSection(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	var body []string
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			in = strings.TrimSpace(line) == "## Demo"
			continue
		}
		if in {
			body = append(body, line)
		}
	}
	if len(body) == 0 {
		t.Fatal("README.md has no '## Demo' section")
	}
	return strings.Join(body, "\n")
}

func TestReadmeDemoSectionEmbedsTheRecordedAssets(t *testing.T) {
	section := demoSection(t)

	embedded := demoImageRef.FindStringSubmatch(section)
	if embedded == nil {
		t.Fatal("README Demo section embeds no image; the GIF must render inline")
	}
	if embedded[1] != demoGIF {
		t.Errorf("Demo section embeds %q, want %q", embedded[1], demoGIF)
	}

	linked := demoLinkRef.FindStringSubmatch(section)
	if linked == nil || linked[1] != demoMP4 {
		t.Errorf("Demo section must link the full-quality %s, got %q", demoMP4, section)
	}

	for _, path := range []string{demoGIF, demoMP4} {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("README Demo references %s but it is not committed: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", path)
		}
	}
}

func TestDemoGIFIsARealRecordingWithinBudget(t *testing.T) {
	data, err := os.ReadFile(demoGIF)
	if err != nil {
		t.Fatalf("read %s: %v", demoGIF, err)
	}

	if len(data) > gifBudget {
		t.Errorf("%s is %d bytes, over the %d byte budget record.sh enforces", demoGIF, len(data), gifBudget)
	}

	cfg, err := gif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s is not a decodable GIF: %v", demoGIF, err)
	}
	if cfg.Width < minDemoWidth {
		t.Errorf("%s is %dpx wide, too narrow to read the TUI (want >= %d)", demoGIF, cfg.Width, minDemoWidth)
	}

	// Walking the block stream beats gif.DecodeAll here: 900+ full-frame
	// paletted images would allocate hundreds of MiB just to count them.
	frames, seconds := gifTiming(t, data)
	if frames < 2 {
		t.Fatalf("%s has %d frame(s); a still image is not a recording", demoGIF, frames)
	}
	if seconds < minDemoSeconds {
		t.Errorf("%s runs %.1fs across %d frames, want >= %ds of recorded session", demoGIF, seconds, frames, minDemoSeconds)
	}
}

// gifTiming counts image frames and sums their delays by stepping over the
// GIF block stream, skipping every sub-block payload without decoding it.
func gifTiming(t *testing.T, data []byte) (frames int, seconds float64) {
	t.Helper()

	i := 6 // header: "GIF89a"
	if len(data) < 13 {
		t.Fatal("GIF truncated before the logical screen descriptor")
	}
	packed := data[i+4]
	i += 7
	if packed&0x80 != 0 {
		i += 3 * (1 << ((packed & 0x07) + 1)) // global color table
	}

	skipSubBlocks := func() {
		for i < len(data) {
			n := int(data[i])
			i++
			if n == 0 {
				return
			}
			i += n
		}
	}

	var hundredths int
	for i < len(data) {
		switch data[i] {
		case 0x3B: // trailer
			return frames, float64(hundredths) / 100
		case 0x21: // extension
			label := data[i+1]
			i += 2
			if label == 0xF9 && i+4 < len(data) {
				hundredths += int(binary.LittleEndian.Uint16(data[i+2 : i+4]))
			}
			skipSubBlocks()
		case 0x2C: // image descriptor
			frames++
			local := data[i+9]
			i += 10
			if local&0x80 != 0 {
				i += 3 * (1 << ((local & 0x07) + 1)) // local color table
			}
			i++ // LZW minimum code size
			skipSubBlocks()
		default:
			t.Fatalf("unexpected GIF block 0x%02X at offset %d", data[i], i)
		}
	}
	return frames, float64(hundredths) / 100
}
