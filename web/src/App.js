import React from 'react';
import './App.css';

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

		fetch(`${document.config.baseURL}/_search?q=${query}`)
			.then(res => res.json())
			.then(function(res){
				setEntries(res.hits.map(x => {
					return {
						id: x.id,
						date: x.fields.date,
						executable: x.fields.executable,
						hostname: x.fields.hostname,
					}
				}));
			});
	}

	return (
		<React.Fragment>
			<header>
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
		<input type="text" name="search" placeholder="coredump search query" onChange={handler}/>
	);
}

function Table(props) {
	console.log(props);
	return (
		<table>
			{props.entries.map(x => {
				return (<tr key={x.id}>
					<td>{x.id}</td>
					<td>{x.date}</td>
					<td>{x.executable}</td>
					<td>{x.hostname}</td>
				</tr>);
			})}
		</table>
	);
}


export default App;
