import React from 'react';
import styles from './App.scss';
import dayjs from 'dayjs';


// Encore a query as string.
function encodeQuery(q) {
	return btoa(JSON.stringify(q));
}

// Decode the string version of a query.
function decodeQuery(q) {
	return JSON.parse(atob(q));
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


// AppBoundary is the error-catching component for the whole app.
export class AppBoundary extends React.Component {
	constructor(props) {
		super(props);
		this.state = {
			error: false,
		};
	}

	componentDidCatch(error, info) {
		this.setState({
			error: error,
			info: info,
		});
	}

	goback() {
		this.setState({error: false});
	}

	render() {
		if (this.state.error !== false) {
			return (
				<React.Fragment>
					<Header/>
					<h2>something went wrong</h2>
					<p>{ this.state.error.message }</p>
					<pre>{ this.state.info.componentStack.slice(1) }</pre>
					{ this.state.showStack
						? (
							<React.Fragment>
								<p><a href="#" onClick={() => this.setState({showStack: false})}>hide stack</a></p>
								<pre>{ this.state.error.stack }</pre>
							</React.Fragment>
						)
						: <p><a href="#" onClick={() => this.setState({showStack: true})}>show stack</a></p>
					}
					<p><a href="#" onClick={() => this.goback()}>go back</a></p>
					<Footer/>
				</React.Fragment>
			);
		}

		return this.props.children;
	}
}


export function App() {
	// query and results are the primary states of the whole app. Initial
	// value of the query is especially important as the query will be
	// runned immediately during the first render. We use a lazy load to
	// avoid doing the job on every render.
	const [query, setQuery] = React.useState(function() {
		const raw = new URLSearchParams(window.location.search).get('q');
		if (raw === null) {
			return {q: '*', sort: 'dumped_at', order: 'desc', size: '150'};
		}
		return decodeQuery(raw);
	});
	const [result, setResult] = React.useState({results: [], total: 0, error: null});

	// Whenever the query change, we want to run the query and update the
	// result. We don't check the return status yet, because in most
	// actionable cases we have an error message in the payload.
	React.useEffect(function() {
		let params = [];
		for (const name in query) {
			params.push(encodeURIComponent(name) + '=' + encodeURIComponent(query[name]));
		}
		fetch(`${document.config.baseURL}/cores?${params.join('&')}`)
			.then(function(res) {
				return res.json();
			})
			.then(function(res) {
				setResult(res);
			})
			.catch(function(e) {
				setResult({error: e.message});
			});
	}, [query]);

	// The popstate event notify of the user using the back button of his
	// browser (or other similar event). We don't really need to cleanup
	// because the App component should never be unmounted.
	React.useEffect(function() {
		window.addEventListener('popstate', function (event) {
			console.log('popstate');
			setQuery(decodeQuery(new URLSearchParams(window.location.search).get('q')));
		});
	}, []);

	// When the query change, we want to update the URL value. We have to
	// check for the current value despite the hook dependency on the query
	// because the any history event already does this, and doing it again
	// break the forward-history.
	React.useEffect(function(){
		const q = encodeQuery(query);
		if (new URLSearchParams(window.location.search).get('q') === q) {
			return;
		}
		history.pushState({}, '', `/?q=${q}`);
	}, [query]);

	// Finally, render the component itself. The header and searchbar are
	// always displayed, and the table gives way for the fallback display
	// in case of error.
	return (
		<React.Fragment>
			<Header/>
			<Searchbar setQuery={setQuery} query={query} />
			{ result.error == null
				? (
					<Table results={result.results || []} total={result.total} />
				)
				: (
					<React.Fragment>
						<h2>Unexpected error</h2>
						<p>{ result.error }</p>
					</React.Fragment>
				)
			}
			<Footer/>
		</React.Fragment>
	);
}

// Header is a separate component so it can be shared in the AppBoundary and in
// the App itself.
function Header() {
	return (
		<header className={styles.Header}>
			<h1>RCoredump <sup>{document.Version}</sup></h1>
		</header>
	);
}

function Footer() {
	return (
		<footer className={styles.Footer}>
			<p>For documentation, issues, see the <a href="https://github.com/elwinar/rcoredump">repository</a>.</p>
		</footer>
	)
}

function Searchbar(props) {
	// query and state are respectively the initial and current value of
	// the form. We use setQuery to warn the components above when the use
	// wants to apply the local state to the app.
	const {query, setQuery} = props;
	const [state, setState] = React.useState(query);

	// We want to update the current state when the query change. As the
	// searchbar is never unmounted, this isn't done automatically.
	React.useEffect(function() {
		setState(query);
	}, [query]);

	// dirty is used to activate or not the apply and reset buttons when
	// the state isn't equivalent to the initial query.
	const [dirty, setDirty] = React.useState(false);
	React.useEffect(function() {
		setDirty(Object.keys(query).some(prop => state[prop] !== query[prop]));
	}, [state]);

	// change is used by form component when their value change to update
	// the local state.
	function change(ev) {
		setState({
			...state,
			[ev.target.name]: ev.target.value,
		});
	}

	// submit is used by the apply button when it is clicked so we can
	// propagate the state to the parent component.
	function submit(ev) {
		ev.preventDefault();
		setQuery(state);
		setDirty(false);
		return false;
	}

	// reset is used by the reset button when it is clicked so we can reset
	// the state to the query value.
	function reset() {
		setState(query);
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
					<button onClick={reset} disabled={!dirty}>reset</button>
				</div>
				<div>
					<p><a href="https://blevesearch.com/docs/Query-String-Query/" target="_blank">query string reference</a></p>
				</div>
			</form>
		</React.Fragment>
	);
}

function Table(props) {
	// results and total are given by the search results. The length of the
	// results isn't expected to be equal to total, as the query is run
	// with a limit parameter and no actual API-based pagination is done.
	const {results, total} = props;

	// page and selected are used to control what gets displayed on screen,
	// either by limiting the number of elements or displaying the details
	// of a result.
	const [page, setPage] = React.useState(1);
	const [selected, setSelected] = React.useState(null);

	// If we don't have anything to display, fallback to a line saying so,
	// and a nice message. Query strings can be frustrating, and Bleve's
	// format is especially horendous.
	if (results.length == 0) {
		const quotes = [
			":-)",
			"Seems like good news.",
			"Do not fear failure, but rather fear not trying.",
		];
		return (
			<p className={styles.NoResult}>No match for this query. {quotes[Math.floor(Math.random() * quotes.length)]}</p>
		)
	}

	// Compute the page list by transforming a list of indices like [0, 1,
	// 2, 3, 4] by shifting them from an offset computed from the current
	// page (to avoid the "-1" page, and "max+1" page).
	// Special case if there is less than maxPages pages to display, in
	// which case we display them all.
	const maxPages = 5;
	const pageSize = 15;
	const totalPages = Math.ceil(results.length/pageSize);
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

	// Display both the pagination, the table, and the eventually selected
	// coredump details.
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
								<tr onClick={() => setSelected(selected == x.uid ? null : x.uid)} active={boolattr(selected == x.uid)}>
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

	// The component is a pure component that does nothing else than
	// extract a bunch of formatting details from the already non-trivial
	// Table component.
	return (
		<React.Fragment>
			<ul>
				<li><a className={styles.Button} href={`${document.config.baseURL}/cores/${core.uid}`}>download core ({formatSize(core.size, true)})</a></li>
				<li><a className={styles.Button} href={`${document.config.baseURL}/executables/${core.executable_hash}`}>download executable ({formatSize(core.executable_size, true)})</a></li>
			</ul>
			<h2>executable</h2>
			<dl>
				<dt>executable_hash</dt><dd><QueryLink query={`executable_hash:"${core.executable_hash}"`}>{core.executable_hash}</QueryLink></dd>
				<dt>executable_path</dt><dd>{core.executable_path}</dd>
			</dl>
			<h2>coredump</h2>
			<dl>
				<dt>uid</dt><dd><QueryLink query={`uid:"${core.uid}"`}>{core.uid}</QueryLink></dd>
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
			<h3>download & debug</h3>
			<pre>
				curl -s "{document.config.baseURL}/cores/{core.uid}" --output {core.executable}.{core.uid}<br/>
				curl -s "{document.config.baseURL}/executables/{core.executable_hash}" --output {core.executable}<br/>
				{core.lang == "C" && `gdb ${core.executable} ${core.executable}.${core.uid}`}
				{core.lang == "Go" && `dlv core ${core.executable} ${core.executable}.${core.uid}`}
			</pre>
		</React.Fragment>
	);
}

// QueryLink can be used to make a direct link to a query search. The link is a
// standard HTML link with a valid href, but the navigation is intercepted to
// be handled by the app. This allow the user to copy-paste the link via his
// navigator contextual menu, while making internal navigation easy.
function QueryLink(props) {
	const {query} = props;
	const href = `/?q=${encodeQuery({q: query})}`;

	function redirect(event) {
		event.preventDefault();
		history.pushState({}, '', href);
		window.dispatchEvent(new Event('popstate'));
		return false;
	}

	return <a href={href} onClick={redirect}>{props.children}</a>
}
