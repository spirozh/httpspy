// Package main starts the server
package main

import "www.github.com/spirozh/httpspy"

func main() {
	httpspy.New(":6969").Run()
}
