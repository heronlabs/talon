package terminal

import "syscall"

// getTermios and setTermios are the ioctl requests that read and set the termios of a terminal on Linux.
const (
	getTermios uintptr = syscall.TCGETS
	setTermios uintptr = syscall.TCSETS
)
