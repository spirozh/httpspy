// Package main provides the command to start the server
package main

import (
	"os"

	app "www.github.com/spirozh/httpspy/internal/app"
)

func main() {
	addr, exists := os.LookupEnv("ADDR")
	if !exists {
		addr = ":6969"
	}

	app.Run(addr)
}
