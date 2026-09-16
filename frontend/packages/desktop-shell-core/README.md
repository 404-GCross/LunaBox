# Desktop Shell Core

Framework-independent TypeScript and DOM primitives with zero runtime dependencies.
Exports window adapter types, semantic layers, and `mountModalScope`.

```ts
import { mountModalScope } from "@lunabox/desktop-shell-core";

const dispose = mountModalScope({
  host: overlayHost,
  panel: dialogElement,
  onDismiss: closeDialog,
});
// Call dispose() when the dialog unmounts.
```

The host's parent defines the blocked content region. Put the titlebar outside
that parent. Place page content and the overlay host next to each other inside it.
The host needs explicit positioning and dimensions. The caller supplies the
backdrop, panel, accessible labels, and focus management.

Only a pointer interaction beginning and ending outside the panel inside the
host dismisses the top scoped modal. Escape is handled after nested controls
can consume it. Cleanup restores original `inert` and `aria-hidden` values.
Nested modal registrations share a stack per host.

Use this from Vue mounted/unmounted hooks, React effects, or plain JavaScript.
Framework components and lifecycle adapters remain separate.
