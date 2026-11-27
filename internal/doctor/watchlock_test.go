package doctor

import "testing"

// flockLine is what the kernel prints in /proc/<pid>/fdinfo/<fd> for an open
// file that carries an exclusive flock.
const flockLine = "lock:\t1: FLOCK  ADVISORY  WRITE 4242 00:24:985401497 0 EOF\n"

const fdinfoBase = "pos:\t0\nflags:\t0100002\nmnt_id:\t168\nino:\t985401497\n"

func TestFdinfoHoldsFlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		info string
		want bool
	}{
		{"exclusive flock", fdinfoBase + flockLine, true},
		{"no lock line", fdinfoBase, false},
		{"empty", "", false},
		{"two locks, the second exclusive", fdinfoBase + "lock:\t1: FLOCK  ADVISORY  READ 1 00:24:1 0 EOF\nlock:\t2: FLOCK  ADVISORY  WRITE 2 00:24:1 0 EOF\n", true},
		{"a waiter", fdinfoBase + "lock:\t1: -> FLOCK  ADVISORY  WRITE 1 00:24:1 0 EOF\n", false},
		{"a lease", fdinfoBase + "lock:\t1: LEASE  ACTIVE    WRITE 1 00:24:1 0 EOF\n", false},
	} {
		if got := fdinfoHoldsFlock([]byte(tc.info)); got != tc.want {
			t.Errorf("%s: fdinfoHoldsFlock = %v, want %v", tc.name, got, tc.want)
		}
	}
}
