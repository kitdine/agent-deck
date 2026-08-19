import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App.jsx";
import { WidgetGallery } from "./Widgets.jsx";
import { StateBoard } from "./States.jsx";
import "./styles.css";
import { runContract } from "./contract.js";
import { runMeasure } from "./measure.js";
import { runProbe } from "./probe.js";

const surface = new URLSearchParams(window.location.search).get("surface");
const Surface = surface === "widgets" ? WidgetGallery : surface === "states" ? StateBoard : App;

runContract();
runMeasure();
runProbe();

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <Surface />
  </StrictMode>,
);
