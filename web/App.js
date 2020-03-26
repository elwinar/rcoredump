import React from 'react';
import styles from './App.scss';
import dayjs from 'dayjs';

var utc = require('dayjs/plugin/utc');
dayjs.extend(utc);

function encodeQuery(q) {
	return btoa(JSON.stringify(q));
}

function decodeQuery(q) {
	return JSON.parse(atob(q));
}

const defaultQuery = {q: '*', sort: 'date', order: 'desc', size: '50'};

// Those variables are defined at compile-time by Parcel.
const Version = process.env.VERSION;

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

function App() {
	let q = new URLSearchParams(document.location.search.substring(1)).get('q');
	if (q === null) {
		q = defaultQuery;
	} else {
		q = decodeQuery(q);
	}

	const [query, setQuery] = React.useState(q);
	const [entries, setEntries] = React.useState([]);

	React.useEffect(function() {
		let params = [];
		for (const name in query) {
			params.push(encodeURIComponent(name) + '=' + encodeURIComponent(query[name]));
		}
		fetch(`${document.config.baseURL}/cores?${params.join('&')}`)
			.then(res => res.json())
			.then(function(res){
				setEntries(res || []);
			});
	}, [query]);

	React.useEffect(function(){
		history.pushState({}, '', `/?q=${encodeQuery(query)}`);
	}, [query]);

	return (
		<React.Fragment>
			<header className={styles.Header}>
				<h1>RCoredump <sup>{Version}</sup></h1>
			</header>
			<Searchbar setQuery={setQuery} query={query} />
			<Table entries={entries} />
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
						{['date', 'hostname'].map(field => {
							const isActive = state.sort === field ? 'true' : undefined;
							const isDirty = state.sort === field && state.sort !== query.sort ? 'true' : undefined;
							return (
								<label className={styles.Radio} key={field} field="sort" active={isActive} dirty={isDirty}>
									{field}
									<input type="radio" name="sort" value={field} onChange={change} checked={state.sort === field} />
								</label>
							);
						})}
					</fieldset>
					<fieldset>
						{['asc', 'desc'].map(field => {
							const isActive = state.order === field ? 'true' : undefined;
							const isDirty = state.order === field && state.order !== query.order ? 'true' : undefined;
							return (
								<label className={styles.Radio} key={field} field="order" active={isActive} dirty={isDirty}>
									{field}
									<input type="radio" name="order" value={field} onChange={change} checked={state.order === field} />
								</label>
							);
						})}
					</fieldset>
				</div>
				<div>
					<input type="text" placeholder="coredump search query" name="q" value={state.q} onChange={change} dirty={state.q !== query.q ? 'true' : undefined} />
					<button type="submit" disabled={!dirty}>Apply</button>
				</div>
				<div>
					<p><a href="https://blevesearch.com/docs/Query-String-Query/" target="_blank">query string reference</a></p>
				</div>
			</form>
		</React.Fragment>
	);
}

function Table(props) {
	const {entries} = props;
	const [selected, setSelected] = React.useState(null);

	function toggle(uid) {
		if (selected == uid) {
			setSelected(null);
		} else {
			setSelected(uid);
		}
		return false;
	}

	return (
		<React.Fragment>
			<table className={styles.Table}>
				<thead>
					<tr>
						<th></th>
						<th>date</th>
						<th>hostname</th>
						<th>executable</th>
						<th>lang</th>
					</tr>
				</thead>
				<tbody>
					{entries.map(x => {
						return (
							<React.Fragment key={x.uid}>
								<tr onClick={() => toggle(x.uid)} active={selected == x.uid ? 'true' : undefined}>
									<td>▶</td>
									<td>{dayjs(x.date).local().format('YYYY-MM-DD HH:mm:ss')}</td>
									<td>{x.hostname}</td>
									<td>{x.executable_path.split('/').pop()}</td>
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
			<h3>metadata</h3>
			<dl>
				<dt>core</dt><dd>{core.uid} ({formatSize(core.size, true)}, <a href={`${document.config.baseURL}/cores/${core.uid}`}>download</a>)</dd>
				<dt>executable hash</dt><dd>{core.executable_hash} ({formatSize(core.executable_size, true)}, <a href={`${document.config.baseURL}/executables/${core.executable_hash}`}>download</a>)</dd>
				<dt>executable path</dt><dd>{core.executable_path}</dd>
				{Object.keys(core.metadata).map(x => {
					return (
						<React.Fragment key={x}>
							<dt>metadata.{x}</dt>
							<dd>{core.metadata[x]}</dd>
						</React.Fragment>
					);
				})}
			</dl>
			<h3>stack trace</h3>
			{core.trace !== undefined ? <pre>{core.trace}</pre> : <p>No trace</p>}
		</React.Fragment>
	);
}

export default App;
