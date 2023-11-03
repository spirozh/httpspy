// Package static provides static files which will be served by this app
package static

import (
	_ "embed"
)

// WatchFavicon is the favicon for the app
//
//go:embed "favicon.ico"
var WatchFavicon []byte

// WatchPage is the static html page for the app
//
//go:embed "watch.html"
var WatchPage []byte

// WatchCSS is the css for the app
//
//go:embed "watch.css"
var WatchCSS []byte

// WatchJS is the javascript for the app
//
//go:embed "watch.js"
var WatchJS []byte
