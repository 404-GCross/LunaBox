package backend

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"lunabox/internal/wailsruntime"
)

func TestSeedCopiesOnce(t *testing.T) {
	from, dir := t.TempDir(), t.TempDir()
	write := func(path, s string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string { b, _ := os.ReadFile(path); return string(b) }
	write(filepath.Join(from, "lunabox.db"), "db1")
	write(filepath.Join(from, "appconf.json"), "{}")
	write(filepath.Join(from, "covers", "a.jpg"), "cover")
	write(filepath.Join(from, "logs", "app.log"), "log")

	if ok, err := Seed(dir, from, false); err != nil || !ok {
		t.Fatalf("first seed: %v %v", ok, err)
	}
	if read(filepath.Join(dir, "lunabox.db")) != "db1" || read(filepath.Join(dir, "covers", "a.jpg")) != "cover" {
		t.Fatal("seed did not copy database and covers")
	}
	if _, err := os.Stat(filepath.Join(dir, "logs")); err == nil {
		t.Fatal("logs should not be copied")
	}

	// 已有数据时不覆盖；force 时覆盖。来源始终不变。
	write(filepath.Join(dir, "lunabox.db"), "mine")
	write(filepath.Join(from, "lunabox.db"), "db2")
	if ok, _ := Seed(dir, from, false); ok || read(filepath.Join(dir, "lunabox.db")) != "mine" {
		t.Fatal("seed overwrote existing data")
	}
	if ok, _ := Seed(dir, from, true); !ok || read(filepath.Join(dir, "lunabox.db")) != "db2" {
		t.Fatal("forced seed did not overwrite")
	}
	if read(filepath.Join(from, "lunabox.db")) != "db2" {
		t.Fatal("source changed")
	}
}

func TestFilters(t *testing.T) {
	got := filters([]wailsruntime.FileFilter{{DisplayName: "Executables", Pattern: "*.exe;*.lnk"}, {DisplayName: "All", Pattern: "*.*"}})
	if len(got) != 2 || !slices.Equal(got[0].Extensions, []string{"exe", "lnk"}) || got[0].Name != "Executables" {
		t.Fatalf("filters = %+v", got)
	}
}

func TestEventsDispatch(t *testing.T) {
	e := NewEvents()
	var got []any
	var dispatched int
	e.SetDispatcher(func(fn func()) { dispatched++; fn() })
	if e.Emit("x", 1) {
		t.Fatal("emit without subscribers reported delivery")
	}
	e.On("x", func(d any) { got = append(got, d) })
	if !e.Emit("x", "payload") || dispatched != 1 || len(got) != 1 || got[0] != "payload" {
		t.Fatalf("got %v, dispatched %d", got, dispatched)
	}
}
