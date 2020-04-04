import React from 'react';
import styles from './App.scss';
import dayjs from 'dayjs';


// Default query the user is redirected to if there is none.
const defaultQuery = {q: '*', sort: 'dumped_at', order: 'desc', size: '150'};
// Default result to use for initial values and in case of errors.
const defaultResults = {results:[], total: 0};
// Maximum number of pages displayed by the pagination.
const maxPages = 5;
// Page size for displaying the results.
const pageSize = 15;


// Encore a query as string.
function encodeQuery(q) {
	return btoa(JSON.stringify(q));
}

// Decode the string version of a query.
function decodeQuery(q) {
	return JSON.parse(atob(q));
}

// Return a human-readable version of a size in Bytes.
function formatSize(bytes) {
	const threshold = 1000;
	const units = ['B', 'KB','MB','GB','TB','PB','EB','ZB','YB'];
	let u = 0;
	while (Math.abs(bytes) >= threshold && u < units.length - 1) {
		bytes /= threshold;
		u += 1;
	}
	return bytes.toFixed(1) + ' ' + units[u];
}

// dayjs is a lightweight momentjs-like library with mostly compatible API. I
// just need the UTC plugin to be able to handle timezones.
var utc = require('dayjs/plugin/utc');
dayjs.extend(utc);

// Format the date in a more friendly manner for display.
function formatDate(date) {
	return dayjs(date).local().format('YYYY-MM-DD HH:mm:ss');
}

// boolattr return the value for a non-HTML boolean attribute.
function boolattr(b) {
	return b ? 'true' : undefined;
}

const quotes = [
	":-)",
	"Seems like good news.",
	"Do not fear failure, but rather feat not trying.",
];

// inspirational returns a random quote from a list to display when there is
// not result to display.
function inspirational() {
	return quotes[Math.floor(Math.random() * quotes.length)];
}


function App() {
	let q = new URLSearchParams(document.location.search.substring(1)).get('q');
	if (q === null) {
		q = defaultQuery;
	} else {
		q = decodeQuery(q);
	}

	const [query, setQuery] = React.useState(q);
	const [results, setResults] = React.useState(defaultResults);

	React.useEffect(function() {
		let params = [];
		for (const name in query) {
			params.push(encodeURIComponent(name) + '=' + encodeURIComponent(query[name]));
		}
		fetch(`${document.config.baseURL}/cores?${params.join('&')}`)
			.then(res => res.json())
			.then(function(res){
				if (res.results == null) {
					res.results = [];
				}
				setResults(res || defaultResults);
			});
	}, [query]);

	React.useEffect(function(){
		history.pushState({}, '', `/?q=${encodeQuery(query)}`);
	}, [query]);

	return (
		<React.Fragment>
			<header className={styles.Header}>
				<h1>RCoredump <sup>{document.Version}</sup></h1>
			</header>
			<Searchbar setQuery={setQuery} query={query} />
			<Table results={results.results} total={results.total} />
		</React.Fragment>
	);
}

function Searchbar(props) {
	const {query, setQuery} = props;
	const [state, setState] = React.useState(query);
	const [dirty, setDirty] = React.useState(false);

	React.useEffect(function() {
		setState(query);
	}, [query]);

	React.useEffect(function() {
		setDirty(Object.keys(query).some(prop => state[prop] !== query[prop]));
	}, [state]);

	function change(ev) {
		setState({
			...state,
			[ev.target.name]: ev.target.value,
		});
	}

	function submit(ev) {
		ev.preventDefault();
		setQuery(state);
		setDirty(false);
	}

	return (
		<React.Fragment>
			<form className={styles.Searchbar} onSubmit={submit}>
				<div>
					<fieldset>
						{['dumped_at', 'hostname'].map(field => {
							const isActive = boolattr(state.sort === field);
							const isDirty = boolattr(state.sort === field && state.sort !== query.sort);
							const isChecked = state.sort === field;
							return (
								<label className={styles.Radio} key={field} field="sort" active={isActive} dirty={isDirty}>
									{field}
									<input type="radio" name="sort" value={field} onChange={change} checked={isChecked} />
								</label>
							);
						})}
					</fieldset>
					<fieldset>
						{['asc', 'desc'].map(field => {
							const isActive = boolattr(state.order === field);
							const isDirty = boolattr(state.order === field && state.order !== query.order);
							const isChecked = state.order === field;
							return (
								<label className={styles.Radio} key={field} field="order" active={isActive} dirty={isDirty}>
									{field}
									<input type="radio" name="order" value={field} onChange={change} checked={isChecked} />
								</label>
							);
						})}
					</fieldset>
				</div>
				<div>
					<input type="text" placeholder="coredump search query" name="q" value={state.q} onChange={change} dirty={boolattr(state.q !== query.q)} />
					<button type="submit" disabled={!dirty}>apply</button>
				</div>
				<div>
					<p><a href="https://blevesearch.com/docs/Query-String-Query/" target="_blank">query string reference</a></p>
				</div>
			</form>
		</React.Fragment>
	);
}

function Table(props) {
	const {results, total} = props;
	const [selected, setSelected] = React.useState(null);
	const [page, setPage] = React.useState(1);
	const totalPages = Math.ceil(results.length/pageSize);

	function toggle(uid) {
		setSelected(selected == uid ? null : uid);
		return false;
	}

	if (results.length == 0) {
		return (
			<p className={styles.NoResult}>No match for this query. {inspirational()}</p>
		)
	}

	// Compute the page list by transforming a list of indices like [0, 1,
	// 2, 3, 4] by shifting them from an offset computed from the current
	// page (to avoid the "-1" page, and "max+1" page).
	// Special case if there is less than maxPages pages to display, in
	// which case we display them all.
	var pages;
	if (totalPages == 1) {
		pages = [];
	} else if (totalPages <= maxPages) {
		pages = Array.from({length: totalPages}).map((_, index) => index + 1);
	} else {
		const spread = Math.floor(maxPages / 2);
		const offset = Math.min(Math.max(page, spread+1), totalPages-spread);
		pages = Array.from({length: maxPages}).map((_,index) => {
			return offset - spread + index;
		});
	}

	return (
		<React.Fragment>
			<ul className={styles.Pagination}>
				{pages.map(p => {
					return <li key={p} active={boolattr(p === page)} onClick={() => setPage(p)}>{p}</li>
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
					{results.slice((page-1)*pageSize, page*pageSize).map(x => {
						return (
							<React.Fragment key={x.uid}>
								<tr onClick={() => toggle(x.uid)} active={boolattr(selected == x.uid)}>
									<td>▶</td>
									<td>{formatDate(x.dumped_at)}</td>
									<td>{x.hostname}</td>
									<td>{x.executable}</td>
									<td>{x.lang}</td>
								</tr>
								{selected == x.uid && <tr className={styles.Detail}><td colSpan="5"><Core core={x} /></td></tr>}
							</React.Fragment>
						);
					})}
				</tbody>
			</table>
		</React.Fragment>
	);
}

function Core(props) {
	const {core} = props;

	function analyze(uid) {
		fetch(`${document.config.baseURL}/cores/${uid}/_analyze`, { method: 'POST' });
	}

	return (
		<React.Fragment>
			<ul>
				<li><a class={styles.Button} href={`${document.config.baseURL}/cores/${core.uid}`}>download core ({formatSize(core.size, true)})</a></li>
				<li><a class={styles.Button} href={`${document.config.baseURL}/executables/${core.executable_hash}`}>download executable ({formatSize(core.executable_size, true)})</a></li>
			</ul>
			<h2>executable</h2>
			<dl>
				<dt>executable_hash</dt><dd>{core.executable_hash}</dd>
				<dt>executable_path</dt><dd>{core.executable_path}</dd>
			</dl>
			<h2>coredump</h2>
			<dl>
				<dt>uid</dt><dd>{core.uid}</dd>
				{Object.keys(core.metadata).map(x => {
					return (
						<React.Fragment key={x}>
							<dt>metadata.{x}</dt>
							<dd>{core.metadata[x]}</dd>
						</React.Fragment>
					);
				})}
			</dl>
			<h2>stack trace</h2>
			<dl>
				<dt>analyzed_at</dt><dd>{formatDate(core.analyzed_at)}</dd>
			</dl>
			{core.trace !== undefined ? <pre>{core.trace}</pre> : <p>No trace</p>}
		</React.Fragment>
	);
}

export default App;
