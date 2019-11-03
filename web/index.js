import React from 'react';
import ReactDOM from 'react-dom';
import App from './App';
import './index.scss';

if (document.config === undefined) {
	document.config = {
		baseURL: window.location.origin,
	};
}


ReactDOM.render(<App />, document.getElementById('root'));
