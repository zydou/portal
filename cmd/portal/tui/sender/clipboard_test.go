package sender

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/aymanbagabas/go-osc52/v2"
)

// TestOsc52Sequence_Default verifies that an OSC 52 sequence for plain data is
// generated in the standard form: ESC ] 52 ; c ; <base64> BEL, with the base64
// payload round-tripping back to the original string.
func TestOsc52Sequence_Default(t *testing.T) {
	t.Setenv("TMUX", "")

	want := "portal receive 2-carbon-liquid-star --relay 124.221.8.23:433"
	seq := osc52Sequence(want)

	const (
		esc = "\x1b"
		bel = "\x07"
		st  = "\x1b\\"
	)
	got := seq.String()

	// Must be a single, well-formed OSC 52 sequence.
	if !strings.HasPrefix(got, esc+"]52;c;") {
		t.Fatalf("sequence does not start with ESC ] 52 ; c ; ; got: %q", got)
	}
	if !strings.HasSuffix(got, bel) {
		t.Fatalf("sequence does not end with BEL; got: %q", got)
	}
	// No screen/tmux wrapping in default mode.
	if strings.Contains(got, st) {
		t.Fatalf("default-mode sequence should not contain tmux/screen wrapping; got: %q", got)
	}

	// Extract and decode the base64 payload.
	body := strings.TrimSuffix(strings.TrimPrefix(got, esc+"]52;c;"), bel)
	decoded, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("payload is not valid base64 %q: %v", body, err)
	}
	if string(decoded) != want {
		t.Fatalf("decoded payload %q does not match original %q", decoded, want)
	}
}

// TestOsc52Sequence_Tmux verifies that the tmux wrapping is applied when the
// TMUX environment variable is set.
func TestOsc52Sequence_Tmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	seq := osc52Sequence("some-command")
	got := seq.String()

	// In tmux mode the sequence is wrapped in a DCS passthrough.
	if !strings.HasPrefix(got, "\x1bPtmux;\x1b") {
		t.Fatalf("tmux-wrapped sequence should start with DCS passthrough; got: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b\\") {
		t.Fatalf("tmux-wrapped sequence should end with ST; got: %q", got)
	}

	// And the inner OSC 52 must still be well-formed.
	if !strings.Contains(got, "\x1b]52;c;") {
		t.Fatalf("wrapped sequence missing inner OSC 52 ; got: %q", got)
	}
}

// TestOsc52Sequence_Empty verifies empty data still produces a valid (empty)
// clipboard-clearing sequence rather than panicking.
func TestOsc52Sequence_Empty(t *testing.T) {
	t.Setenv("TMUX", "")

	seq := osc52.New("")
	got := seq.String()
	if !strings.HasPrefix(got, "\x1b]52;c;") || !strings.HasSuffix(got, "\x07") {
		t.Fatalf("unexpected empty-data sequence: %q", got)
	}
}
