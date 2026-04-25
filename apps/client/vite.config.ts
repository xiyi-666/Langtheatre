import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      "/graphql": "http://localhost:8177",
      "/healthz": "http://localhost:8177",
      "/readyz": "http://localhost:8177",
      "/media-proxy": "http://localhost:8177"
    }
  }
});
