import React from "react";
import styles from "./Core.scss";
import { formatSize, formatDate } from "./utils.js";
import api from "./api.js";
import QueryLink from "./QueryLink.js";

// Core is a view of a core's details.
export default function Core({ core, onDelete }) {
  // We use a ref so we can have a simpler copy routine.
  const downloadAndDebug = React.useRef();
  function copy() {
    const selection = window.getSelection();
    selection.selectAllChildren(downloadAndDebug.current);
    document.execCommand("copy");
    selection.removeAllRanges();
  }

  // The component is a pure component that does nothing else than
  // extract a bunch of formatting details from the already non-trivial
  // Table component.
  return (
    <article className={styles.Core}>
      <ul>
        <li>
          <a className={styles.Button} href={api.route(`/cores/${core.uid}`)}>
            download core ({formatSize(core.size)})
          </a>
        </li>
        <li>
          <a className={styles.Button} href={api.route(`/executables/${core.executable_hash}`)}>
            download executable ({formatSize(core.executable_size)})
          </a>
        </li>
        <li>
          <button onClick={onDelete}>delete core</button>
        </li>
      </ul>
      <h2>executable</h2>
      <dl>
        <dt>executable_hash</dt>
        <dd>
          <QueryLink query={`executable_hash:"${core.executable_hash}"`}>
            {core.executable_hash}
          </QueryLink>
        </dd>
        <dt>executable_path</dt>
        <dd>{core.executable_path}</dd>
      </dl>
      <h2>coredump</h2>
      <dl>
        <dt>uid</dt>
        <dd>
          <QueryLink query={`uid:"${core.uid}"`}>{core.uid}</QueryLink>
        </dd>
        {Object.keys(core.metadata).map((x) => {
          return (
            <React.Fragment key={x}>
              <dt>metadata.{x}</dt>
              <dd>{core.metadata[x]}</dd>
            </React.Fragment>
          );
        })}
      </dl>
      <h3>
        download & debug <button onClick={copy}>copy</button>
      </h3>
      <pre ref={downloadAndDebug}>
        curl -s --compressed "{api.route(`/cores/${core.uid}`)}" --output {core.executable}.
        {core.uid}
        <br />
        curl -s --compressed "{api.route(`/executables/${core.executable_hash}`)}" --output{" "}
        {core.executable}
        <br />
        {core.lang == "C" && `gdb ${core.executable} ${core.executable}.${core.uid}`}
        {core.lang == "Go" && `dlv core ${core.executable} ${core.executable}.${core.uid}`}
      </pre>
      <h2>stack trace</h2>
      <dl>
        <dt>analyzed_at</dt>
        <dd>{formatDate(core.analyzed_at)}</dd>
      </dl>
      {core.trace !== undefined ? <pre>{core.trace}</pre> : <p>No trace</p>}
    </article>
  );
}
