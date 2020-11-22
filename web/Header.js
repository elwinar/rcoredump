import React from "react";
import styles from "./Header.scss";

// Header is a separate component so it can be shared in the AppBoundary and in
// the App itself.
export default function Header() {
  return (
    <header className={styles.Header}>
      <h1>
        RCoredump <sup>{document.Version}</sup>
      </h1>
    </header>
  );
}
