package backend

import "testing"

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1 KiB"},
		{1536, "2 KiB"},
		{1024 * 1024, "1 MiB"},
		{1024 * 1024 * 1024, "1 GiB"},
		{2 * 1024 * 1024 * 1024, "2 GiB"},
	}
	for _, tt := range tests {
		got := humanBytes(tt.input)
		if got != tt.want {
			t.Errorf("humanBytes(%d): want %q, got %q", tt.input, tt.want, got)
		}
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		m    Metrics
		ok   bool
		want string
	}{
		{Metrics{}, false, "N/A"},
		{Metrics{CPUPercent: 0.0}, true, "0.0%"},
		{Metrics{CPUPercent: 12.345}, true, "12.3%"},
		{Metrics{CPUPercent: 100.0}, true, "100.0%"},
	}
	for _, tt := range tests {
		got := FormatCPU(tt.m, tt.ok)
		if got != tt.want {
			t.Errorf("FormatCPU(%v, %v): want %q, got %q", tt.m, tt.ok, tt.want, got)
		}
	}
}

func TestFormatMem(t *testing.T) {
	tests := []struct {
		m    Metrics
		ok   bool
		want string
	}{
		{Metrics{}, false, "N/A"},
		{Metrics{MemUsage: 512, MemLimit: 0}, true, "N/A"},
		{Metrics{MemUsage: 1024, MemLimit: 1024 * 1024}, true, "1 KiB / 1 MiB"},
		{Metrics{MemUsage: 0, MemLimit: 2 * 1024 * 1024 * 1024}, true, "0 B / 2 GiB"},
	}
	for _, tt := range tests {
		got := FormatMem(tt.m, tt.ok)
		if got != tt.want {
			t.Errorf("FormatMem(%v, %v): want %q, got %q", tt.m, tt.ok, tt.want, got)
		}
	}
}
