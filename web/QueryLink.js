import React from "react";
import api from "./api.js";

// QueryLink can be used to make a direct link to a query search. The link is a
// standard HTML link with a valid href, but the navigation is intercepted to
// be handled by the app. This allow the user to copy-paste the link via his
// navigator contextual menu, while making internal navigation easy.
export default function QueryLink({ children, query, onClick }) {
  return (
    <a
      href={`/?q=${api.encodeQuery({ q: query })}`}
      onClick={(e) => {
        e.preventDefault();
        onClick();
      }}
    >
      {children}
    </a>
  );
}
