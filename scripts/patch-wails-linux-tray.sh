#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

if [[ "$(uname -s)" != "Linux" ]]; then
    exit 0
fi

check_tool() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "ERROR: $1 was not found in PATH." >&2
        exit 1
    }
}

check_tool go
check_tool python3

module_version="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)"
go mod download github.com/wailsapp/wails/v3

mod_cache="$(go env GOMODCACHE)"
module_dir="$mod_cache/github.com/wailsapp/wails/v3@$module_version"
systemtray_go="$module_dir/pkg/application/systemtray.go"
linux_go="$module_dir/pkg/application/systemtray_linux.go"

for path in "$systemtray_go" "$linux_go"; do
    if [[ ! -f "$path" ]]; then
        echo "ERROR: Wails source file not found: $path" >&2
        exit 1
    fi
done

chmod u+w "$systemtray_go" "$linux_go"

python3 - "$systemtray_go" "$linux_go" <<'PY'
from pathlib import Path
import sys

systemtray_go = Path(sys.argv[1])
linux_go = Path(sys.argv[2])


def replace_once(path: Path, old: str, new: str) -> bool:
    text = path.read_text()
    if new in text:
        return False
    if old not in text:
        raise SystemExit(f"ERROR: expected source block not found in {path}")
    path.write_text(text.replace(old, new, 1))
    return True


changed = False

changed |= replace_once(
    systemtray_go,
    '''\tif s.rightClickHandler == nil && hasMenu {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
    '''\tif s.rightClickHandler == nil && hasMenu && runtime.GOOS != "linux" {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
)

changed |= replace_once(
    linux_go,
    '''\timpl := &linuxSystemTray{
\t\tparent:         s,
\t\tid:             s.id,
\t\tlabel:          label,
\t\ticon:           s.icon,
\t\tmenu:           s.menu,
\t\ticonPosition:   s.iconPosition,
\t\tisTemplateIcon: s.isTemplateIcon,
\t\tquitChan:       make(chan struct{}),
\t}
''',
    '''\timpl := &linuxSystemTray{
\t\tparent:         s,
\t\tid:             s.id,
\t\tlabel:          label,
\t\ttooltip:        s.tooltip,
\t\ticon:           s.icon,
\t\tmenu:           s.menu,
\t\ticonPosition:   s.iconPosition,
\t\tisTemplateIcon: s.isTemplateIcon,
\t\tquitChan:       make(chan struct{}),
\t}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) setTooltip(_ string) {
\t// TBD
}
''',
    '''func (s *linuxSystemTray) setTooltip(tooltipText string) {
\ts.tooltip = tooltipText
\tif tooltipText == "" {
\t\ttooltipText = s.label
\t}
\tif s.props == nil {
\t\treturn
\t}
\tif err := s.props.Set("org.kde.StatusNotifierItem", "ToolTip", dbus.MakeVariant(tooltip{V2: tooltipText})); err != nil {
\t\tglobalApplication.error("systray error: failed to set ToolTip prop: %w", err)
\t}
}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) openMenu() {
\t// FIXME: Emit com.canonical to open?
\tglobalApplication.info("systray error: openMenu not implemented on Linux")
}
''',
    '''func (s *linuxSystemTray) openMenu() {
\t// Linux StatusNotifier hosts open the exported dbusmenu themselves.
}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) setLabel(label string) {
\ts.label = label

\tif err := s.props.Set("org.kde.StatusNotifierItem", "Title", dbus.MakeVariant(label)); err != nil {
\t\tglobalApplication.error("systray error: failed to set Title prop: %w", err)
\t\treturn
\t}

\tif s.conn == nil {
\t\treturn
\t}

\tif err := notifier.Emit(s.conn, &notifier.StatusNotifierItem_NewTitleSignal{
\t\tPath: itemPath,
\t\tBody: &notifier.StatusNotifierItem_NewTitleSignalBody{},
\t}); err != nil {
\t\tglobalApplication.error("systray error: failed to emit new title signal: %w", err)
\t\treturn
\t}

}
''',
    '''func (s *linuxSystemTray) setLabel(label string) {
\ts.label = label
\tif s.props == nil {
\t\treturn
\t}

\tif err := s.props.Set("org.kde.StatusNotifierItem", "Title", dbus.MakeVariant(label)); err != nil {
\t\tglobalApplication.error("systray error: failed to set Title prop: %w", err)
\t\treturn
\t}
\tif s.tooltip == "" {
\t\tif err := s.props.Set("org.kde.StatusNotifierItem", "ToolTip", dbus.MakeVariant(tooltip{V2: label})); err != nil {
\t\t\tglobalApplication.error("systray error: failed to set ToolTip prop: %w", err)
\t\t}
\t}

\tif s.conn == nil {
\t\treturn
\t}

\tif err := notifier.Emit(s.conn, &notifier.StatusNotifierItem_NewTitleSignal{
\t\tPath: itemPath,
\t\tBody: &notifier.StatusNotifierItem_NewTitleSignalBody{},
\t}); err != nil {
\t\tglobalApplication.error("systray error: failed to emit new title signal: %w", err)
\t\treturn
\t}

}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) createPropSpec() map[string]map[string]*prop.Prop {
\tprops := map[string]*prop.Prop{
''',
    '''func (s *linuxSystemTray) createPropSpec() map[string]map[string]*prop.Prop {
\ttooltipText := s.tooltip
\tif tooltipText == "" {
\t\ttooltipText = s.label
\t}

\tprops := map[string]*prop.Prop{
''',
)

changed |= replace_once(
    linux_go,
    '''\t\t"ToolTip": {
\t\t\tValue:    tooltip{V2: s.label},
\t\t\tWritable: true,
\t\t\tEmit:     prop.EmitTrue,
\t\t\tCallback: nil,
\t\t},
''',
    '''\t\t"ToolTip": {
\t\t\tValue:    tooltip{V2: tooltipText},
\t\t\tWritable: true,
\t\t\tEmit:     prop.EmitTrue,
\t\t\tCallback: nil,
\t\t},
''',
)

changed |= replace_once(
    linux_go,
    '''\tif s.menu != nil {
\t\tprops["Menu"] = &prop.Prop{
\t\t\tValue:    dbus.ObjectPath(menuPath),
\t\t\tWritable: true,
\t\t\tEmit:     prop.EmitTrue,
\t\t\tCallback: nil,
\t\t}
\t}
''',
    '''\tprops["Menu"] = &prop.Prop{
\t\tValue:    dbus.ObjectPath(menuPath),
\t\tWritable: false,
\t\tEmit:     prop.EmitTrue,
\t\tCallback: nil,
\t}
''',
)

changed |= replace_once(
    linux_go,
    '''\tcase "opened":
\t\tif s.parent.clickHandler != nil {
\t\t\ts.parent.clickHandler()
\t\t}
\t\tif s.parent.onMenuOpen != nil {
\t\t\ts.parent.onMenuOpen()
\t\t}
''',
    '''\tcase "opened":
\t\tif s.parent.onMenuOpen != nil {
\t\t\ts.parent.onMenuOpen()
\t\t}
''',
)

if changed:
    print("patched")
else:
    print("already patched")
PY

echo "Wails Linux tray patch applied for $module_version"
