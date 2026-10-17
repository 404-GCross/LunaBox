// 一次跑完 scenarios.mjs 里的对照：启动参考版截图，再让原型用同一份数据截图，逐区域做像素差异，
// 输出 report.md 和每个区域的并排图（原版 | 原型 | 差异×3）。流程说明见 docs/parity.md。
//
//   node tools/parity/compare.mjs [--only drawer,addmenu] [--attach] [--no-build]
//
// 选项：
//   --data <目录>       prepare-data.mjs 的输出（默认 %TEMP%/lunabox-parity）
//   --reference <exe>   build-reference.mjs 的输出（默认 %TEMP%/lunabox-reference/lunabox-reference.exe）
//   --attach            参考版已经在运行（调试端口 --port），不启动也不关闭它
//   --out <目录>        截图与报告（默认 <data>/report）
//   --scale 1.5         显示器缩放；原版视口按它模拟，必须与原型窗口所在显示器一致
//   --mygo-size 1295x807  原型窗口尺寸（DIP）。无边框窗口仍有约 11 物理像素的边框，这样客户区才是 1280x800
//   --threshold 24      亮度差超过它的像素计为不同
import { execFileSync, spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { connect, sleep } from "./cdp.mjs";
import { prelude, scenarios } from "./scenarios.mjs";

const args = Object.fromEntries(process.argv.slice(2).flatMap((a, i, all) => a.startsWith("--") ? [[a.slice(2), all[i + 1]?.startsWith("--") ? "1" : all[i + 1] ?? "1"]] : []));
const app = join(dirname(fileURLToPath(import.meta.url)), "../..");
const data = args.data ?? join(tmpdir(), "lunabox-parity");
const reference = args.reference ?? join(tmpdir(), "lunabox-reference", "lunabox-reference.exe");
const out = args.out ?? join(data, "report");
const port = Number(args.port ?? 9333);
const scale = Number(args.scale ?? 1.5);
const [mygoW, mygoH] = (args["mygo-size"] ?? "1295x807").split("x");
const threshold = args.threshold ?? "24";
const only = args.only?.split(",");
const list = scenarios.filter(s => !only || only.includes(s.name));

for (const dir of ["ref", "mygo"]) {
  if (!existsSync(join(data, dir, "lunabox.db")))
    throw new Error(`${join(data, dir)} 没有数据，先运行 node tools/parity/prepare-data.mjs`);
}
mkdirSync(out, { recursive: true });

// 原型与 pixdiff。
const bin = join(app, "build");
const mygo = join(bin, "desktop-mygo.exe");
const pixdiff = join(bin, "pixdiff.exe");
if (!args["no-build"]) {
  const env = { ...process.env, CGO_ENABLED: "1" };
  execFileSync("go", ["build", "-o", mygo, "."], { cwd: app, stdio: "inherit", env });
  execFileSync("go", ["build", "-o", pixdiff, "./tools/parity/pixdiff"], { cwd: app, stdio: "inherit", env });
}

async function runSteps(s, steps) {
  for (const [step, value] of steps) {
    if (step === "js")
      await s.evaluate(value);
    else
      await s[step](value);
  }
}

// 1. 参考版截图。
let proc;
if (!args.attach) {
  if (!existsSync(reference))
    throw new Error(`${reference} 不存在，先运行 node tools/parity/build-reference.mjs`);
  proc = spawn(reference, [], { env: { ...process.env, LUNABOX_PORTABLE_ROOT: join(data, "ref") }, stdio: "ignore" });
}
try {
  const s = await connect({ port, scale });
  await sleep(args.attach ? 0 : 4000); // 等首页数据和封面
  for (const sc of list) {
    await runSteps(s, prelude);
    await runSteps(s, sc.setup);
    await s.screenshot(join(out, `ref-${sc.name}.png`));
    await runSteps(s, sc.teardown);
    console.log(`ref  ${sc.name}`);
  }
  s.close();
}
finally {
  if (proc)
    spawnSync("taskkill", ["/PID", String(proc.pid), "/T", "/F"], { stdio: "ignore" });
}

// 2. 原型截图（-state 直接进入对应界面状态）。
for (const sc of list) {
  const shot = join(out, `mygo-${sc.name}.png`);
  const argv = ["-data-dir", join(data, "mygo"), "-width", mygoW, "-height", mygoH, "-screenshot", shot];
  if (sc.state)
    argv.push("-state", sc.state);
  // 原版每个场景从游戏库开始（prelude），没有写 route 的场景在原型里也从游戏库开始。
  argv.push("-route", sc.route ?? "/library");
  const r = spawnSync(mygo, argv, { timeout: 120_000, encoding: "utf8" });
  if (!existsSync(shot))
    throw new Error(`原型截图失败（${sc.name}）：${r.stderr || r.error}`);
  console.log(`mygo ${sc.name}`);
}

// 3. 逐区域比较。
const rows = [];
for (const sc of list) {
  for (const [region, rect] of Object.entries(sc.regions)) {
    const sbs = `sbs-${sc.name}-${region}.png`;
    const res = JSON.parse(execFileSync(pixdiff, [
      "-ref", join(out, `ref-${sc.name}.png`),
      "-mine", join(out, `mygo-${sc.name}.png`),
      "-rect", rect.join(","),
      "-scale", String(scale),
      "-threshold", threshold,
      "-sbs", join(out, sbs),
      "-zoom", rect[2] > 600 ? "0.5" : "1",
      "-json",
    ], { encoding: "utf8" }));
    rows.push({ ...res, scenario: sc.name, region, rect, sbs });
  }
}

const sizeNote = rows.find(r => r.refSize.join() !== r.mineSize.join());
const report = [
  `# 原版 / 原型对照（${new Date().toLocaleString()}）`,
  "",
  `缩放 ${scale}，亮度差阈值 ${threshold}。差异像素占比越低越好；整体偏移 1 像素和抗锯齿差异一般在 1% 以内，菜单、抽屉这类大块浮层会更高。`,
  sizeNote ? `\n> 截图尺寸不同：原版 ${sizeNote.refSize.join("x")}，原型 ${sizeNote.mineSize.join("x")}。差 1 像素是无边框窗口的边框取整，更多则检查 --scale 与 --mygo-size。\n` : "",
  "| 场景 | 区域 | 范围（DIP） | 差异像素 | 并排图 |",
  "| --- | --- | --- | --- | --- |",
  ...rows.map(r => `| ${r.scenario} | ${r.region} | ${r.rect.join(", ")} | ${r.percent.toFixed(2)}% | [${r.sbs}](${r.sbs}) |`),
  "",
].join("\n");
writeFileSync(join(out, "report.md"), report);
console.log(`\n${report}\n报告与截图：${out}`);
