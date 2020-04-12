import React from 'react';
import ReactDOM from 'react-dom';
import {AppBoundary, App} from './App';
import './index.scss';

if (document.config === undefined) {
	document.config = {
		baseURL: window.location.origin,
	};
}

ReactDOM.render(<AppBoundary><App /></AppBoundary>, document.getElementById('root'));
