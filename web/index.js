import React from "react";
import ReactDOM from "react-dom";
import App from "./App.js";
import AppBoundary from "./AppBoundary.js";
import "./index.scss";

if (document.config === undefined) {
  document.config = {
    baseURL: window.location.origin,
  };
}

ReactDOM.render(
  <AppBoundary>
    <App />
  </AppBoundary>,
  document.getElementById("root")
);
