package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/mjacobs/agy-reader/internal/contract"
)

// RunVerifyContract validates trajectory JSON sidecars against the formal Draft 2020-12 JSON Schema.
func RunVerifyContract(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("verify-contract", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var schemaFlag bool
	fs.BoolVar(&schemaFlag, "schema", false, "Print the JSON Schema (Draft 2020-12) to stdout and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if schemaFlag {
		_, err := stdout.Write(contract.SchemaBytes)
		return err
	}

	targets := fs.Args()
	if len(targets) == 0 {
		return fmt.Errorf("usage: agy-reader verify-contract [--schema] <FILE_OR_DIR...>")
	}

	passed, failures := contract.ValidateFiles(targets...)
	if len(failures) > 0 {
		for _, f := range failures {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", f)
		}
		return fmt.Errorf("contract verification failed: %d failed, %d passed", len(failures), passed)
	}

	if passed == 0 {
		return fmt.Errorf("no .trajectory.json sidecars found in specified path(s)")
	}

	_, err := fmt.Fprintf(stdout, "contract valid: %d sidecar(s) verified\n", passed)
	return err
}
