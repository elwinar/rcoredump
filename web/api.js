export function search(query) {
	let params = [];
	for (const name in query) {
		params.push(encodeURIComponent(name) + '=' + encodeURIComponent(query[name]));
	}
	return fetch(`${document.config.baseURL}/cores?${params.join('&')}`);
}

export default { search };
