package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A drop keeps its files in order; folders, missing paths and repeats go.
func TestDroppableFiles(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.pdf")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	folder := filepath.Join(dir, "folder")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	got := droppableFiles([]string{b, folder, "", filepath.Join(dir, "gone.png"), a, b})
	if want := []string{b, a}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := droppableFiles(nil); len(got) != 0 {
		t.Fatalf("nothing dropped: %v", got)
	}
}

// Dropped files reach the UI as one files_dropped event; a drop with
// nothing sendable sends nothing.
func TestFilesDroppedEvent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.jpg")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	hub := &eventHub{clients: map[chan []byte]struct{}{}}
	c := make(chan []byte, 4)
	hub.clients[c] = struct{}{}
	m := &Messages{httpEvents: hub}

	m.filesDropped([]string{dir, file})
	select {
	case line := <-c:
		var got struct {
			Name string   `json:"name"`
			Data []string `json:"data"`
		}
		if err := json.Unmarshal(line, &got); err != nil {
			t.Fatal(err)
		}
		if got.Name != "files_dropped" || !slices.Equal(got.Data, []string{file}) {
			t.Fatalf("got %+v", got)
		}
	default:
		t.Fatal("no event")
	}

	m.filesDropped([]string{dir})
	if len(c) != 0 {
		t.Fatalf("a folder alone sent %s", <-c)
	}
}

// Windows and Linux have no menu bar: the window catches the menu's
// shortcuts, except Alt+arrows, which the UI handles (twice otherwise).
func TestWindowKeys(t *testing.T) {
	var got []string
	for _, s := range windowKeys([]shortcut{{accel: "CmdOrCtrl+K"}, {accel: "Alt+Down"}, {accel: "Alt+Shift+Up"}, {accel: "CmdOrCtrl+Shift+L"}}) {
		got = append(got, s.accel)
	}
	if want := []string{"CmdOrCtrl+K", "CmdOrCtrl+Shift+L"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Every shortcut in the menu goes through buildMenu's item, which also hands
// it to the windows without a menu bar. One set with SetAccelerator directly
// would work on macOS only.
func TestMenuShortcutsGoThroughItem(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "SetAccelerator" {
			calls++
		}
		return true
	})
	if calls != 1 {
		t.Fatalf("SetAccelerator called in %d places in main.go; use item() so Windows and Linux get the shortcut", calls)
	}
}
