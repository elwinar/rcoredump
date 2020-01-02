import React from 'react';
import styles from './App.scss';

function App() {
	const [entries, setEntries] = React.useState([]);
	const [query, setQuery] = React.useState({q: '*', sort: '-date', size: '20'});

	React.useEffect(function(){
		const h = new URLSearchParams(document.location.search.substring(1)).get('query');
		if (h === null) {
			return;
		}
		setQuery(JSON.parse(atob(h)));
	}, []);

	React.useEffect(function() {
		let params = [];
		for (const name in query) {
			params.push(encodeURIComponent(name) + '=' + encodeURIComponent(query[name]));
		}
		fetch(`${document.config.baseURL}/cores?${params.join('&')}`)
			.then(res => res.json())
			.then(function(res){
				setEntries(res.hits.map(x => x.fields));
			});
	}, [query]);

	React.useEffect(function(){
		const h = btoa(JSON.stringify(query));
		history.pushState({query: h}, '', `?query=${h}`);
	}, [query]);

	return (
		<React.Fragment>
			<header className={styles.Header}>
				<h1>RCoredump</h1>
			</header>
			<Searchbar setQuery={setQuery} query={query}>
			</Searchbar>
			<Table entries={entries}>
			</Table>
		</React.Fragment>
	);
}

function Searchbar(props) {
	const {query, setQuery, ...attributes} = props;
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
	);
}

function Table(props) {
	return (
		<table className={styles.Table}>
			<thead>
				<tr>
					<th>id</th>
					<th>date</th>
					<th>hostname</th>
					<th>executable</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				{props.entries.map(x => {
					return (<tr key={x.uid}>
						<td>{x.uid}</td>
						<td>{x.date}</td>
						<td>{x.hostname}</td>
						<td>{x.executable_path}</td>
						<td><a href={`${document.config.baseURL}/cores/${x.uid}`}>core</a> <a href={`${document.config.baseURL}/binaries/${x.binary_hash}`}>binary</a></td>
					</tr>);
				})}
			</tbody>
		</table>
	);
}

export default App;
