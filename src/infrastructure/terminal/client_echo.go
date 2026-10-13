package terminal

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

// EchoClient turns the echo of the terminal on a file descriptor off and back on, through the termios ioctls.
type EchoClient struct {
	fd uintptr
}

// NewEchoClient returns an EchoClient on the terminal of fd.
func NewEchoClient(fd uintptr) *EchoClient {
	return &EchoClient{fd: fd}
}

// Off turns the echo off and returns the function that turns it back on and true; until that function runs, SIGINT and SIGTERM turn the echo back on before they end the process. When fd is not a terminal its termios cannot be read, and Off changes nothing and returns a function that does nothing and false.
func (c *EchoClient) Off() (func() error, bool, error) {
	saved := syscall.Termios{}
	if err := c.ioctl(getTermios, &saved); err != nil {
		return func() error { return nil }, false, nil
	}

	stop := c.restoreOnSignal(&saved)
	if err := c.hide(saved); err != nil {
		stop()
		return nil, false, err
	}

	return func() error { return c.restore(&saved, stop) }, true, nil
}

// hide sets termios without ECHO on the fd of c, keeping ICANON and ISIG, so the line is still edited and Ctrl+C still interrupts.
func (c *EchoClient) hide(termios syscall.Termios) error {
	termios.Lflag &^= syscall.ECHO

	return c.ioctl(setTermios, &termios)
}

// restore sets saved back on the fd of c and then stops the relay of the signals with stop.
func (c *EchoClient) restore(saved *syscall.Termios, stop func()) error {
	err := c.ioctl(setTermios, saved)
	stop()

	return err
}

// restoreOnSignal relays SIGINT and SIGTERM to relay until the returned function runs, which stops it.
func (c *EchoClient) restoreOnSignal(saved *syscall.Termios) func() {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go c.relay(signals, done, saved)

	return func() {
		signal.Stop(signals)
		close(done)
	}
}

// relay waits for a signal on signals or for done to close; on a signal it sets saved back on the fd of c, stops the relay and raises the signal again, which now ends the process.
func (c *EchoClient) relay(signals chan os.Signal, done <-chan struct{}, saved *syscall.Termios) {
	select {
	case sig := <-signals:
		_ = c.ioctl(setTermios, saved)
		signal.Stop(signals)
		_ = syscall.Kill(syscall.Getpid(), sig.(syscall.Signal))
	case <-done:
	}
}

// ioctl runs the termios request on the fd of c with termios and returns its errno, nil on success.
func (c *EchoClient) ioctl(request uintptr, termios *syscall.Termios) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, c.fd, request, uintptr(unsafe.Pointer(termios))); errno != 0 { //nolint:gosec // G103: the ioctl reads or writes the termios at its address
		return errno
	}

	return nil
}
