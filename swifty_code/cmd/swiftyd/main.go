package main

import (
	"fmt"
	"os"

	"github.com/hangtiancheng/swifty.go/swifty_code/internal/app"
)

func main() {
	coreApp := app.NewCoreApp()
	if err := coreApp.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "swiftyd: %s\n", err)
		os.Exit(1)
	}
}
