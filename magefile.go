//go:build mage

package main

import (
	"os"
	"os/exec"
	"strings"
)

// Fixtures downloads the test fixtures listed in testdata/sources.tsv via scripts/fetch-fixtures.sh.
func Fixtures() error {
	return run("bash", "scripts/fetch-fixtures.sh")
}

// Parsebench runs the capture parser over the fixture corpus and reports which sections were extracted.
// Set PARSEBENCH_FLAGS to pass flags, e.g. PARSEBENCH_FLAGS="-extractor go-recipe -v" mage parsebench.
func Parsebench() error {
	args := append([]string{"run", "./cmd/parsebench"}, strings.Fields(os.Getenv("PARSEBENCH_FLAGS"))...)
	return run("go", args...)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
