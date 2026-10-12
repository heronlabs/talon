package terminal

import "syscall"

// getTermios and setTermios are the ioctl requests that read and set the termios of a terminal on macOS.
const (
	getTermios uintptr = syscall.TIOCGETA
	setTermios uintptr = syscall.TIOCSETA
)
