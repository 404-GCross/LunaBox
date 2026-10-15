// 从 LunaBox 数据目录复制两份对照用的数据：<out>/ref 给原版，<out>/mygo 给原型。
// 两份是因为原版运行时 DuckDB 独占数据库文件，原型启动时要复制它会失败。
// 配置里与云同步、备份、凭据有关的项被清空，避免对照时触发同步或上传。
//
//   node tools/parity/prepare-data.mjs [--from <数据目录>] [--out <目录>]
//
// 默认从 %APPDATA%/LunaBox 复制到 %TEMP%/lunabox-parity。
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import process from "node:process";

const args = Object.fromEntries(process.argv.slice(2).flatMap((a, i, all) => a.startsWith("--") ? [[a.slice(2), all[i + 1]]] : []));
const from = args.from ?? join(process.env.APPDATA ?? "", "LunaBox");
const out = args.out ?? join(tmpdir(), "lunabox-parity");

if (!existsSync(join(from, "lunabox.db")))
  throw new Error(`${from} 里没有 lunabox.db，用 --from 指定数据目录`);

// 与同步、备份、凭据有关的键：布尔值改为 false，字符串清空。
const SENSITIVE = /cloud|s3|backup|sync|token|secret|password|access_key|api_key|webdav|onedrive/i;

function sanitize(dir) {
  const p = join(dir, "appconf.json");
  if (!existsSync(p))
    return [];
  const cfg = JSON.parse(readFileSync(p, "utf8"));
  const cleared = [];
  for (const [k, v] of Object.entries(cfg)) {
    if (!SENSITIVE.test(k))
      continue;
    if (typeof v === "boolean")
      cfg[k] = false;
    else if (typeof v === "string")
      cfg[k] = "";
    else
      continue;
    cleared.push(k);
  }
  cfg.close_to_tray = false;
  writeFileSync(p, JSON.stringify(cfg, null, 2));
  return cleared;
}

function copy(name, entries) {
  const dir = join(out, name);
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });
  for (const e of entries) {
    if (existsSync(join(from, e)))
      cpSync(join(from, e), join(dir, e), { recursive: true });
  }
  return { dir, cleared: sanitize(dir) };
}

const base = ["appconf.json", "lunabox.db", "lunabox.db.wal", "covers", "backgrounds"];
const ref = copy("ref", [...base, "avatars", "templates"]);
const mygo = copy("mygo", base);
console.log(`ref:  ${ref.dir}`);
console.log(`mygo: ${mygo.dir}`);
console.log(`cleared ${ref.cleared.length} config keys: ${ref.cleared.join(", ")}`);
