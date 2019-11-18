import React from 'react';
import styles from './App.scss';

function App() {
	const [entries, setEntries] = React.useState([]);
	const [query, setQuery] = React.useState({q: "*"});

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

	return (
		<form className={styles.Searchbar} onSubmit={submit}>
			<select name="sort" onChange={change}>
				{['-date', 'date', 'hostname', '-hostname', 'executable', '-executable'].map(field => {
					return <option key={field} value={field} selected={state.sort === field}>{field}</option>;
				})}
			</select>
			<input type="text" placeholder="coredump search query" name="q" value={state.q} onChange={change} dirty={state.q !== query.q ? "true": undefined} />
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
