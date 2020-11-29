import React from "react";

if (process.env.NODE_ENV === "development") {
  const whyDidYouRender = require("@welldone-software/why-did-you-render");
  whyDidYouRender(React, {
    trackAllPureComponents: true,
  });
}

import ReactDOM from "react-dom";
import App from "./App.js";
import AppBoundary from "./AppBoundary.js";
import Header from "./Header.js";
import Footer from "./Footer.js";
import "./index.scss";

if (document.config === undefined) {
  document.config = {
    baseURL: window.location.origin,
  };
}

ReactDOM.render(
  <React.Fragment>
    <Header />
    <AppBoundary>
      <App />
    </AppBoundary>
    <Footer />
  </React.Fragment>,
  document.getElementById("root")
);
