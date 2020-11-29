// Encore a query as string.
function encodeQuery(q) {
  return btoa(JSON.stringify(q));
}

// Decode the string version of a query.
function decodeQuery(q) {
  return JSON.parse(atob(q));
}

// Build the route to an endpoint. Used both by the internal API clients and
// the app for download links.
function route(endpoint) {
  return `${document.config.baseURL}${endpoint}`;
}

function call(endpoint, options) {
  return fetch(route(endpoint), options);
}

function search(query) {
  let params = [];
  for (const name in query) {
    params.push(encodeURIComponent(name) + "=" + encodeURIComponent(query[name]));
  }
  return call(`/cores?${params.join("&")}`).then((res) => res.json());
}

function deleteCore(uid) {
  return call(`/cores/${uid}`, {
    method: "DELETE",
  });
}

export default {
  route,
  search,
  deleteCore,
  encodeQuery,
  decodeQuery,
};
