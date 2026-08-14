package filetable

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSetFiles_RendersRows(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "match.md")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.Width = 80
	m.MaxHeight = 4
	m.SetFiles([]string{tmpFile})

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if !strings.Contains(view, "match.md") {
		t.Fatalf("expected View() to contain filename; got:\n%s", view)
	}
}

func TestSetFiles_ManyFilesCapsHeight(t *testing.T) {
	tmpDir := t.TempDir()
	var paths []string
	for i := 0; i < 5; i++ {
		p := filepath.Join(tmpDir, "file"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	m := New()
	m.Width = 80
	m.MaxHeight = 4
	m.SetFiles(paths)

	view := m.View()
	if !strings.Contains(view, "filea.txt") {
		t.Fatalf("expected View() to contain at least first filename; got:\n%s", view)
	}
}

func TestSetFiles_ReplacesPreviousRows(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "aaa.txt")
	fileB := filepath.Join(tmpDir, "bbb.txt")
	for _, p := range []string{fileA, fileB} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := New()
	m.Width = 80
	m.MaxHeight = 4

	m.SetFiles([]string{fileA})
	view1 := m.View()
	if !strings.Contains(view1, "aaa.txt") {
		t.Fatalf("first call should contain aaa.txt; got:\n%s", view1)
	}

	// Second call should replace, not append.
	m.SetFiles([]string{fileB})
	view2 := m.View()
	if !strings.Contains(view2, "bbb.txt") {
		t.Fatalf("second call should contain bbb.txt; got:\n%s", view2)
	}
	if strings.Contains(view2, "aaa.txt") {
		t.Fatalf("second call should NOT contain aaa.txt; got:\n%s", view2)
	}
}

func TestWindowSizeMsg_ReRendersRows(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "resize.md")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(WithFiles([]string{tmpFile}))

	// Simulate WindowSizeMsg as Bubble Tea would send it after the program starts.
	// Update has a value receiver, so we must capture the returned Model.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()
	if !strings.Contains(view, "resize.md") {
		t.Fatalf("expected View() to contain filename after WindowSizeMsg; got:\n%s", view)
	}
}

// TestReceiverFlow_WindowSizeBeforeSetFiles verifies the exact sequence that
// broke the receiver: New() (empty table) -> WindowSizeMsg (clamps inner
// cursor to -1 via SetRows([])) -> Finalize -> SetFiles (populates rows).
// Before the cursor fix, the cursor stayed at -1 and the viewport rendered one
// fewer row than intended, yielding an empty-looking table for a single file.
func TestReceiverFlow_WindowSizeBeforeSetFiles(t *testing.T) {
	tmpDir := t.TempDir()
	var paths []string
	for i := 0; i < 3; i++ {
		p := filepath.Join(tmpDir, "file"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	m := New()
	// Receiver gets a WindowSizeMsg before any files exist.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = m.Finalize()
	// Receiver uses math.MaxInt so all rows are visible.
	m.SetMaxHeight(math.MaxInt)
	// Now populate rows, as the receiver does on unpackDoneMsg.
	m.SetFiles(paths)

	view := m.View()
	for _, p := range paths {
		name := filepath.Base(p)
		if !strings.Contains(view, name) {
			t.Fatalf("expected View() to contain %s; got:\n%s", name, view)
		}
	}
}

// TestSetFiles_NoWindowSizeYet covers the case where SetFiles is called before
// any WindowSizeMsg arrives (Width == 0). Without a fallback in getMaxWidth,
// all column widths would be 0 and bubbles would skip every cell, rendering a
// literally empty box. The fallback to MAX_WIDTH ensures columns get sane
// widths even before the first resize.
func TestSetFiles_NoWindowSizeYet(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "early.txt")
	if err := os.WriteFile(tmpFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	// No WindowSizeMsg: Width is still 0.
	m.SetMaxHeight(math.MaxInt)
	m.SetFiles([]string{tmpFile})

	view := m.View()
	if !strings.Contains(view, "early.txt") {
		t.Fatalf("expected View() to contain filename without prior WindowSizeMsg; got:\n%s", view)
	}
}
