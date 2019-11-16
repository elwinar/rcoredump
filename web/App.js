import React from 'react';
import styles from './App.scss';

function App() {
	const [entries, setEntries] = React.useState([]);
	React.useEffect(function(){
		searchHandler({q:""});
	}, []);

	function searchHandler(query) {
		const q = encodeURIComponent(query.q.trim() || "*");
		fetch(`${document.config.baseURL}/cores?q=${q}`)
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
	const [submitted, setSubmitted] = React.useState({});
	let refs = {
		'q': React.useRef(null),
	};

	function changeHandler(ev) {
		if (submitted[ev.target.name] !== ev.target.value) {
			ev.target.setAttribute('dirty', '');
		} else {
			ev.target.removeAttribute('dirty');
		}
	}

	function submitHandler(ev) {
		ev.preventDefault();
		let query = {};
		Object.values(refs).forEach(function(ref) {
			query[ref.current.name] = ref.current.value;
		});
		setSubmitted(query);
		props.handler(query);
		Object.values(refs).forEach(function(ref) {
			ref.current.removeAttribute('dirty');
		});
	}

	return (
		<form className={styles.Searchbar} onSubmit={submitHandler}>
			<input type="text" ref={refs.q} name="q" placeholder="coredump search query" onChange={changeHandler} />
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
