import React from "react";
import styles from "./Searchbar.scss";
import { boolattr } from "./utils.js";

// Searchbar is one of the top-level components, tasked with handling the
// interface to edit the search query.
export default function Searchbar({ query, onSubmit }) {
  // The local payload is initialized from the current query, and will hold
  // dirty values until the user submit the form.
  const [payload, setPayload] = React.useState(query);
  const [dirty, setDirty] = React.useState(false);

  // We want to update the current payload when the query change. As the
  // searchbar is never unmounted, this isn't done automatically.
  React.useEffect(resetHandler, [query]);

  // dirty is used to activate or not the apply and reset buttons when
  // the state isn't equivalent to the initial query.
  React.useEffect(
    function updateDirty() {
      setDirty(Object.keys(query).some((prop) => payload[prop] !== query[prop]));
    },
    [payload]
  );

  // changeHandler is used by form component when their payload change to update
  // the local state.
  function changeHandler(e) {
    setPayload({
      ...payload,
      [e.target.name]: e.target.value,
    });
  }

  // submitHandler is used by the apply button when it is clicked so we can
  // propagate the state to the parent component.
  function submitHandler(e) {
    e.preventDefault();
    onSubmit(payload);
  }

  // resetHandler is used by the reset button when it is clicked so we can
  // reset the state to the query payload.
  function resetHandler() {
    setPayload(query);
  }

  return (
    <React.Fragment>
      <form className={styles.Searchbar} onSubmit={submitHandler}>
        <div>
          <fieldset>
            {["dumped_at", "hostname"].map((field) => {
              const isActive = boolattr(payload.sort === field);
              const isDirty = boolattr(payload.sort === field && payload.sort !== query.sort);
              const isChecked = payload.sort === field;
              return (
                <label
                  className={styles.Radio}
                  key={field}
                  field="sort"
                  active={isActive}
                  dirty={isDirty}
                >
                  {field}
                  <input
                    type="radio"
                    name="sort"
                    value={field}
                    onChange={changeHandler}
                    checked={isChecked}
                  />
                </label>
              );
            })}
          </fieldset>
          <fieldset>
            {["asc", "desc"].map((field) => {
              const isActive = boolattr(payload.order === field);
              const isDirty = boolattr(payload.order === field && payload.order !== query.order);
              const isChecked = payload.order === field;
              return (
                <label
                  className={styles.Radio}
                  key={field}
                  field="order"
                  active={isActive}
                  dirty={isDirty}
                >
                  {field}
                  <input
                    type="radio"
                    name="order"
                    value={field}
                    onChange={changeHandler}
                    checked={isChecked}
                  />
                </label>
              );
            })}
          </fieldset>
        </div>
        <div>
          <input
            type="text"
            placeholder="coredump search query"
            name="q"
            value={payload.q}
            onChange={changeHandler}
            dirty={boolattr(payload.q !== query.q)}
          />
          <button type="submit" disabled={!dirty}>
            apply
          </button>
          <button onClick={resetHandler} disabled={!dirty}>
            reset
          </button>
        </div>
        <div>
          <p>
            <a href="https://blevesearch.com/docs/Query-String-Query/" target="_blank">
              query string reference
            </a>
          </p>
        </div>
      </form>
    </React.Fragment>
  );
}
