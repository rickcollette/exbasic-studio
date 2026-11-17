//go:build windows

package terminal

import (
	"errors"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// conPTY hosts a shell on a Windows pseudo console. The shell is placed in a
// job object so closing the terminal ends every process it started.
type conPTY struct {
	console    windows.Handle
	input      *os.File // our write end of the console's input pipe
	output     *os.File // our read end of the console's output pipe
	process    windows.Handle
	job        windows.Handle
	closeOnce  sync.Once
	waitOnce   sync.Once
	outputOnce sync.Once
	mu         sync.Mutex
	closed     bool
	exitCode   int
}

func startPTY(options Options) (pty, error) {
	var inRead, inWrite, outRead, outWrite windows.Handle
	if err := windows.CreatePipe(&inRead, &inWrite, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outRead, &outWrite, nil, 0); err != nil {
		windows.CloseHandle(inRead)
		windows.CloseHandle(inWrite)
		return nil, err
	}
	closeAll := func(handles ...windows.Handle) {
		for _, handle := range handles {
			if handle != 0 {
				windows.CloseHandle(handle)
			}
		}
	}

	var console windows.Handle
	size := windows.Coord{X: int16(options.Cols), Y: int16(options.Rows)}
	if err := windows.CreatePseudoConsole(size, inRead, outWrite, 0, &console); err != nil {
		closeAll(inRead, inWrite, outRead, outWrite)
		return nil, err
	}
	// The pseudo console duplicated its ends; ours are no longer needed.
	closeAll(inRead, outWrite)

	fail := func(err error, extra ...windows.Handle) (pty, error) {
		windows.ClosePseudoConsole(console)
		closeAll(append(extra, inWrite, outRead)...)
		return nil, err
	}

	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return fail(err)
	}
	defer attributes.Delete()
	// The attribute value is the console handle itself, passed as a pointer.
	consoleValue := *(*unsafe.Pointer)(unsafe.Pointer(&console))
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, consoleValue, unsafe.Sizeof(console)); err != nil {
		return fail(err)
	}
	startup := windows.StartupInfoEx{ProcThreadAttributeList: attributes.List()}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	// Without this flag a child of a process whose own standard handles are
	// redirected writes to those handles instead of the pseudo console.
	startup.Flags |= windows.STARTF_USESTDHANDLES

	line, err := CommandLine(options.Shell, options.Args)
	if err != nil {
		return fail(err)
	}
	commandLine, err := windows.UTF16PtrFromString(line)
	if err != nil {
		return fail(err)
	}
	directory, err := windows.UTF16PtrFromString(options.Dir)
	if err != nil {
		return fail(err)
	}
	environment, err := environmentBlock(os.Environ())
	if err != nil {
		return fail(err)
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fail(err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return fail(err, job)
	}

	var info windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_SUSPENDED)
	if err = windows.CreateProcess(nil, commandLine, nil, nil, false, flags, &environment[0], directory, &startup.StartupInfo, &info); err != nil {
		return fail(err, job)
	}
	// Assign before the first instruction runs so no child can escape the job.
	if err = windows.AssignProcessToJobObject(job, info.Process); err != nil {
		windows.TerminateProcess(info.Process, 1)
		closeAll(info.Thread)
		return fail(err, job, info.Process)
	}
	if _, err = windows.ResumeThread(info.Thread); err != nil {
		windows.TerminateProcess(info.Process, 1)
		closeAll(info.Thread)
		return fail(err, job, info.Process)
	}
	closeAll(info.Thread)

	device := &conPTY{
		console: console,
		input:   os.NewFile(uintptr(inWrite), "conpty-input"),
		output:  os.NewFile(uintptr(outRead), "conpty-output"),
		process: info.Process,
		job:     job,
	}
	// A pseudo console keeps its output pipe open after the shell exits, so
	// the reader would never see end-of-file. Release it when the shell ends.
	go func() {
		device.Wait()
		_ = device.Close()
	}()
	return device, nil
}

// environmentBlock builds the double-NUL terminated UTF-16 block expected by
// CreateProcess.
func environmentBlock(env []string) ([]uint16, error) {
	var block []uint16
	for _, entry := range env {
		encoded, err := windows.UTF16FromString(entry)
		if err != nil {
			return nil, err
		}
		block = append(block, encoded...)
	}
	if len(block) == 0 {
		block = append(block, 0)
	}
	return append(block, 0), nil
}

// Read returns console output. The pipe is released on the first error,
// which is the end-of-file that follows ClosePseudoConsole.
func (c *conPTY) Read(p []byte) (int, error) {
	n, err := c.output.Read(p)
	if err != nil {
		c.outputOnce.Do(func() { _ = c.output.Close() })
	}
	return n, err
}

func (c *conPTY) Write(p []byte) (int, error) { return c.input.Write(p) }

func (c *conPTY) Resize(cols, rows int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("terminal is closed")
	}
	return windows.ResizePseudoConsole(c.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (c *conPTY) Wait() int {
	c.waitOnce.Do(func() {
		_, _ = windows.WaitForSingleObject(c.process, windows.INFINITE)
		var code uint32
		if err := windows.GetExitCodeProcess(c.process, &code); err == nil {
			c.exitCode = int(int32(code))
		}
	})
	return c.exitCode
}

// Close ends the job (the shell and its children), then releases the pseudo
// console, which makes the output pipe report end-of-file to the reader.
func (c *conPTY) Close() error {
	c.closeOnce.Do(func() {
		_ = windows.TerminateJobObject(c.job, 1)
		c.Wait()
		_ = c.input.Close()
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		// Flushes pending output and closes the console's end of the pipe;
		// the reader then drains what is left and sees end-of-file.
		windows.ClosePseudoConsole(c.console)
		windows.CloseHandle(c.process)
		windows.CloseHandle(c.job)
	})
	return nil
}
