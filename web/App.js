import React from 'react';
import styles from './App.scss';

function encodeQuery(q) {
	return btoa(JSON.stringify(q));
}

function decodeQuery(q) {
	return JSON.parse(atob(q));
}

const defaultQuery = {q: '*', sort: '-date', size: '20'};

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
				setEntries(res.hits.map(x => {
					sessionStorage.setItem(x.fields.uid, JSON.stringify(x.fields));
					return x.fields;
				}));
			});
	}, [query]);

	React.useEffect(function(){
		history.pushState({}, '', `/?q=${encodeQuery(query)}`);
	}, [query]);

	return (
		<React.Fragment>
			<header className={styles.Header}>
				<h1>RCoredump</h1>
			</header>
			<Searchbar setQuery={setQuery} query={query} />
			<Table entries={entries} />
		</React.Fragment>
	);
}

function Searchbar(props) {
	const {query, setQuery} = props;
	const [state, setState] = React.useState(query);
	const [advanced, setAdvanced] = React.useState('');

	React.useEffect(function() {
		setState(query);
	}, [query]);

	function change(ev) {
		setState({
			...state,
			[ev.target.name]: ev.target.value,
		});
	}

	function submit(ev) {
		ev.preventDefault();
		setQuery(state);
	}

	function toggle(ev) {
		setAdvanced(advanced === ev.target.name ? '' : ev.target.name);
	}

	return (
		<React.Fragment>
			<form className={styles.Searchbar} onSubmit={submit}>
				<fieldset>
					<button type="button" name="size" dirty={state.size !== query.size ? 'true' : undefined } onClick={toggle}>size: {state.size}</button>
					<button type="button" name="sort" dirty={state.sort !== query.sort ? 'true' : undefined } onClick={toggle}>sort: {state.sort}</button>
					<button type="submit" onClick={toggle}>Apply</button>
				</fieldset>
				<fieldset style={{display: advanced === 'size' ? 'block' : 'none' }}>
					<p>Size: {['10', '20', '50'].map(field => {
						return (<React.Fragment key={field}>
							<input type="radio" name="size" id={field} value={field} checked={state.size === field} onChange={change} />
							<label htmlFor={field}>{field}</label>
						</React.Fragment>);
					})}</p>
				</fieldset>
				<fieldset style={{display: advanced === 'sort' ? 'block' : 'none' }}>
					<p>Sort: {['-date', 'date', 'hostname', '-hostname', 'executable', '-executable'].map(field => {
						return (<React.Fragment key={field}>
							<input type="radio" name="sort" id={field} value={field} checked={state.sort === field} onChange={change} />
							<label htmlFor={field}>{field}</label>
						</React.Fragment>);
					})}</p>
				</fieldset>
				<input type="text" placeholder="coredump search query" name="q" value={state.q} onChange={change} dirty={state.q !== query.q ? 'true' : undefined} />
				<p><a href="https://blevesearch.com/docs/Query-String-Query/" target="_blank">query string reference</a></p>
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
					{props.entries.map(x => {
						return (
							<React.Fragment key={x.uid}>
								<tr>
									<td className={styles.Toggle} onClick={() => toggle(x.uid)}>{ selected == x.uid ? '▼' : '▶' }</td>
									<td>{x.date}</td>
									<td>{x.hostname}</td>
									<td>{x.executable_path}</td>
									<td>{x.lang}</td>
								</tr>
								{ selected == x.uid && <tr className={styles.Detail}><td colspan="6"><Core core={x} /></td></tr> }
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

	return (
		<React.Fragment>
			<dl className={styles.Description}>
				<dt>uid</dt><dd>{core.uid}</dd>
				<dt>date</dt><dd>{core.date}</dd>
				<dt>hostname</dt><dd>{core.hostname}</dd>
				<dt>executable</dt><dd>{core.executable_path}</dd>
				<dt>lang</dt><dd>{core.lang}</dd>
			</dl>
			{ core.trace !== undefined ? <pre>{core.trace}</pre> : <p>No trace</p> }
		</React.Fragment>
	);
}

export default App;
