package sender

import (
	"fmt"
	"os"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
)

// copyToClipboard copies data to the system clipboard using the best available
// backend, trying them in order of preference:
//
//  1. A native clipboard backend, if one is available on the host (macOS
//     pbcopy, Linux xclip/xsel/wl-copy, ...). This is the most capable backend
//     and is preferred wherever it exists.
//  2. An OSC 52 escape sequence emitted to the terminal. When no native backend
//     is available — typically the case when portal runs on a headless server
//     reached over SSH — this is the mechanism that actually gets the data onto
//     the user's *local* clipboard: the escape sequence travels through the SSH
//     connection to the terminal emulator (Ghostty, iTerm2, Terminal.app, ...),
//     which interprets OSC 52 and applies it to the local machine's clipboard.
//
// Both backends are best-effort. If neither can place the data on a clipboard
// (e.g. portal runs in a non-interactive, redirected pipeline with no
// controlling terminal and no OSC 52-capable stdout), an error is returned so
// the caller can decide how to surface it.
func copyToClipboard(data string) error {
	if err := clipboard.WriteAll(data); err == nil {
		return nil
	}
	return writeOSC52(data)
}

// osc52Sequence builds the OSC 52 escape sequence for data. If the process is
// running inside tmux, the sequence is escaped for tmux's set-clipboard
// passthrough; otherwise the default mode is used (natively supported by
// Ghostty, iTerm2, Terminal.app, kitty, and most modern terminals).
func osc52Sequence(data string) osc52.Sequence {
	seq := osc52.New(data)
	if os.Getenv("TMUX") != "" {
		seq = seq.Tmux()
	}
	return seq
}

// writeOSC52 emits an OSC 52 sequence for data to a terminal emulator. The
// sequence is written to the controlling terminal (/dev/tty) when one is
// present so that a redirection of stdout/stderr does not swallow it. When
// there is no controlling terminal, the sequence is written to stderr instead,
// matching the convention used by the upstream osc52 library.
func writeOSC52(data string) error {
	seq := osc52Sequence(data)

	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, os.ModeCharDevice); err == nil {
		defer func() { _ = tty.Close() }()
		if _, err := seq.WriteTo(tty); err != nil {
			return err
		}
		return nil
	}

	if _, err := seq.WriteTo(os.Stderr); err != nil {
		return fmt.Errorf("writing OSC 52 sequence to stderr: %w", err)
	}
	return nil
}
