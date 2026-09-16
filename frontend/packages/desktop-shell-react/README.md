# Desktop Shell React

React infrastructure for desktop WebView applications. The package provides
window adapters, desktop insets, drag regions, named overlay hosts, and
semantic overlay layers without imposing an application theme.

The package is private while its API is validated in LunaBox.

## Scoped Modals

Mount `DesktopModal` while open, or inside a `Transition` that unmounts on exit.
Provide `aria-labelledby` or `aria-label`. Its `backdrop` prop places the backdrop
outside the focusable panel. The default host is `content`; the titlebar must be
outside that host's parent. The component omits document-wide `aria-modal`
because window controls remain available.

Headless UI portals inside the modal target a local container. Use `modal={false}`
on nested menus and selects so their global inert handling keeps window chrome
available. `LayerPortal` alone provides placement, not dismissal or focus handling.
Third-party dialogs require adapters using their portal and modality extension
points. CSS offsets alone cannot constrain document-level event handling.

## API and Distribution

The Provider accepts `adapter`, `insets`, and `children`. Host registration stays
internal. Applications use `OverlayHost`, `LayerPortal`, `DesktopModal`,
`useDesktopInsets`, and `useDesktopWindow`. Insets use CSS pixels.

The core and Wails adapter have no React dependency. JSX, portals, focus integration,
and hooks live here. Vue wrappers can reuse the core; no Vue wrapper is included.

Run `pnpm build:desktop` from the frontend directory. Package exports point to
ESM JavaScript and declarations in `dist`. `pnpm build` and `pnpm dev` build the
packages first. Rebuild after editing package source. Packages remain private;
publication still requires release metadata, license review and supported-version
tests. Layer names currently represent fixed bands, not automatic dynamic stacking.
Window polling is internal; a runtime subscription contract can replace it later.

## Browser Regression

With Vite running, execute `node tests/desktopShell.mjs` from the frontend directory.
The runner needs Playwright and installed Chrome. `PLAYWRIGHT_MODULE` can reference
an external Playwright ESM entry. `TEST_BASE_URL` defaults to
`http://127.0.0.1:9245`. Tests cover titlebar interaction, dismissal, nested selects
and drawers, focus restoration, resizing and zoom at desktop and narrow widths.

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
