import React from 'react';
import styles from './App.scss';

function App() {
	const [entries, setEntries] = React.useState([]);
	React.useEffect(function(){
		searchHandler({q:""});
	}, []);

	function searchHandler(query) {
		query.q = query.q.trim();
		if (query.q.length === 0) {
			query.q = "*";
		}
		query.q = encodeURIComponent(query.q);

		fetch(`${document.config.baseURL}/cores?q=${query.q}`)
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
	const input = React.useRef(null);

	function handler(ev) {
		ev.preventDefault();
		props.handler({
			q: input.current.value,
		});
	}

	return (
		<Form className={styles.Searchbar} onSubmit={handler}>
			<Input type="text" inputRef={input} placeholder="coredump search query" />
			<p><a href="https://blevesearch.com/docs/Query-String-Query/" target="_blank">query string reference</a></p>
		</Form>
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

function Form(props) {
	const {children, onSubmit, ...attributes} = props;

	function handler(ev) {
		ev.target.querySelectorAll('input').forEach(function(el) {
			el.dataset.saved = el.value;
			el.removeAttribute('dirty');
		});

		if (onSubmit !== undefined) {
			onSubmit(ev);
		}
	}

	return <form {...attributes} onSubmit={handler}>{children}</form>;
}

function Input(props) {
	const {inputRef, onChange, ...attributes} = props;

	function handler(ev) {
		if (ev.target.dataset.saved == ev.target.value) {
			ev.target.removeAttribute('dirty');
		} else {
			ev.target.setAttribute('dirty', '');
		}

		if (onChange !== undefined) {
			onChange(ev)
		};
	}

	return <input {...attributes} ref={inputRef} onChange={handler} />;
}

export default App;
