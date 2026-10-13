package completeness

import (
	"context"
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// UnsupportedError is the profile's refusal of a source or of its projection:
// a file that is not a SQLite database, or one whose version, tables, columns,
// step format, indexing or persisted messages are outside Profile. Reading the
// same source again refuses it again, so only a reader with a newer profile
// can verify it.
//
// Every other error from ReadSnapshot says nothing about support: the writer
// holding a lock, a permission or I/O failure, or a canceled context can
// clear without any change to the reader. Missing, obsolete or malformed
// cached evidence is not a refusal either; see Snapshot.Refusal.
type UnsupportedError struct{ Err error }

func (e *UnsupportedError) Error() string { return e.Err.Error() }
func (e *UnsupportedError) Unwrap() error { return e.Err }

// IsUnsupported reports whether err carries the profile's refusal.
func IsUnsupported(err error) bool {
	var u *UnsupportedError
	return errors.As(err, &u)
}

// refuse marks err as the profile's refusal. The message is unchanged.
func refuse(err error) error {
	if err == nil || IsUnsupported(err) {
		return err
	}
	return &UnsupportedError{Err: err}
}

func refusal(msg string) error { return &UnsupportedError{Err: errors.New(msg)} }

// refuseValue marks an error from scanning a column value as a refusal: the
// row was read, and a value the profile's types cannot hold is content outside
// the profile. A SQLite error (a lock, an I/O failure) or a canceled read is
// not about the content, and is returned unchanged.
func refuseValue(ctx context.Context, err error) error {
	var driver *sqlite.Error
	if err == nil || ctx.Err() != nil || errors.As(err, &driver) {
		return err
	}
	return refuse(err)
}

// notADatabase reports SQLite's refusal of a file that is not a SQLite
// database at all, which no supported source can be.
func notADatabase(err error) bool {
	var driver *sqlite.Error
	return errors.As(err, &driver) && driver.Code()&0xff == sqlite3.SQLITE_NOTADB
}

// Refusal reports the profile's refusal of this snapshot that no daemon answer
// can change, as an *UnsupportedError, or nil when there is none: an empty
// persisted cascade identity, which a fetch needs to name the trajectory, or
// a supported daemon projection the profile refuses. A verified fetch refuses
// both before any sidecar is written.
//
// CheckAttestation returns Unknown both for such a snapshot and for missing,
// obsolete or malformed cached evidence. Only the first is a refusal; the
// second is repaired by verifying the source again and rewriting the sidecar.
func (s *Snapshot) Refusal() error {
	if s == nil || s.trajectory == nil {
		return nil
	}
	if s.CascadeID() == "" {
		return refusal("persisted cascade identity is empty")
	}
	_, err := project(s.trajectory)
	return refuse(err)
}
