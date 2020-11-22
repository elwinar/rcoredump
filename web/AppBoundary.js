import React from "react";
import Header from "./Header.js";
import Footer from "./Footer.js";

// AppBoundary is the error-catching component for the whole app.
export default class AppBoundary extends React.Component {
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

  render() {
    if (this.state.error !== false) {
      return (
        <React.Fragment>
          <Header />
          <h2>something went wrong</h2>
          <p>{this.state.error.message}</p>
          <pre>{this.state.info.componentStack.slice(1)}</pre>
          {this.state.showStack ? (
            <React.Fragment>
              <p>
                <a href="#" onClick={() => this.setState({ showStack: false })}>
                  hide stack
                </a>
              </p>
              <pre>{this.state.error.stack}</pre>
            </React.Fragment>
          ) : (
            <p>
              <a href="#" onClick={() => this.setState({ showStack: true })}>
                show stack
              </a>
            </p>
          )}
          <p>
            <a href="#" onClick={() => this.setState({ error: false })}>
              go back
            </a>
          </p>
          <Footer />
        </React.Fragment>
      );
    }

    return this.props.children;
  }
}
