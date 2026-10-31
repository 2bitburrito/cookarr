//go:build mage

package main

import (
	"os"
	"os/exec"
	"strings"
)

// Fixtures downloads the test fixtures listed in internal/capture/testdata/sources.tsv via scripts/fetch-fixtures.sh.
func Fixtures() error {
	return run("bash", "scripts/fetch-fixtures.sh")
}

// Parsebench runs the capture test harness over the fixture corpus.
// Set PARSEBENCH_FLAGS to pass harness flags, e.g. PARSEBENCH_FLAGS="-extractor ours -v" mage parsebench.
func Parsebench() error {
	cmd := exec.Command("go", "test", "-v", "-run", "^TestParseBench$", "./internal/capture")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "PARSEBENCH_FLAGS="+strings.Join(strings.Fields(os.Getenv("PARSEBENCH_FLAGS")), " "))
	return cmd.Run()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
