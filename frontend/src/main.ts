import App from "./App.svelte";
import { mount } from "svelte";
import "./style.css";
import "@vscode/codicons/dist/codicon.css";

mount(App, { target: document.getElementById("app")! });
