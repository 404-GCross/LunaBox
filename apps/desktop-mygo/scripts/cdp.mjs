// 用无头 Edge 打开 Wails server 模式下的原版前端，截图并执行 JS，作为一比一还原的参照。
//
//   go build -tags server -o lunabox-server.exe .   (在 apps/desktop 下)
//   WAILS_SERVER_PORT=8765 LUNABOX_PORTABLE_ROOT=<数据副本> lunabox-server.exe
//   node scripts/cdp.mjs --url http://127.0.0.1:8765/library --out ref.png [--eval dump.js] [--dark]
//
// 选项：--width/--height（DIP，默认 1280x800）、--scale（默认 1.5）、--wait（毫秒，默认 3000）、
// --hover x,y（截图前把指针移到该位置）、--click x,y、--route /path（前端路由跳转）。
// --attach <port> 连接已开启远程调试的 WebView2（例如加了 --remote-debugging-port 的 LunaBox），
// 此时不启动 Edge，也不导航；加 --emulate 才覆盖视口尺寸。
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, a, i, all) => {
    if (a.startsWith("--")) acc.push([a.slice(2), all[i + 1]?.startsWith("--") || all[i + 1] === undefined ? "1" : all[i + 1]]);
    return acc;
  }, []),
);
const width = Number(args.width ?? 1280);
const height = Number(args.height ?? 800);
const scale = Number(args.scale ?? 1.5);
const attach = args.attach ? Number(args.attach) : 0;
const port = attach || 9400 + Math.floor(Math.random() * 400);
const edge = process.env.EDGE ?? "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe";

const browser = attach ? null : spawn(edge, [
  "--headless=new",
  `--remote-debugging-port=${port}`,
  `--user-data-dir=${mkdtempSync(join(tmpdir(), "cdp-"))}`,
  "--hide-scrollbars=false",
  "about:blank",
], { stdio: "ignore" });

const sleep = ms => new Promise(r => setTimeout(r, ms));

async function target() {
  for (let i = 0; i < 50; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
      const page = list.find(t => t.type === "page");
      if (page) return page.webSocketDebuggerUrl;
    }
    catch {}
    await sleep(200);
  }
  throw new Error("edge did not start");
}

const ws = new WebSocket(await target());
await new Promise(r => ws.addEventListener("open", r, { once: true }));
let seq = 0;
const pending = new Map();
ws.addEventListener("message", (e) => {
  const msg = JSON.parse(e.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    msg.error ? reject(new Error(JSON.stringify(msg.error))) : resolve(msg.result);
  }
});
const send = (method, params = {}) => new Promise((resolve, reject) => {
  const id = ++seq;
  pending.set(id, { resolve, reject });
  ws.send(JSON.stringify({ id, method, params }));
});

try {
  if (!attach || args.emulate)
    await send("Emulation.setDeviceMetricsOverride", { width, height, deviceScaleFactor: scale, mobile: false });
  await send("Page.enable");
  if (!attach) {
    await send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: args.dark ? "dark" : "light" }] });
    await send("Page.navigate", { url: args.url });
    await sleep(Number(args.wait ?? 3000));
  }
  if (args.route) {
    // server 模式没有 SPA 回退，先打开首页再让前端路由跳转。
    await send("Runtime.evaluate", {
      expression: `history.pushState({}, "", ${JSON.stringify(args.route)}); dispatchEvent(new PopStateEvent("popstate"));`,
    });
    await sleep(Number(args.wait ?? 3000));
  }

  const point = s => s.split(",").map(Number);
  if (args.click) {
    const [x, y] = point(args.click);
    for (const type of ["mouseMoved", "mousePressed", "mouseReleased"])
      await send("Input.dispatchMouseEvent", { type, x, y, button: "left", clickCount: 1 });
    await sleep(800);
  }
  if (args.hover) {
    const [x, y] = point(args.hover);
    await send("Input.dispatchMouseEvent", { type: "mouseMoved", x, y });
    await sleep(600);
  }
  if (args.eval) {
    const expression = readFileSync(args.eval, "utf8");
    const res = await send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
    const out = res.result?.value ?? res.exceptionDetails;
    const text = typeof out === "string" ? out : JSON.stringify(out, null, 2);
    if (args["eval-out"]) writeFileSync(args["eval-out"], text);
    else console.log(text);
  }
  if (args.out) {
    const shot = await send("Page.captureScreenshot", { format: "png" });
    writeFileSync(args.out, Buffer.from(shot.data, "base64"));
    console.log(`saved ${args.out}`);
  }
}
finally {
  ws.close();
  browser?.kill();
}
