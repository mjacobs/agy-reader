package contract

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed trajectory-sidecar.schema.json
var SchemaBytes []byte

var compiledSchema *jsonschema.Schema

const schemaResourceURL = "https://github.com/mjacobs/agy-reader/schema/trajectory-sidecar.schema.json"

func init() {
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaBytes))
	if err != nil {
		panic(fmt.Sprintf("internal/contract: unmarshal embedded schema: %v", err))
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaResourceURL, schemaDoc); err != nil {
		panic(fmt.Sprintf("internal/contract: add schema resource: %v", err))
	}
	sch, err := c.Compile(schemaResourceURL)
	if err != nil {
		panic(fmt.Sprintf("internal/contract: compile schema: %v", err))
	}
	compiledSchema = sch
}

// ValidateBytes validates raw trajectory JSON bytes against the sidecar JSON schema.
// Number-preserving decoding (json.Number) is used so numbers outside the float64
// range (such as 1e400) in unrestricted fields are preserved and validated.
func ValidateBytes(data []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}
	if err := compiledSchema.Validate(doc); err != nil {
		return fmt.Errorf("contract validation failed: %w", err)
	}
	return nil
}

// ValidateFile validates a trajectory JSON sidecar file against the schema.
func ValidateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := ValidateBytes(data); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

// ValidateFiles validates all given file paths or directory trees against the schema.
// Returns a slice of validation errors for each failed file.
func ValidateFiles(paths ...string) (passed int, failures []error) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", p, err))
			continue
		}
		if info.IsDir() {
			err := filepath.Walk(p, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !fi.IsDir() && strings.HasSuffix(path, ".trajectory.json") {
					if vErr := ValidateFile(path); vErr != nil {
						failures = append(failures, vErr)
					} else {
						passed++
					}
				}
				return nil
			})
			if err != nil {
				failures = append(failures, fmt.Errorf("walk %s: %w", p, err))
			}
		} else {
			if vErr := ValidateFile(p); vErr != nil {
				failures = append(failures, vErr)
			} else {
				passed++
			}
		}
	}
	return passed, failures
}
