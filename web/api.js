export function route(endpoint) {
	return `${document.config.baseURL}${endpoint}`;
}

export function call(endpoint, options) {
	return fetch(route(endpoint), options);
}

export function search(query) {
	let params = [];
	for (const name in query) {
		params.push(encodeURIComponent(name) + '=' + encodeURIComponent(query[name]));
	}
	return call(`/cores?${params.join('&')}`);
}

export function deleteCore(uid) {
	return call(`/cores/${uid}`, {method: 'delete'});
}

export default { route, call, search, deleteCore };
