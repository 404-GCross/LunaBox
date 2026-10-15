// 从原版前端依赖的 @iconify-json/mdi 导出 i-mdi-* 图标到 ui/assets/icons/<name>.svg（assets.Icon 读取）。
//
//   node tools/icons/export-icons.mjs clock-outline bookmark-outline   导出指定图标
//   node tools/icons/export-icons.mjs --check                          列出 Go 代码引用了、但还没导出的图标
//
// 需要先在 apps/desktop/frontend 下安装依赖（pnpm install）。
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const outDir = join(root, "ui/assets/icons");
const iconsJSON = join(root, "../desktop/frontend/node_modules/@iconify-json/mdi/icons.json");

function loadSet() {
  if (!existsSync(iconsJSON))
    throw new Error(`找不到 ${iconsJSON}，先在 apps/desktop/frontend 下运行 pnpm install`);
  return JSON.parse(readFileSync(iconsJSON, "utf8"));
}

function svg(set, name) {
  let icon = set.icons[name];
  const alias = set.aliases?.[name];
  if (!icon && alias)
    icon = set.icons[alias.parent];
  if (!icon)
    throw new Error(`@iconify-json/mdi 里没有 ${name}`);
  const w = icon.width ?? set.width ?? 24;
  const h = icon.height ?? set.height ?? 24;
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}">${icon.body}</svg>\n`;
}

// referencedIcons 粗略收集 Go 代码里传给 Icon(c, "...")、Icon: "..." 等处的图标名。
function referencedIcons() {
  const names = new Set();
  const walk = (dir) => {
    for (const e of readdirSync(dir, { withFileTypes: true })) {
      const p = join(dir, e.name);
      if (e.isDirectory())
        walk(p);
      else if (e.name.endsWith(".go") && !e.name.endsWith("_test.go")) {
        const src = readFileSync(p, "utf8");
        for (const m of src.matchAll(/(?:Icon\(c, |IconButton\(c, t, "[\w-]+", |Icon: |icon: |return )"([a-z][a-z0-9-]+)"/g))
          names.add(m[1]);
      }
    }
  };
  walk(join(root, "ui"));
  return [...names].sort();
}

const args = process.argv.slice(2);
if (args[0] === "--check") {
  const missing = referencedIcons().filter(n => !existsSync(join(outDir, `${n}.svg`)));
  if (missing.length) {
    console.log(`未导出的图标（可能有误报）：${missing.join(" ")}`);
    process.exitCode = 1;
  }
  else {
    console.log("引用的图标都已导出");
  }
}
else if (args.length) {
  const set = loadSet();
  for (const name of args) {
    writeFileSync(join(outDir, `${name}.svg`), svg(set, name));
    console.log(`ui/assets/icons/${name}.svg`);
  }
}
else {
  console.log("用法：node tools/icons/export-icons.mjs <name...> | --check");
}
