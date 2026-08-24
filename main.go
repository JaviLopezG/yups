// Command yups prints the "#_?" marker and manages its own installation.
package main

import (
	"os"

	"yups/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
