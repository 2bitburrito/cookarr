//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Generate regenerates the typed database queries using the pinned sqlc version.
func Generate() error {
	return run("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0", "generate", "-f", "db/sqlc.yaml")
}

// Fixtures downloads the test fixtures listed in internal/capture/testdata/sources.tsv via scripts/fetch-fixtures.sh.
func Fixtures() error {
	return run("bash", "scripts/fetch-fixtures.sh")
}

// Parsebench runs the capture test harness over the fixture corpus.
// Set PARSEBENCH_FLAGS to pass harness flags, e.g. PARSEBENCH_FLAGS="-extractor ours -v" mage parsebench.
func Parsebench() error {
	return runVerbose("go", "test", "-run", "^$", "-bench", "^BenchmarkParseBench$", "./internal/capture")
}

func TestShort() error {
	return run("go", "test", "./...")
}

func Test() error {
	return run("go", "test", "./...", "--race")
}

func TestVerbose() error {
	return runVerbose("go", "test", "./...", "-v", "--race")
}

func Lint() error {
	return runVerbose("golangci-lint", "run")
}

func Build() error {
	return runVerbose("go", "build", "./cmd/cookarr/")
}

func BuildDocker() error {
	// TODO: Add this command when we have a dockerfile
	return nil
}

func run(name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runVerbose is a helper function to run a command with all outs and errs printed.
func runVerbose(name string, args ...string) error {
	fmt.Printf("%s %s\n", name, strings.Join(args, " "))
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
