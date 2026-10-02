import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App.jsx";
import { WidgetGallery } from "./Widgets.jsx";
import { StateBoard } from "./States.jsx";
import { CliSurface } from "./Cli.jsx";
import "./styles.css";
import { runContract } from "./contract.js";
import { runMeasure } from "./measure.js";
import { runProbe } from "./probe.js";
import { runScanProbe } from "./scan-probe.js";
import { runHealthRecoveryProbe } from "./health-recovery-probe.js";

const surface = new URLSearchParams(window.location.search).get("surface");
const Surface =
  surface === "widgets" ? WidgetGallery : surface === "states" ? StateBoard : surface === "cli" ? CliSurface : App;

if (import.meta.env.DEV && new URLSearchParams(window.location.search).get("widgetContrast") === "1") {
  import("./widget-contrast-probe.js").then(({ runWidgetContrastProbe }) => runWidgetContrastProbe());
}

runContract();
runMeasure();
runProbe();
runScanProbe();
runHealthRecoveryProbe();

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <Surface />
  </StrictMode>,
);
