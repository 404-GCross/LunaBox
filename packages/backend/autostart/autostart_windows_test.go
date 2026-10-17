package autostart

import "testing"

func TestCommandExe(t *testing.T) {
	for cmd, want := range map[string]string{
		`"C:\Program Files\LunaBox\LunaBox.exe" --autostart`: `C:\Program Files\LunaBox\LunaBox.exe`,
		`C:\LunaBox\LunaBox.exe --autostart`:                 `C:\LunaBox\LunaBox.exe`,
		`  C:\LunaBox\LunaBox.exe`:                           `C:\LunaBox\LunaBox.exe`,
	} {
		if got := commandExe(cmd); got != want {
			t.Errorf("commandExe(%q) = %q, want %q", cmd, got, want)
		}
	}
}
