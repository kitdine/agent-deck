// 把 vite 的构建产物折叠成一个可以直接双击打开的 HTML。
//
// 存在的理由：看一眼设计不该需要 npm install 拉 116 MB 再起 dev server。
// 产物是本地查看用的，进 .gitignore —— 设计真相仍然是 src/ 下的源码，
// 因为一个压缩成一行的 blob 没法 diff，也就没法评审。
//
// 两个约束决定了实现方式：
//   1. Chrome 用 CORS 挡掉 file:// 下的 type="module"，所以 vite 必须打成
//      IIFE（见 vite.config.mjs 的 SINGLE_FILE 分支），这里再去掉 type 属性。
//      去掉 type 的副作用是脚本不再延迟执行：vite 把它放在 <head>，而经典
//      脚本会在 <div id="root"> 存在之前就跑，createRoot(null) 抛错、页面全白。
//      所以内联后必须把它挪到 </body> 之前；
//   2. 图片必须内联成 data URI。src/ 下被 import 的资源由 assetsInlineLimit
//      处理，但 public/ 下的文件不走资源管线，是被 JSX 以绝对路径 "/x.png"
//      直接引用的 —— 那在 file:// 下指向文件系统根目录。这里按文件名把它们
//      替换成 data URI，然后断言一个都没剩。

import { readFileSync, writeFileSync, existsSync, readdirSync } from "node:fs";
import { dirname, resolve, join } from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const distDir = join(root, "dist", "client");
const entry = join(distDir, "index.html");
const out = join(root, "preview.html");

if (!existsSync(entry)) {
  console.error(`missing ${entry} — run \`npm run build:single\`, not this script alone`);
  process.exit(1);
}

const read = (href) => readFileSync(join(distDir, href.replace(/^\.?\//, "")), "utf8");

// 一个恰好出现在 JS 里的 </script> 会提前关闭标签。它只可能出现在字符串字面量
// 中，而字符串里 <\/script> 与之等价，所以这样替换不会改变语义。
const guard = (js) => js.replaceAll("</script", "<\\/script");

const SCRIPT = /<script\b[^>]*\bsrc="([^"]+)"[^>]*><\/script>/g;
const STYLE = /<link\b[^>]*\brel="stylesheet"[^>]*\bhref="([^"]+)"[^>]*>/g;

const html = readFileSync(entry, "utf8");

// 自包含检查跑在内联之前：内联之后的文档里全是压缩 JS，它的字符串拼接
// （src="'+n+'"）会被当成外部引用。这里问的是原始产物有没有我没处理的引用。
const leftovers = [...html.replace(SCRIPT, "").replace(STYLE, "").matchAll(/(?:src|href)="(?!data:|#)([^"]+)"/g)]
  .map((m) => m[1]);
if (leftovers.length > 0) {
  console.error(`not self-contained, still references: ${leftovers.join(", ")}`);
  process.exit(1);
}

// 1 —— 取出 JS，把 CSS 折进 <style>。script 标签整个删掉，稍后重新插到 body 末尾。
let bundle = "";
let doc = html
  .replace(SCRIPT, (_m, href) => { bundle = guard(read(href)); return ""; })
  .replace(STYLE, (_m, href) => `<style>\n${read(href)}\n</style>`);

if (!bundle) {
  console.error("no <script src> found in the built index.html");
  process.exit(1);
}

// 2 —— public/ 下的文件不走 vite 资源管线，是被 JSX 以绝对路径 "/x.png" 引用的，
// 那在 file:// 下指向文件系统根目录。按文件名换成 data URI，bundle 和文档都要换。
const MIME = { ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".svg": "image/svg+xml",
               ".gif": "image/gif", ".webp": "image/webp", ".ico": "image/x-icon" };
const publicDir = join(root, "public");
const assets = existsSync(publicDir) ? readdirSync(publicDir) : [];
const unresolved = [];
for (const name of assets) {
  const mime = MIME[name.slice(name.lastIndexOf(".")).toLowerCase()];
  if (!mime) { unresolved.push(name); continue; }
  const uri = `data:${mime};base64,${readFileSync(join(publicDir, name)).toString("base64")}`;
  for (const q of ['"', "'"]) {
    bundle = bundle.replaceAll(`${q}/${name}${q}`, () => `${q}${uri}${q}`);
    doc = doc.replaceAll(`${q}/${name}${q}`, () => `${q}${uri}${q}`);
  }
}

// 3 —— 校验 JS 本身能解析。做完这一步，bundle 就是最终形态，之后只允许被
// 原样搬运。
try {
  new vm.Script(bundle);
} catch (err) {
  console.error(`the bundle does not parse: ${err.message}`);
  process.exit(1);
}

// 4 —— 插到 </body> 之前。替换串必须是函数：String.replace 会把替换串里的
// $` / $' / $& 当成引用匹配上下文的模式，而压缩后的 bundle 全是 $ 开头的变量名，
// 传字符串会把文档片段注进 JS 中间，产出一个只表现为白屏的语法错误页面。
if (!doc.includes("</body>")) {
  console.error("built index.html has no </body> to place the bundle before");
  process.exit(1);
}
const inlined = doc.replace("</body>", () => `<script>\n${bundle}\n</script>\n</body>`);

// 5 —— 三条断言守的是同一类故障：打开就白屏，而 file:// 下未必留下控制台痕迹。
// 一律以 bundle 变量为准，不能在文档里找 "<script>"，压缩后的 bundle 自己就含这串。
const stillAbsolute = assets.filter((n) => inlined.includes(`/${n}`));
if (stillAbsolute.length > 0 || unresolved.length > 0) {
  console.error(`public asset not inlined: ${[...stillAbsolute, ...unresolved].join(", ")}`);
  process.exit(1);
}
const at = inlined.indexOf(bundle);
if (at < 0) {
  console.error("the bundle was altered while being inlined — it is not present verbatim");
  process.exit(1);
}
if (at < inlined.indexOf('id="root"')) {
  console.error("the bundle runs before #root exists — the page would render blank");
  process.exit(1);
}

writeFileSync(out, inlined);
const kb = (Buffer.byteLength(inlined) / 1024).toFixed(0);
console.log(`preview.html  ${kb} KB`);
console.log(`open file://${out}?surface=cli   (also ?surface=widgets, ?surface=states, ?state=partial)`);
