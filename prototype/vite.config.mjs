import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// SINGLE_FILE=1 时构建一个能从 file:// 直接打开的产物：Chrome 用 CORS 挡掉
// file:// 下的 type="module"，所以打成 IIFE；图片必须内联，否则单文件里的相对
// 路径指向不存在的目录。tools/build-single-file.mjs 随后把 JS 与 CSS 折进 HTML。
const singleFile = process.env.SINGLE_FILE === "1";

export default defineConfig({
  build: {
    outDir: "dist/client",
    ...(singleFile
      ? {
          cssCodeSplit: false,
          assetsInlineLimit: Number.MAX_SAFE_INTEGER,
          rollupOptions: { output: { format: "iife", inlineDynamicImports: true } },
        }
      : {}),
  },
  optimizeDeps: { include: ["react", "react-dom/client"] },
  server: {
    host: "0.0.0.0",
    allowedHosts: ["terminal.local"],
    warmup: { clientFiles: ["./src/main.jsx"] },
  },
  plugins: [react(), {
    name: "exclude-widget-test-probe",
    apply: "build",
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        if (output.type !== "chunk") continue;
        const testModule = Object.keys(output.modules).find((id) =>
          id.includes("/node_modules/axe-core/") || id.endsWith("/src/widget-contrast-probe.js"),
        );
        if (testModule) this.error(`Test-only module entered the production bundle: ${testModule}`);
      }
    },
  }],
});
