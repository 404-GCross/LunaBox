// 构建一份开启了 WebView2 远程调试端口的原版 LunaBox，作为对照的“参考”。不改动仓库里的任何文件：
// 复制 apps/desktop 与 packages/updater 到临时目录，在副本的 main.go 里给 application.Options 加
// Windows.AdditionalBrowserArgs（Wails 会覆盖 WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS 环境变量，只能这样开），
// 前端用 vite 构建到副本里，再 go build。
//
//   node tools/parity/build-reference.mjs [--out <目录>] [--port 9333] [--skip-frontend]
//
// 默认输出 %TEMP%/lunabox-reference/lunabox-reference.exe。需要 apps/desktop/frontend 已 pnpm install。
//
// 前端构建时去掉了 VSCODE_CWD：UnoCSS 的 presetIcons 检测到它（从 VS Code 终端运行）就不加载
// @iconify-json，产物里的 i-mdi-* 图标全部为空，参考截图会缺图标。
import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const args = Object.fromEntries(process.argv.slice(2).flatMap((a, i, all) => a.startsWith("--") ? [[a.slice(2), all[i + 1]?.startsWith("--") ? "1" : all[i + 1] ?? "1"]] : []));
const repo = join(dirname(fileURLToPath(import.meta.url)), "../../../..");
const out = args.out ?? join(tmpdir(), "lunabox-reference");
const port = Number(args.port ?? 9333);
const desktop = join(out, "apps/desktop");

const run = (cmd, argv, opts) => execFileSync(cmd, argv, { stdio: "inherit", ...opts });

// 1. 复制源码（go.mod 里 replace 的 ../../packages/updater 保持相对位置）。不复制前端、bin、
//    build/bin（打包产物，几个 GB）和 build 下的数据库；--skip-frontend 时保留上次构建的 frontend/dist。
if (!args["skip-frontend"])
  rmSync(out, { recursive: true, force: true });
mkdirSync(join(out, "packages"), { recursive: true });
cpSync(join(repo, "packages/updater"), join(out, "packages/updater"), { recursive: true });
cpSync(join(repo, "apps/desktop"), desktop, {
  recursive: true,
  filter: src => !/[\\/]apps[\\/]desktop[\\/](frontend|bin|build[\\/]bin|build[\\/][^\\/]*\.db(\.wal)?)([\\/]|$)/.test(src),
});

// 2. 打开调试端口。
const mainGo = join(desktop, "main.go");
const src = readFileSync(mainGo, "utf8");
const anchor = "application.New(application.Options{";
if (!src.includes(anchor))
  throw new Error(`main.go 里找不到 ${anchor}`);
writeFileSync(mainGo, src.replace(anchor, `${anchor}\n\t\tWindows: application.WindowsOptions{AdditionalBrowserArgs: []string{"--remote-debugging-port=${port}"}},`));

// 3. 前端。
const dist = join(desktop, "frontend/dist");
if (!args["skip-frontend"] || !existsSync(dist)) {
  const env = { ...process.env };
  delete env.VSCODE_CWD;
  const frontend = join(repo, "apps/desktop/frontend");
  const vite = join(frontend, "node_modules/vite/bin/vite.js");
  if (!existsSync(vite))
    throw new Error("找不到 vite，先在 apps/desktop/frontend 下运行 pnpm install");
  run(process.execPath, [vite, "build", "--outDir", dist, "--emptyOutDir"], { cwd: frontend, env });
}

// 4. 后端（DuckDB 需要 cgo）。
const exe = join(out, "lunabox-reference.exe");
run("go", ["build", "-o", exe, "."], { cwd: desktop, env: { ...process.env, CGO_ENABLED: "1" } });
console.log(`\nbuilt ${exe}（调试端口 ${port}）`);
console.log(`运行：LUNABOX_PORTABLE_ROOT=<prepare-data 的 ref 目录> ${exe}`);
