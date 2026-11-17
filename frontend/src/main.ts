import App from "./App.svelte";
import { mount } from "svelte";
import { setNative } from "./lib/native";
import "./style.css";
import "@vscode/codicons/dist/codicon.css";
import "@xterm/xterm/css/xterm.css";

async function start() {
  // Browser-only runs (end-to-end tests, UI development without the Go
  // backend) use a deterministic in-memory backend instead of Wails.
  if (import.meta.env.VITE_MOCK_NATIVE === "1") {
    const { createMockNative } = await import("./lib/mockNative");
    const mock = createMockNative();
    setNative(mock);
    (window as unknown as { __mockNative: unknown }).__mockNative = mock;
  } else {
    const { createWailsNative } = await import("./lib/native.wails");
    setNative(createWailsNative());
  }
  mount(App, { target: document.getElementById("app")! });
}

void start();