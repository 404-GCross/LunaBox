# Desktop Shell React

React infrastructure for desktop WebView applications. The package provides
window adapters, desktop insets, drag regions, named overlay hosts, and
semantic overlay layers without imposing an application theme.

The package is private while its API is validated in LunaBox.

## Setup

```tsx
import {
  DesktopShellProvider,
  OverlayHost,
} from "@lunabox/desktop-shell-react";

function App() {
  return (
    <DesktopShellProvider adapter={adapter} insets={{ top: 28 }}>
      <Application />
      <OverlayHost name="window" />
    </DesktopShellProvider>
  );
}
```

Place additional hosts inside constrained application regions when an overlay
must preserve native window chrome:

```tsx
<main className="relative">
  <Page />
  <OverlayHost name="content" />
</main>
```

Render an overlay through a semantic layer instead of assigning a numeric
`z-index`:

```tsx
<LayerPortal host="content" layer="modal" pointerEvents="auto">
  <Dialog />
</LayerPortal>
```

Available layers are `content`, `modal`, `floating`, `dropdown`, `tooltip`,
`toast`, and `critical`. Insets and layer values are also exposed as CSS custom
properties prefixed with `--desktop-inset-` and `--desktop-layer-`.

## Adapter contract

Runtime packages implement `DesktopWindowAdapter`. Application components use
`useDesktopWindow`, `WindowDragRegion`, and `WindowNoDragRegion` without
importing Electron, Tauri, or Wails APIs.
