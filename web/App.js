import React from "react";
import styles from "./App.scss";
import api from "./api.js";
import Header from "./Header.js";
import Footer from "./Footer.js";
import Searchbar from "./Searchbar.js";
import Table from "./Table.js";

// App is the main component, and is mainly concerned with high-level features
// like state management and top-level components.
export default function App() {
  const [cores, setCores] = React.useState([]);
  const [error, setError] = React.useState(null);
  const [query, setQuery] = React.useState({
    q: "*",
    sort: "dumped_at",
    order: "desc",
    size: "150",
  });
  const [total, setTotal] = React.useState(0);

  // Initialize the state, essentially updating it to use the query encoded in
  // the URL, if any.
  React.useEffect(function initializeQuery() {
    const raw = new URLSearchParams(window.location.search).get("q");
    if (raw === null) {
      return;
    }
    setQuery(api.decodeQuery(raw));
  }, []);

  // When the query change, we want to run the search query and update
  // the cores.
  React.useEffect(
    function runQuery() {
      api
        .search(query)
        .then((res) => {
          if (res.error) {
            setError(res.error);
            return;
          }
          setCores(res.results);
          setTotal(res.total);
        })
        .catch((err) => {
          setError(res.error);
        });
    },
    [query]
  );

  // The popstate event notify of the user using the back button of his browser
  // (or other similar event). Here we add an event listener when the app is
  // mounted, and remove it when it's unmounted. The handler is defined in the
  // effect function so its reference is the same for the full app lifecycle.
  React.useEffect(function popstateHandler() {
    function handler() {
      setQuery(api.decodeQuery(new URLSearchPArams(window.location.search).get("q")));
    }
    window.addEventListener("popstate", handler);
    return function () {
      window.removeEventListener("popstate", handler);
    };
  }, []);

  // When the query change, we want to update the URL value. We have to
  // check for the current value despite the hook dependency on the query
  // because the popstate history event already does this, and doing it
  // again break the forward-history.
  React.useEffect(
    function updateHash() {
      const q = api.encodeQuery(query);
      if (new URLSearchParams(window.location.search).get("q") === q) {
        return;
      }
      history.pushState({}, "", `/?q=${q}`);
    },
    [query]
  );

  function queryHandler(q) {
    setQuery(q);
  }

  function deleteCoreHandler() {
    if (!window.confirm(`are you sure you want to delete this core?`)) {
      return;
    }

    api
      .deleteCore(core.uid)
      .then((res) => {
        if (res.error) {
          setError(res.error);
          return;
        }
        setCores([...cores].filter((c) => c.uid != core.uid));
        setTotal(total - 1);
      })
      .catch((res) => {
        setError(err.message);
      });
  }

  // Finally, render the component itself. The header and searchbar are
  // always displayed, and the table gives way for fallback display in
  // case of error or if the first query didn't execute yet.
  return (
    <React.Fragment>
      <Header />
      <Searchbar query={query} onSubmit={queryHandler} />
      {error !== null && (
        <React.Fragment>
          <h2>Unexpected error</h2>
          <p>{error}</p>
        </React.Fragment>
      )}
      {cores === null && <p>No result yet.</p>}
      {cores !== null && <Table cores={cores} total={total} onDeleteCore={deleteCoreHandler} />}
      <Footer />
    </React.Fragment>
  );
}
