package backend

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func lastCmd(t *testing.T, c *Client) string {
	t.Helper()
	log := c.CommandLog()
	if len(log) == 0 {
		t.Fatal("expected at least one recorded command")
	}
	return log[len(log)-1]
}

func TestRemoveImage_singleID_preservesCallPath(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.RemoveImage(ctx, "alpine"); err != nil {
		t.Fatal(err)
	}
	if got := lastCmd(t, c); !strings.HasSuffix(got, "image delete alpine") {
		t.Fatalf("command = %q, want suffix %q", got, "image delete alpine")
	}
}

func TestRemoveImage_multipleIDs_singleCall(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.RemoveImage(ctx, "alpine", "busybox", "debian"); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(lastCmd(t, c), " ")
	if !reflect.DeepEqual(got, []string{"container", "image", "delete", "alpine", "busybox", "debian"}) {
		t.Fatalf("command = %v, want one call with all three ids", got)
	}
}

func TestRemoveVolume_singleID_preservesCallPath(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.RemoveVolume(ctx, "data"); err != nil {
		t.Fatal(err)
	}
	if got := lastCmd(t, c); !strings.HasSuffix(got, "volume delete data") {
		t.Fatalf("command = %q, want suffix %q", got, "volume delete data")
	}
}

func TestRemoveVolume_multipleNames_singleCall(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.RemoveVolume(ctx, "data", "logs"); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(lastCmd(t, c), " ")
	if !reflect.DeepEqual(got, []string{"container", "volume", "delete", "data", "logs"}) {
		t.Fatalf("command = %v, want one call with both names", got)
	}
}

func TestRemoveImage_withNoTargets_refusesWithoutRunning(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.RemoveImage(ctx); !errors.Is(err, errNoDeleteTargets) {
		t.Fatalf("RemoveImage() error = %v, want errNoDeleteTargets", err)
	}
	if log := c.CommandLog(); len(log) != 0 {
		t.Fatalf("no command should be issued, got %v", log)
	}
}

func TestRemoveVolume_withNoTargets_refusesWithoutRunning(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.RemoveVolume(ctx); !errors.Is(err, errNoDeleteTargets) {
		t.Fatalf("RemoveVolume() error = %v, want errNoDeleteTargets", err)
	}
	if log := c.CommandLog(); len(log) != 0 {
		t.Fatalf("no command should be issued, got %v", log)
	}
}
