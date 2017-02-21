package plugin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"crypto/cipher"
	"github.com/mattn/go-shellwords"
	"github.com/starkandwayne/shield/crypter"
)

const NOPIPE = 0
const STDIN = 1
const STDOUT = 2

type ExecOptions struct {
	Stdout   io.Writer
	Stdin    io.Reader
	Stderr   *os.File
	Cmd      string
	ExpectRC []int
}

func ExecWithOptions(opts ExecOptions) error {
	cmdArgs, err := shellwords.Parse(opts.Cmd)
	if err != nil {
		return ExecFailure{Err: fmt.Sprintf("Could not parse '%s' into exec-able command: %s", opts.Cmd, err.Error())}
	}
	DEBUG("Executing '%s' with arguments %v", cmdArgs[0], cmdArgs[1:])

	// some liberties will be taken here.  hang on!
	keyRaw := "\xDE\xAD\xBE\xEF\xDE\xCA\xFB\xAD\xDE\xAD\xBE\xEF\xDE\xCA\xFB\xAD"
	ivRaw := "\xCA\xFE\xBA\xBE\xAB\xAD\x1D\xEA"
	ivRaw = keyRaw

	encStream, decStream, err := crypter.Stream("twofish-cfb", keyRaw, ivRaw)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	if opts.Stdout != nil {
		cmd.Stdout = cipher.StreamWriter{
			S: encStream,
			W: opts.Stdout,
		}
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
		cmd.Stdin = cipher.StreamReader{
			S: decStream,
			R: opts.Stdin,
		}
	}

	if len(opts.ExpectRC) == 0 {
		opts.ExpectRC = []int{0}
	}

	err = cmd.Run()
	if err != nil {
		// make sure we got an Exit error
		if exitErr, ok := err.(*exec.ExitError); ok {
			sys := exitErr.ProcessState.Sys()
			// os.ProcessState.Sys() may not return syscall.WaitStatus on non-UNIX machines,
			// so currently this feature only works on UNIX, but shouldn't crash on other OSes
			if rc, ok := sys.(syscall.WaitStatus); ok {
				code := rc.ExitStatus()
				// -1 indicates signals, stops, or traps, so force an error
				if code >= 0 {
					for _, expect := range opts.ExpectRC {
						if code == expect {
							return nil
						}
					}
				}
			}
		}
		return ExecFailure{Err: fmt.Sprintf("Unable to exec '%s': %s", cmdArgs[0], err.Error())}
	}
	return nil
}

func Exec(cmdString string, flags int) error {
	opts := ExecOptions{
		Cmd:    cmdString,
		Stderr: os.Stderr,
	}

	if flags&STDOUT == STDOUT {
		opts.Stdout = os.Stdout
	}
	if flags&STDIN == STDIN {
		opts.Stdin = os.Stdin
	}
	return ExecWithOptions(opts)
}
