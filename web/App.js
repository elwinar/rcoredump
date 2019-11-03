import React from 'react';
import styles from './App.scss';

function App() {
	const [entries, setEntries] = React.useState([]);
	React.useEffect(function(){
		searchHandler("");
	}, []);

	function searchHandler(query) {
		query = query.trim();
		if (query.length === 0) {
			query = "*";
		}
		query = encodeURIComponent(query);

		fetch(`${document.config.baseURL}/cores?q=${query}`)
			.then(res => res.json())
			.then(function(res){
				setEntries(res.hits.map(x => x.fields));
			});
	}

	return (
		<React.Fragment>
			<header className={styles.Header}>
				<h1>RCoredump</h1>
			</header>
			<Searchbar handler={searchHandler}>
			</Searchbar>
			<Table entries={entries}>
			</Table>
		</React.Fragment>
	);
}

function Searchbar(props) {
	let searchTimeout = null;

	function handler(ev) {
		let val = ev.target.value;
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}
		searchTimeout = setTimeout(function() {
			props.handler(val);
		}, 500);
	}

	return (
		<input className={styles.Searchbar} type="text" name="search" placeholder="coredump search query" onChange={handler}/>
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
