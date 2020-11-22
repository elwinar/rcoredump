import React from "react";
import styles from "./Footer.scss";

// Footer is a separate component so it can be shared in the AppBoundary and in
// the App itself.
export default function Footer() {
  return (
    <footer className={styles.Footer}>
      <p>
        For documentation, issues, see the{" "}
        <a href="https://github.com/elwinar/rcoredump">repository</a>.
      </p>
    </footer>
  );
}
