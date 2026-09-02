import { createApp } from "vue";
import App from "./App.vue";
import "./style.css";

// Register before Vue mounts so a drop cannot navigate the WebView
// (Windows needs AllowExternalDrag on; this is the substitute for
// DisableWebViewDrop).
window.addEventListener(
  "dragover",
  (e) => {
    if (!e.dataTransfer?.types || ![...e.dataTransfer.types].includes("Files")) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
  },
  true
);
window.addEventListener(
  "drop",
  (e) => {
    if (!e.dataTransfer?.types || ![...e.dataTransfer.types].includes("Files")) return;
    e.preventDefault();
  },
  true
);

createApp(App).mount("#app");
