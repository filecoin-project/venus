package main

import (
	"fmt"
)

// Version is the current git commit, injected through ldflags.
var Version string

// PrintVersion returns the hello statemen including the current git version.
func PrintVersion() string {
	return fmt.Sprintf("Hello Filecoin: %s\n", Version)
}

func main() {
	fmt.Println(PrintVersion())
}
