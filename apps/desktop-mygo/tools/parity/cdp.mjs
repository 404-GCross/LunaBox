// 通过 Chrome DevTools Protocol 操作开启了远程调试的原版 LunaBox（WebView2）：点击、悬停、按键、执行脚本、截图。
// 既是 compare.mjs 用的模块，也可以单独在命令行里用，参数按出现的顺序依次执行：
//
//   node tools/parity/cdp.mjs --port 9333 --click 'a[href="/library"]' --hover 5,500 --shot ref.png
//   node tools/parity/cdp.mjs --port 9333 --click 'button:has(.i-mdi-filter-variant)' \
//        --eval tools/parity/dump-styles.js --eval-out dump.txt --shot drawer.png --press Escape
//
// 步骤：--click / --hover <x,y 或 CSS 选择器>，--press <键名，如 Escape>，--eval <JS 文件>（结果打印，
// 或写到紧随其后的 --eval-out），--js <表达式>，--shot <png>，--wait <毫秒>。
// 选项：--port（默认 9333）、--width/--height（DIP，默认 1280x800）、--scale（默认 1.5），
// 视口会被覆盖成这个尺寸与缩放，和原型 -screenshot 的客户区一致。
import { readFileSync, writeFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

export const sleep = ms => new Promise(r => setTimeout(r, ms));

async function pageTarget(port) {
  let lastErr;
  for (let i = 0; i < 300; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
      // 启动时先有一个 /startup 窗口，初始化完成后关闭；要连的是主窗口。
      const pages = list.filter(t => t.type === "page");
      const starting = pages.some(t => new URL(t.url).pathname.startsWith("/startup"));
      const page = pages.find(t => !new URL(t.url).pathname.startsWith("/startup"));
      if (page && !starting)
        return page.webSocketDebuggerUrl;
    }
    catch (e) {
      lastErr = e;
    }
    await sleep(200);
  }
  throw new Error(`no page on debugging port ${port} (${lastErr?.message ?? "no targets"})；原版是否用 build-reference.mjs 构建并已启动？`);
}

const KEYS = {
  Escape: { code: "Escape", windowsVirtualKeyCode: 27 },
  Enter: { code: "Enter", windowsVirtualKeyCode: 13 },
  Tab: { code: "Tab", windowsVirtualKeyCode: 9 },
};

// connect 连接调试端口上的页面并覆盖视口。返回的会话方法都会在动作后等待界面稳定。
export async function connect({ port = 9333, width = 1280, height = 800, scale = 1.5 } = {}) {
  const ws = new WebSocket(await pageTarget(port));
  await new Promise((resolve, reject) => {
    ws.addEventListener("open", resolve, { once: true });
    ws.addEventListener("error", reject, { once: true });
  });
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
  ws.addEventListener("close", () => {
    for (const { reject } of pending.values())
      reject(new Error("debugging connection closed"));
    pending.clear();
  });
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++seq;
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error(`${method} timed out`));
    }, 30_000);
    pending.set(id, {
      resolve: (v) => { clearTimeout(timer); resolve(v); },
      reject: (e) => { clearTimeout(timer); reject(e); },
    });
    ws.send(JSON.stringify({ id, method, params }));
  });

  const evaluate = async (expression) => {
    const res = await send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
    if (res.exceptionDetails)
      throw new Error(`eval failed: ${res.exceptionDetails.exception?.description ?? res.exceptionDetails.text}`);
    return res.result?.value;
  };

  // point 把 "x,y" 或 CSS 选择器解析成视口坐标（选择器取元素中心）。
  const point = async (target) => {
    if (/^\s*[\d.]+\s*,\s*[\d.]+\s*$/.test(target))
      return target.split(",").map(Number);
    const r = await evaluate(`(() => {
      const el = document.querySelector(${JSON.stringify(target)});
      if (!el) return null;
      const r = el.getBoundingClientRect();
      return [r.x + r.width / 2, r.y + r.height / 2];
    })()`);
    if (!r)
      throw new Error(`selector matched nothing: ${target}`);
    return r;
  };

  await send("Emulation.setDeviceMetricsOverride", { width, height, deviceScaleFactor: scale, mobile: false });
  await send("Page.enable");

  return {
    send,
    evaluate,
    async click(target, settle = 800) {
      const [x, y] = await point(target);
      for (const type of ["mouseMoved", "mousePressed", "mouseReleased"])
        await send("Input.dispatchMouseEvent", { type, x, y, button: "left", clickCount: 1 });
      await sleep(settle);
    },
    async hover(target, settle = 600) {
      const [x, y] = await point(target);
      await send("Input.dispatchMouseEvent", { type: "mouseMoved", x, y });
      await sleep(settle);
    },
    async press(key, settle = 600) {
      const k = KEYS[key] ?? { code: key };
      for (const type of ["rawKeyDown", "keyUp"])
        await send("Input.dispatchKeyEvent", { type, key, ...k });
      await sleep(settle);
    },
    async screenshot(path) {
      const shot = await send("Page.captureScreenshot", { format: "png" });
      writeFileSync(path, Buffer.from(shot.data, "base64"));
    },
    close() {
      ws.close();
    },
  };
}

// 命令行：参数按顺序执行。
async function main() {
  const argv = process.argv.slice(2);
  const opts = {};
  const steps = [];
  for (let i = 0; i < argv.length; i++) {
    const name = argv[i].replace(/^--/, "");
    const value = argv[i + 1];
    if (["port", "width", "height", "scale"].includes(name)) {
      opts[name] = Number(value);
      i++;
    }
    else if (["click", "hover", "press", "eval", "eval-out", "js", "shot", "wait"].includes(name)) {
      steps.push([name, value]);
      i++;
    }
    else {
      throw new Error(`unknown argument ${argv[i]}`);
    }
  }
  const s = await connect(opts);
  try {
    for (let i = 0; i < steps.length; i++) {
      const [step, value] = steps[i];
      switch (step) {
        case "click": await s.click(value); break;
        case "hover": await s.hover(value); break;
        case "press": await s.press(value); break;
        case "wait": await sleep(Number(value)); break;
        case "shot":
          await s.screenshot(value);
          console.log(`saved ${value}`);
          break;
        case "js":
        case "eval": {
          const out = await s.evaluate(step === "js" ? value : readFileSync(value, "utf8"));
          const text = typeof out === "string" ? out : JSON.stringify(out, null, 2);
          if (steps[i + 1]?.[0] === "eval-out") {
            writeFileSync(steps[++i][1], text);
            console.log(`saved ${steps[i][1]}`);
          }
          else {
            console.log(text);
          }
          break;
        }
        case "eval-out": throw new Error("--eval-out must follow --eval or --js");
      }
    }
  }
  finally {
    s.close();
  }
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((e) => {
    console.error(e.message);
    process.exit(1);
  });
}
