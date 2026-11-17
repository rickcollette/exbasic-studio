package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"os"

	"exbasicstudio/internal/app"
	"exbasicstudio/internal/selftest"
	"exbasicstudio/internal/version"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	args := os.Args[1:]
	switch {
	case len(args) > 0 && args[0] == "--version":
		fmt.Println(version.Version)
		return
	case len(args) > 0 && args[0] == "--self-test":
		os.Exit(selfTest(args[1:]))
	}
	if err := app.Run(assets); err != nil {
		log.Fatal(err)
	}
}

// lenient ignores write failures: a Windows GUI executable has no usable
// standard output, and that must not stop the report file being written.
type lenient struct{ w io.Writer }

func (l lenient) Write(p []byte) (int, error) {
	_, _ = l.w.Write(p)
	return len(p), nil
}

// selfTest verifies the installation without opening a window. A GUI build
// on Windows has no console, so the report can also be written to a file:
//
//	exbasic-studio --self-test [report-file]
func selfTest(args []string) int {
	var out io.Writer = lenient{os.Stdout}
	if len(args) > 0 {
		file, err := os.Create(args[0])
		if err != nil {
			return 2
		}
		defer file.Close()
		out = io.MultiWriter(file, lenient{os.Stdout})
	}
	if err := selftest.Run(out); err != nil {
		fmt.Fprintln(out, "FAIL "+err.Error())
		return 1
	}
	return 0
}
