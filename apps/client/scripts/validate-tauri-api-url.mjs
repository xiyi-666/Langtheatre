const apiURL = process.env.VITE_API_URL?.trim();

if (!apiURL) {
  console.error("VITE_API_URL is required for Tauri release builds.");
  process.exit(1);
}

let parsed;
try {
  parsed = new URL(apiURL);
} catch {
  console.error("VITE_API_URL must be an absolute URL for Tauri release builds.");
  process.exit(1);
}

const localDevelopmentURL = process.env.TAURI_LOCAL_API_DEV === "1"
  && parsed.protocol === "http:"
  && (parsed.hostname === "localhost" || parsed.hostname === "127.0.0.1");

if (parsed.username || parsed.password || (parsed.protocol !== "https:" && !localDevelopmentURL)) {
  console.error("Tauri release builds require an HTTPS VITE_API_URL; set TAURI_LOCAL_API_DEV=1 only for localhost development.");
  process.exit(1);
}
