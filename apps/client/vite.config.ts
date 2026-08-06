import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "VITE_");
  const edition = String(env.VITE_APP_EDITION || (mode === "open-source" ? "OPEN_SOURCE" : mode === "mini-program" ? "MINI_PROGRAM" : "COMMERCIAL"))
    .trim()
    .toUpperCase()
    .replace(/-/g, "_");

  return {
  define: {
    __LINGUAQUEST_COMMERCIAL_EDITION__: JSON.stringify(!["OPEN_SOURCE", "OPEN", "OSS"].includes(edition)),
    __LINGUAQUEST_APP_EDITION__: JSON.stringify(edition === "MINI_PROGRAM" || edition === "MINIPROGRAM" || edition === "MINI" ? "MINI_PROGRAM" : ["OPEN_SOURCE", "OPEN", "OSS"].includes(edition) ? "OPEN_SOURCE" : "COMMERCIAL")
  },
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      "/graphql": "http://localhost:8177",
      "/healthz": "http://localhost:8177",
      "/readyz": "http://localhost:8177",
      "/media-proxy": "http://localhost:8177",
      "/telemetry": "http://localhost:8177"
    }
  }
  };
});
