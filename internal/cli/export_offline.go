package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"github.com/mjacobs/agy-reader/internal/daemon"
	"github.com/mjacobs/agy-reader/internal/render"
)

const offlineMarkdownNotice = "> Offline SQLite reconstruction; not daemon-verified.\n\n"

// RunExportOffline reads one SQLite session without discovery, RPCs, or sidecars.
func RunExportOffline(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("export-offline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "md", "Output format: md or json")
	out := fs.String("out", "", "New output file (default stdout; existing paths refused)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: agy-reader export-offline [--format=md|json] [--out=NEW_FILE] SESSION.db")
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("export-offline requires one SQLite session DB path")
	}
	if *format != "md" && *format != "json" {
		return errors.New("export-offline format must be md or json")
	}
	path := fs.Arg(0)
	if !strings.EqualFold(filepath.Ext(path), ".db") {
		return errors.New("export-offline requires a .db SQLite session (legacy .pb is not supported)")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("export-offline source must be a regular file")
	}
	snapshot, err := completeness.ReadSnapshot(ctx, path)
	if err != nil {
		return fmt.Errorf("read offline source: %w", err)
	}
	data, err := snapshot.OfflineJSON()
	if err != nil {
		return err
	}
	if *format == "md" {
		var trajectory daemon.Trajectory
		if err = json.Unmarshal(data, &trajectory); err != nil {
			return fmt.Errorf("render offline trajectory: %w", err)
		}
		var buf bytes.Buffer
		buf.WriteString(offlineMarkdownNotice)
		if _, err = render.Markdown(&buf, &trajectory, time.Time{}); err != nil {
			return err
		}
		data = buf.Bytes()
	} else {
		data = append(data, '\n')
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if *out != "" {
		return writeNewExport(*out, bytes.NewReader(data))
	}
	_, err = io.Copy(stdout, bytes.NewReader(data))
	return err
}

// Create a new file without replacing any existing directory entry, including
// dangling symlinks. Encode and validate the transcript before calling this.
// Remove the new file on a reported write/close failure; process termination
// during a write can still leave a partial file.
func writeNewExport(path string, data io.Reader) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create new export: %w", err)
	}
	_, err = io.Copy(f, data)
	if err = CloseOutput(f, err); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}
