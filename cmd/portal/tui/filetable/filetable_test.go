package filetable

import (
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
