/*
 * Preload shim for `agy-pair.py --token-mode=shim`.
 *
 * The helper starts agy with a placeholder first argument
 * (--pair_token_slot=1), the token in ANTIGRAVITY_PAIR_TOKEN, agy's own path
 * in ANTIGRAVITY_PAIR_EXE, and this library in LD_PRELOAD. Before agy's Go
 * runtime reads its arguments, the constructor repoints that argv entry at a
 * heap copy of --csrf_token=<token>, then removes all three variables so
 * tools agy runs neither load the shim nor inherit the token.
 *
 * Act only in agy itself. Every process started with this environment loads
 * the library, including a wrapper script's interpreter or a launcher such as
 * a mise shim that then runs agy. If the swap happened there, the wrapper
 * would pass --csrf_token=<token> to agy as an exec argument, readable by
 * every local account. So the swap happens only when this process runs the
 * very file named in ANTIGRAVITY_PAIR_EXE, the one the helper checked. Any
 * other process gets no swap: the token is cleared and the variables
 * removed, so nothing it starts can receive the token, and agy, reached
 * through a wrapper, rejects the placeholder and exits with status 2.
 *
 * unsetenv changes only the live environment, not the original environment
 * block that /proc/<pid>/environ shows to the same user. There, after this
 * runs, the token's bytes are zeroed, but LD_PRELOAD (with this library's
 * path), ANTIGRAVITY_PAIR_EXE (with agy's path) and the emptied
 * ANTIGRAVITY_PAIR_TOKEN= name stay listed.
 *
 * The helper sets LD_PRELOAD to this library alone, so removing the variable
 * drops nothing else; an LD_PRELOAD the user already had never reaches agy.
 *
 * Never overwrite the argument bytes in place: /proc/<pid>/cmdline reads
 * them live from process memory, so other local accounts would see the
 * token again. Repointing argv[i] leaves the placeholder visible there.
 *
 * Fail closed: without a token, outside the checked executable, or if this
 * library does not load at all, agy sees the unknown placeholder flag and
 * exits with status 2.
 *
 * Linux and glibc only. Build with `make shim`.
 */
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>

#define SLOT "--pair_token_slot"
#define TOKEN_VAR "ANTIGRAVITY_PAIR_TOKEN"
#define EXE_VAR "ANTIGRAVITY_PAIR_EXE"

/* Whether this process runs the file the helper checked, compared by device
 * and inode so that paths, symlinks and hard links cannot confuse it. */
static int checked_executable(void) {
    const char *expected = getenv(EXE_VAR);
    struct stat self, checked;
    return expected && expected[0] && stat("/proc/self/exe", &self) == 0 &&
           stat(expected, &checked) == 0 && self.st_dev == checked.st_dev &&
           self.st_ino == checked.st_ino;
}

__attribute__((constructor)) static void agy_pair_shim(int argc, char **argv, char **envp) {
    (void)envp;
    char *token = getenv(TOKEN_VAR);
    if (token && token[0]) {
        if (checked_executable()) {
            for (int i = 1; i < argc; i++) {
                size_t n = strlen(SLOT);
                if (strncmp(argv[i], SLOT, n) == 0 && (argv[i][n] == '\0' || argv[i][n] == '=')) {
                    char *flag = NULL;
                    if (asprintf(&flag, "--csrf_token=%s", token) > 0) {
                        argv[i] = flag;
                    }
                    break;
                }
            }
        }
        /* The value lives in the original environment block, which the same
         * user can still read through /proc/<pid>/environ after unsetenv.
         * The argument, if swapped, holds its own copy, so clear these bytes.
         * Only the value goes: the variable's name stays in that block. */
        memset(token, 0, strlen(token));
    }
    unsetenv(TOKEN_VAR);
    unsetenv(EXE_VAR);
    unsetenv("LD_PRELOAD");
}
