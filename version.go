package main

import (
	"fmt"
	"runtime/debug"
)

// version is overridden at build time via:
//
//	go build -ldflags="-X main.version=v1.2.3"
var version = "dev"

func cmdVersion() {
	v := version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok &&
			info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	fmt.Printf("gossh %s\n", v)
}
