package httpspy

import (
	_ "embed"
)

//go:embed "static/favicon.ico"
var watchFavicon []byte

//go:embed "static/watch.html"
var watchPage []byte

//go:embed "static/watch.js"
var watchJs []byte

//go:embed "static/watch.css"
var watchCSS []byte
