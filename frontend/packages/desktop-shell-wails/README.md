# Desktop Shell Wails

Wails v3 adapter for `@lunabox/desktop-shell-react`.

```tsx
import { DesktopShellProvider } from "@lunabox/desktop-shell-react";
import { createWailsDesktopAdapter } from "@lunabox/desktop-shell-wails";

const adapter = createWailsDesktopAdapter();

function App() {
  return (
    <DesktopShellProvider adapter={adapter}>
      <Application />
    </DesktopShellProvider>
  );
}
```

The adapter maps Wails platform names, window commands, maximized state, and
`--wails-draggable` regions to the shared React contract.
