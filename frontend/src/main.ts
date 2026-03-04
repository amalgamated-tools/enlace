import { mount } from "svelte";
import "./app.css";
import App from "./App.svelte";
import { applyInitialTheme } from "./lib/stores/theme";

applyInitialTheme();

const app = mount(App, {
  target: document.getElementById("app")!,
});

export default app;
