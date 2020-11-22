import React from "react";
import styles from "./Table.scss";
import Core from "./Core.js";
import { boolattr, formatDate } from "./utils.js";

// Table is the top-level component tasked with displaying the cores.
export default function Table({ cores, total }) {
  // page and selected are used to control what gets displayed on screen,
  // either by limiting the number of elements or displaying the details
  // of a result.
  const [page, setPage] = React.useState(1);
  const [selected, setSelected] = React.useState(null);

  // If we don't have anything to display, fallback to a line saying so,
  // and a nice message. Query strings can be frustrating, and Bleve's
  // format is especially horendous.
  if (cores.length == 0) {
    const quotes = [
      ":-)",
      "Seems like good news.",
      "Do not fear failure, but rather fear not trying.",
    ];
    return (
      <p className={styles.NoResult}>
        No match for this query. {quotes[Math.floor(Math.random() * quotes.length)]}
      </p>
    );
  }

  // Compute the page list by transforming a list of indices like [0, 1,
  // 2, 3, 4] by shifting them from an offset computed from the current
  // page (to avoid the "-1" page, and "max+1" page).
  // Special case if there is less than maxPages pages to display, in
  // which case we display them all.
  const maxPages = 5;
  const pageSize = 15;
  const totalPages = Math.ceil(cores.length / pageSize);
  var pages;
  if (totalPages == 1) {
    pages = [];
  } else if (totalPages <= maxPages) {
    pages = Array.from({ length: totalPages }).map((_, index) => index + 1);
  } else {
    const spread = Math.floor(maxPages / 2);
    const offset = Math.min(Math.max(page, spread + 1), totalPages - spread);
    pages = Array.from({ length: maxPages }).map((_, index) => {
      return offset - spread + index;
    });
  }

  // Display both the pagination, the table, and the eventually selected
  // coredump details.
  return (
    <React.Fragment>
      <ul className={styles.Pagination}>
        {pages.map((p) => {
          return (
            <li key={p} active={boolattr(p === page)} onClick={() => setPage(p)}>
              {p}
            </li>
          );
        })}
      </ul>
      <table className={styles.Table}>
        <thead>
          <tr>
            <th></th>
            <th>dumped_at</th>
            <th>hostname</th>
            <th>executable</th>
            <th>lang</th>
          </tr>
        </thead>
        <tbody>
          {cores.slice((page - 1) * pageSize, page * pageSize).map((x) => {
            return (
              <React.Fragment key={x.uid}>
                <tr
                  onClick={() => setSelected(selected == x.uid ? null : x.uid)}
                  active={boolattr(selected == x.uid)}
                >
                  <td>▶</td>
                  <td>{formatDate(x.dumped_at)}</td>
                  <td>{x.hostname}</td>
                  <td>{x.executable}</td>
                  <td>{x.lang}</td>
                </tr>
                {selected == x.uid && (
                  <tr className={styles.Detail}>
                    <td colSpan="5">
                      <Core core={x} />
                    </td>
                  </tr>
                )}
              </React.Fragment>
            );
          })}
        </tbody>
      </table>
    </React.Fragment>
  );
}
