import {
  DesktopShellProvider,
  OverlayHost,
} from "@lunabox/desktop-shell-react";
import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import { BetterDrawer } from "../src/components/ui/better/BetterDrawer";
import { BetterSelect } from "../src/components/ui/better/BetterSelect";
import "@unocss/reset/tailwind.css";
import "virtual:uno.css";

const adapter = {
  getPlatform: async () => "windows" as const,
  getWindowState: async () => ({ isMaximized: false }),
  minimize: async () => {},
  toggleMaximize: async () => {},
  close: async () => {},
};

export function Fixture() {
  const [open, setOpen] = useState(false);
  const [nested, setNested] = useState(false);
  const [clicks, setClicks] = useState(0);
  const [height, setHeight] = useState(28);
  const [value, setValue] = useState("one");
  return (
    <DesktopShellProvider adapter={adapter} insets={{ top: height }}>
      <div
        style={{ height: "100vh", display: "flex", flexDirection: "column" }}
      >
        <header
          data-testid="chrome"
          style={{
            height,
            flexShrink: 0,
            background: "#ccc",
            display: "flex",
            gap: 20,
          }}
        >
          <button
            type="button"
            data-testid="window-control"
            onClick={() => setClicks(clicks + 1)}
          >
            Window control
            {" "}
            {clicks}
          </button>
          <button
            type="button"
            data-testid="height"
            onClick={() => setHeight(52)}
          >
            Resize titlebar
          </button>
        </header>
        <div style={{ position: "relative", flex: 1 }}>
          <main
            data-testid="content"
            style={{ height: "100%", padding: 30, background: "#eef" }}
          >
            <button
              type="button"
              data-testid="open"
              onClick={() => setOpen(true)}
            >
              Open drawer
            </button>
            <BetterDrawer
              isOpen={open}
              onOpenChange={setOpen}
              title="Test drawer"
            >
              <input aria-label="Test input" />
              <BetterSelect
                value={value}
                onChange={setValue}
                options={[
                  { value: "one", label: "One" },
                  { value: "two", label: "Two" },
                ]}
              />
              <button
                type="button"
                data-testid="nested"
                onClick={() => setNested(true)}
              >
                Nested drawer
              </button>
              <BetterDrawer
                isOpen={nested}
                onOpenChange={setNested}
                title="Nested drawer"
              >
                <button type="button">Nested action</button>
              </BetterDrawer>
            </BetterDrawer>
          </main>
          <OverlayHost name="content" />
        </div>
        <OverlayHost name="window" />
      </div>
    </DesktopShellProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Fixture />
  </StrictMode>,
);
