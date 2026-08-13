#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
    exit 0
fi

if ! command -v go >/dev/null 2>&1; then
    echo "ERROR: go is required to patch Wails Linux tray support." >&2
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    echo "ERROR: python3 is required to patch Wails Linux tray support." >&2
    exit 1
fi

module_version="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)"
go mod download github.com/wailsapp/wails/v3

module_dir="$(go env GOMODCACHE)/github.com/wailsapp/wails/v3@${module_version}"
tray_file="${module_dir}/pkg/application/systemtray_linux.go"
common_file="${module_dir}/pkg/application/systemtray.go"

if [[ ! -f "$tray_file" || ! -f "$common_file" ]]; then
    echo "ERROR: Wails source files were not found in module cache: ${module_dir}" >&2
    exit 1
fi

chmod u+w "$tray_file" "$common_file"

python3 - "$tray_file" "$common_file" <<'PY'
from pathlib import Path
import sys

tray_path = Path(sys.argv[1])
common_path = Path(sys.argv[2])

tray = tray_path.read_text()
common = common_path.read_text()

if "LunaBox patch: keep Linux tray menus host-rendered" not in common:
    old = '''\tif s.rightClickHandler == nil && hasMenu {
\t\ts.rightClickHandler = s.ShowMenu
\t}
'''
    new = '''\t// LunaBox patch: keep Linux tray menus host-rendered through StatusNotifierItem.Menu.
\t// Wails v3 beta.5 OpenMenu is not implemented on Linux, so installing ShowMenu
\t// as the default right-click handler eats the tray host's context-menu event.
\tif s.rightClickHandler == nil && hasMenu && runtime.GOOS != "linux" {
\t\ts.rightClickHandler = s.ShowMenu
\t}
'''
    if old not in common:
        raise SystemExit("failed to patch Wails systemtray.go: smart-default block not found")
    common = common.replace(old, new, 1)
    common_path.write_text(common)

if "LunaBox patch: carry tooltip into Linux tray implementation" not in tray:
    old = '''\t\tlabel:          label,
\t\ticon:           s.icon,
\t\tmenu:           s.menu,
'''
    new = '''\t\tlabel:          label,
\t\t// LunaBox patch: carry tooltip into Linux tray implementation before DBus export.
\t\ttooltip:        s.tooltip,
\t\ticon:           s.icon,
\t\tmenu:           s.menu,
'''
    if old not in tray:
        raise SystemExit("failed to patch Wails systemtray_linux.go: impl fields not found")
    tray = tray.replace(old, new, 1)

if "LunaBox patch: update DBus ToolTip property" not in tray:
    old = '''func (s *linuxSystemTray) setTooltip(_ string) {
\t// TBD
}
'''
    new = '''func (s *linuxSystemTray) setTooltip(value string) {
\t// LunaBox patch: update DBus ToolTip property instead of leaving KDE/Plasma on "Wails".
\ts.tooltip = value
\tif s.props == nil {
\t\treturn
\t}
\ttooltipText := value
\tif tooltipText == "" {
\t\ttooltipText = s.label
\t}
\tif err := s.props.Set("org.kde.StatusNotifierItem", "ToolTip", dbus.MakeVariant(tooltip{V2: tooltipText})); err != nil {
\t\tglobalApplication.error("systray error: failed to set ToolTip prop: %w", err)
\t}
}
'''
    if old not in tray:
        raise SystemExit("failed to patch Wails systemtray_linux.go: setTooltip block not found")
    tray = tray.replace(old, new, 1)

if "LunaBox patch: use the configured tooltip text during DBus export" not in tray:
    old = '''func (s *linuxSystemTray) createPropSpec() map[string]map[string]*prop.Prop {
\tprops := map[string]*prop.Prop{
'''
    new = '''func (s *linuxSystemTray) createPropSpec() map[string]map[string]*prop.Prop {
\t// LunaBox patch: use the configured tooltip text during DBus export.
\ttooltipText := s.tooltip
\tif tooltipText == "" {
\t\ttooltipText = s.label
\t}
\tprops := map[string]*prop.Prop{
'''
    if old not in tray:
        raise SystemExit("failed to patch Wails systemtray_linux.go: createPropSpec start not found")
    tray = tray.replace(old, new, 1)

if 'Value:    tooltip{V2: tooltipText},' not in tray:
    old = '''\t\t"ToolTip": {
\t\t\tValue:    tooltip{V2: s.label},
'''
    new = '''\t\t"ToolTip": {
\t\t\tValue:    tooltip{V2: tooltipText},
'''
    if old not in tray:
        raise SystemExit("failed to patch Wails systemtray_linux.go: ToolTip property not found")
    tray = tray.replace(old, new, 1)

if "LunaBox patch: always export DBus menu path" not in tray:
    old = '''\tif s.menu != nil {
\t\tprops["Menu"] = &prop.Prop{
\t\t\tValue:    dbus.ObjectPath(menuPath),
\t\t\tWritable: true,
\t\t\tEmit:     prop.EmitTrue,
\t\t\tCallback: nil,
\t\t}
\t}
'''
    new = '''\t// LunaBox patch: always export DBus menu path. KDE/Plasma reads this
\t// StatusNotifierItem property before it can render the com.canonical.dbusmenu
\t// tree; omitting it makes right-click look like a dead tray icon.
\tprops["Menu"] = &prop.Prop{
\t\tValue:    dbus.ObjectPath(menuPath),
\t\tWritable: false,
\t\tEmit:     prop.EmitTrue,
\t\tCallback: nil,
\t}
'''
    if old not in tray:
        raise SystemExit("failed to patch Wails systemtray_linux.go: Menu property block not found")
    tray = tray.replace(old, new, 1)

if "LunaBox patch: Linux tray menu is opened by the tray host" not in tray:
    old = '''func (s *linuxSystemTray) openMenu() {
\t// FIXME: Emit com.canonical to open?
\tglobalApplication.info("systray error: openMenu not implemented on Linux")
}
'''
    new = '''func (s *linuxSystemTray) openMenu() {
\t// LunaBox patch: Linux tray menu is opened by the tray host through
\t// StatusNotifierItem.Menu and com.canonical.dbusmenu. There is no app-side
\t// popup implementation in Wails v3 beta.5.
}
'''
    if old not in tray:
        raise SystemExit("failed to patch Wails systemtray_linux.go: openMenu block not found")
    tray = tray.replace(old, new, 1)

tray_path.write_text(tray)
PY

echo "Patched Wails ${module_version} Linux system tray support."
