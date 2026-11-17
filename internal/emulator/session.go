package emulator

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	processutil "exbasicstudio/internal/process"
)

// ProtocolVersion is the helper protocol this release speaks.
const ProtocolVersion = 2

// Protocol limits. Requests and responses are single JSON lines; both sides
// bound them so a faulty peer cannot exhaust memory.
const (
	maxResponseBytes = 4 << 20
	// MaxMemoryRead is the largest memory block one request may return.
	MaxMemoryRead   = 1024
	maxAddressChunk = 2000
	// MaxAddresses bounds a breakpoint or line-start table.
	MaxAddresses     = 20000
	handshakeTimeout = 15 * time.Second
)

type Frame struct {
	Running          bool        `json:"running"`
	Image            string      `json:"image,omitempty"`
	Number           int         `json:"number,omitempty"`
	HostDevice       bool        `json:"hostDevice"`
	HostDeviceName   string      `json:"hostDeviceName,omitempty"`
	WorkspaceChanged bool        `json:"workspaceChanged,omitempty"`
	Error            string      `json:"error,omitempty"`
	Debug            *DebugState `json:"debug,omitempty"`
}

// DebugState is the CPU and debugger state reported by the helper.
// Location is the code address being executed: the bytecode pointer when
// the interpreter is about to run a bytecode instruction (Bytecode true),
// otherwise the program counter.
type DebugState struct {
	State    string `json:"state"` // running | stopped
	Reason   string `json:"reason"`
	PC       int    `json:"pc"`
	A        int    `json:"a"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	S        int    `json:"s"`
	P        int    `json:"p"`
	Location int    `json:"location"`
	Bytecode bool   `json:"bytecode"`
}

// Stopped reports whether execution is halted in the debugger.
func (d *DebugState) Stopped() bool { return d != nil && d.State == "stopped" }

type InputState struct {
	KeyChar int  `json:"keychar"`
	KeyCode int  `json:"keycode"`
	Shift   bool `json:"shift"`
	Control bool `json:"control"`
}

type Launch struct {
	Helper         string `json:"helper"`
	Artifact       string `json:"artifact"`
	EOSCar         string `json:"eosCar"`
	HostDir        string `json:"hostDir"`
	HostTool       string `json:"hostTool,omitempty"`
	HostDisk       string `json:"hostDisk,omitempty"`
	HostDeviceName string `json:"hostDeviceName,omitempty"`
	// Cart and Cart2 are cartridge images for the primary and secondary
	// slots, with their Atari800 type numbers (zero: decided by Atari800).
	// The EOS cartridge in the primary slot is passed in EOSCar instead.
	Cart      string `json:"cart,omitempty"`
	CartType  int    `json:"cartType,omitempty"`
	Cart2     string `json:"cart2,omitempty"`
	Cart2Type int    `json:"cart2Type,omitempty"`
	// Program is the name of the executable on the disk in D1: when the
	// build was put on a disk for a DOS to load, instead of being loaded
	// directly.
	Program string  `json:"program,omitempty"`
	Profile Profile `json:"profile"`
}

type State struct {
	Running        bool   `json:"running"`
	ProfileID      string `json:"profileId"`
	Artifact       string `json:"artifact"`
	HostDeviceName string `json:"hostDeviceName,omitempty"`
}

// Hello is the helper's answer to the capability handshake.
type Hello struct {
	Protocol     int      `json:"protocol"`
	Helper       string   `json:"helper"`
	Core         string   `json:"core"`
	Capabilities []string `json:"capabilities"`
}

type response struct {
	ID         int         `json:"id"`
	OK         bool        `json:"ok"`
	Frame      string      `json:"frame"`
	Pixels     string      `json:"pixels"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	Number     int         `json:"number"`
	HostDevice bool        `json:"hostDevice"`
	Error      string      `json:"error"`
	KeyChar    int         `json:"keychar"`
	KeyCode    int         `json:"keycode"`
	Shift      int         `json:"shift"`
	Control    int         `json:"control"`
	Debug      *DebugState `json:"debug"`
	Data       string      `json:"data"`
	Address    int         `json:"address"`
	Hello
}

type Session struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	reader         *bufio.Reader
	stderr         bytes.Buffer
	nextID         int
	closed         atomic.Bool
	hostDeviceName string
	hello          Hello
}

// launchArgs converts a launch description to helper arguments.
func launchArgs(launch Launch) []string {
	args := []string{"-machine", launch.Profile.Machine, "-video", launch.Profile.Video}
	args = append(args,
		"-memory", launch.Profile.Memory,
		"-cpu-speed", fmt.Sprint(launch.Profile.CPUSpeed),
		"-u1mb", fmt.Sprint(launch.Profile.Ultimate1MB),
		"-basic", fmt.Sprint(launch.Profile.Basic),
		"-sio-patch", fmt.Sprint(launch.Profile.SIOPatch),
	)
	for i, drive := range launch.Profile.Drives {
		if i >= 4 || !drive.Enabled || drive.Image == "" {
			continue
		}
		args = append(args, fmt.Sprintf("-drive%d", i+1), drive.Image)
		if drive.ReadOnly {
			args = append(args, fmt.Sprintf("-drive%d-readonly", i+1))
		}
	}
	if launch.HostDisk != "" && launch.HostDeviceName == "D8:" {
		args = append(args, "-drive8", launch.HostDisk)
	}
	if launch.Profile.RDevice.Mode != "" && launch.Profile.RDevice.Mode != "disabled" {
		args = append(args,
			"-rdevice-mode", launch.Profile.RDevice.Mode,
			"-rdevice-port", fmt.Sprint(launch.Profile.RDevice.ListenPort),
			"-rdevice-baud", fmt.Sprint(launch.Profile.RDevice.ConnectBaud),
			"-rdevice-modem", fmt.Sprint(launch.Profile.RDevice.Modem),
		)
		if launch.Profile.RDevice.SerialPort != "" {
			args = append(args, "-rdevice-serial", launch.Profile.RDevice.SerialPort)
		}
	}
	if launch.Profile.IDE.Enabled {
		args = append(args, "-ide", launch.Profile.IDE.Image)
		if launch.Profile.IDE.CompactFlash {
			args = append(args, "-ide-cf")
		}
	}
	if launch.Artifact != "" {
		args = append(args, "-artifact", launch.Artifact)
	}
	if launch.EOSCar != "" {
		args = append(args, "-eos", launch.EOSCar)
	} else if launch.Cart != "" {
		args = append(args, "-cart", launch.Cart)
		if launch.CartType > 0 {
			args = append(args, "-cart-type", fmt.Sprint(launch.CartType))
		}
	}
	if launch.Cart2 != "" {
		args = append(args, "-cart2", launch.Cart2)
		if launch.Cart2Type > 0 {
			args = append(args, "-cart2-type", fmt.Sprint(launch.Cart2Type))
		}
	}
	if launch.HostDir != "" {
		args = append(args, "-host-dir", launch.HostDir)
	}
	for _, arg := range launch.Profile.Arguments {
		args = append(args, "-arg", arg)
	}
	return args
}

// Start launches the helper and performs the protocol handshake.
func Start(launch Launch) (*Session, error) {
	cmd := exec.Command(launch.Helper, launchArgs(launch)...)
	processutil.Background(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	s := &Session{cmd: cmd, stdin: stdin, reader: bufio.NewReaderSize(stdout, 64<<10), hostDeviceName: launch.HostDeviceName}
	cmd.Stderr = &s.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start embedded Atari800: %w", err)
	}
	if err := s.handshake(); err != nil {
		_ = s.Stop()
		return nil, err
	}
	return s, nil
}

// handshake verifies that the helper speaks this protocol version. A helper
// that never answers (a different program configured by mistake) is ended
// after a timeout.
func (s *Session) handshake() error {
	type result struct {
		res response
		err error
	}
	done := make(chan result, 1)
	go func() {
		res, err := s.call(map[string]any{"op": "hello", "protocol": ProtocolVersion})
		done <- result{res, err}
	}()
	select {
	case outcome := <-done:
		if outcome.err != nil {
			return fmt.Errorf("the Atari800 helper is not compatible with this version of exBASIC Studio (protocol %d required): %w", ProtocolVersion, outcome.err)
		}
		if outcome.res.Protocol != ProtocolVersion {
			return fmt.Errorf("the Atari800 helper speaks protocol %d; protocol %d is required", outcome.res.Protocol, ProtocolVersion)
		}
		s.hello = outcome.res.Hello
		return nil
	case <-time.After(handshakeTimeout):
		_ = s.cmd.Process.Kill()
		return errors.New("the Atari800 helper did not answer the protocol handshake")
	}
}

// Hello returns the helper's handshake answer.
func (s *Session) Hello() Hello { return s.hello }

// readLine reads one response line, refusing oversized ones.
func (s *Session) readLine() ([]byte, error) {
	var line []byte
	for {
		chunk, err := s.reader.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > maxResponseBytes {
			return nil, errors.New("the Atari800 helper sent an oversized response")
		}
		if err == nil {
			return line, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func (s *Session) call(payload map[string]any) (response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return response{}, errors.New("Atari800 session is stopped")
	}
	s.nextID++
	payload["id"] = s.nextID
	encoded, err := json.Marshal(payload)
	if err != nil {
		return response{}, err
	}
	if _, err = s.stdin.Write(append(encoded, '\n')); err != nil {
		return response{}, s.failure(fmt.Errorf("write Atari800 command: %w", err))
	}
	for {
		line, readErr := s.readLine()
		if readErr != nil {
			return response{}, s.failure(readErr)
		}
		var res response
		if json.Unmarshal(line, &res) != nil || res.ID != s.nextID {
			// Not the answer to this request (the core may print to stdout).
			continue
		}
		if !res.OK {
			if res.Error == "" {
				res.Error = "the Atari800 helper rejected the request"
			}
			return res, errors.New(res.Error)
		}
		return res, nil
	}
}

// failure describes why the helper stopped answering.
func (s *Session) failure(cause error) error {
	message := bytes.TrimSpace(s.stderr.Bytes())
	if len(message) > 2000 {
		message = message[len(message)-2000:]
	}
	if len(message) > 0 {
		return fmt.Errorf("emulator helper stopped: %s", message)
	}
	if errors.Is(cause, io.EOF) {
		return errors.New("emulator helper stopped")
	}
	return fmt.Errorf("emulator helper stopped: %w", cause)
}

func (s *Session) Frame() (Frame, error) {
	res, err := s.call(map[string]any{"op": "frame"})
	if err != nil {
		return Frame{Error: err.Error()}, err
	}
	if res.Pixels != "" {
		raw, decodeErr := base64.StdEncoding.DecodeString(res.Pixels)
		if decodeErr != nil {
			return Frame{}, fmt.Errorf("decode Atari800 pixels: %w", decodeErr)
		}
		if res.Width <= 0 || res.Height <= 0 || res.Width > 1024 || res.Height > 1024 || len(raw) != res.Width*res.Height*3 {
			return Frame{}, errors.New("Atari800 returned an invalid frame buffer")
		}
		frame := image.NewRGBA(image.Rect(0, 0, res.Width, res.Height))
		for src, dst := 0, 0; src < len(raw); src, dst = src+3, dst+4 {
			copy(frame.Pix[dst:dst+3], raw[src:src+3])
			frame.Pix[dst+3] = 255
		}
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, frame); err != nil {
			return Frame{}, err
		}
		res.Frame = "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
	}
	hostDevice := res.HostDevice || (s.hostDeviceName != "" && s.hostDeviceName != "H1:")
	hostDeviceName := ""
	if hostDevice {
		hostDeviceName = s.hostDeviceName
		if hostDeviceName == "" {
			hostDeviceName = "H1:"
		}
	}
	return Frame{Running: true, Image: res.Frame, Number: res.Number, HostDevice: hostDevice, HostDeviceName: hostDeviceName, Debug: res.Debug}, nil
}

func (s *Session) Key(key string, down bool) error {
	_, err := s.KeyState(key, down)
	return err
}

func (s *Session) KeyState(key string, down bool) (InputState, error) {
	res, err := s.call(map[string]any{"op": "key", "key": key, "down": down})
	return InputState{KeyChar: res.KeyChar, KeyCode: res.KeyCode, Shift: res.Shift != 0, Control: res.Control != 0}, err
}

// JoystickNeutral is the STICK() value of a centred joystick.
const JoystickNeutral = 15

// Joystick sets joystick 0. stick is the Atari STICK() value: 15 when
// centred, with a 0 bit for each pressed direction (1 up, 2 down, 4 left,
// 8 right). trigger is true while fire is held (STRIG() returns 0).
func (s *Session) Joystick(stick int, trigger bool) error {
	if stick < 0 || stick > 15 {
		return fmt.Errorf("invalid joystick value %d", stick)
	}
	_, err := s.call(map[string]any{"op": "joystick", "stick": stick, "trigger": trigger})
	return err
}

func (s *Session) ReleaseKeys() error {
	_, err := s.call(map[string]any{"op": "releaseKeys"})
	return err
}

// DismountHostDisk closes D8 before Studio rebuilds the EOS project disk.
// Keeping this in the helper protocol avoids replacing an image while the
// emulator still has it open, which is especially important on Windows.
func (s *Session) DismountHostDisk() error {
	_, err := s.call(map[string]any{"op": "dismountD8"})
	return err
}

// MountHostDisk reopens the configured D8 image after Studio has refreshed it.
func (s *Session) MountHostDisk() error {
	_, err := s.call(map[string]any{"op": "mountD8"})
	return err
}

// ---- Debugger ---------------------------------------------------------------

// DebugState returns the CPU registers and whether execution is stopped.
func (s *Session) DebugState() (DebugState, error) {
	res, err := s.call(map[string]any{"op": "state"})
	if err != nil {
		return DebugState{}, err
	}
	if res.Debug == nil {
		return DebugState{}, errors.New("the Atari800 helper returned no debugger state")
	}
	return *res.Debug, nil
}

// SetInterpreter tells the helper where the bytecode interpreter dispatches
// instructions, so bytecode addresses can be matched like code addresses.
func (s *Session) SetInterpreter(dispatch, pointer int) error {
	_, err := s.call(map[string]any{"op": "setInterpreter", "dispatch": dispatch, "pointer": pointer})
	return err
}

func (s *Session) setAddresses(op string, addresses []int) error {
	if len(addresses) > MaxAddresses {
		return fmt.Errorf("too many addresses (%d, limit %d)", len(addresses), MaxAddresses)
	}
	for _, address := range addresses {
		if address < 0 || address > 0xFFFF {
			return fmt.Errorf("address %d is out of range", address)
		}
	}
	if addresses == nil {
		addresses = []int{}
	}
	// Large tables are sent in chunks so every request stays bounded.
	mode := "replace"
	for {
		chunk := addresses
		if len(chunk) > maxAddressChunk {
			chunk = chunk[:maxAddressChunk]
		}
		if _, err := s.call(map[string]any{"op": op, "mode": mode, "addresses": chunk}); err != nil {
			return err
		}
		addresses = addresses[len(chunk):]
		if len(addresses) == 0 {
			return nil
		}
		mode = "append"
	}
}

// SetBreakpoints replaces the breakpoint table.
func (s *Session) SetBreakpoints(addresses []int) error {
	return s.setAddresses("setBreakpoints", addresses)
}

// SetLineStarts replaces the addresses where source lines begin; they are
// the stops of a source-line step.
func (s *Session) SetLineStarts(addresses []int) error {
	return s.setAddresses("setLines", addresses)
}

// Pause requests a stop at the next instruction; it takes effect on the
// following frame.
func (s *Session) Pause() error {
	_, err := s.call(map[string]any{"op": "pause"})
	return err
}

// Resume continues a stopped machine with one of: continue,
// stepInstruction, stepLine, stepOver, stepOut.
func (s *Session) Resume(command string) error {
	switch command {
	case "continue", "stepInstruction", "stepLine", "stepOver", "stepOut":
	default:
		return fmt.Errorf("unknown debugger command %q", command)
	}
	_, err := s.call(map[string]any{"op": command})
	return err
}

// RunUntil continues until execution reaches a code address.
func (s *Session) RunUntil(address int) error {
	if address < 0 || address > 0xFFFF {
		return fmt.Errorf("address %d is out of range", address)
	}
	_, err := s.call(map[string]any{"op": "runUntil", "address": address})
	return err
}

// ReadMemory returns up to MaxMemoryRead bytes without triggering hardware
// register side effects.
func (s *Session) ReadMemory(address, length int) ([]byte, error) {
	if address < 0 || address > 0xFFFF || length < 1 || length > MaxMemoryRead || address+length > 0x10000 {
		return nil, fmt.Errorf("invalid memory range %d+%d", address, length)
	}
	res, err := s.call(map[string]any{"op": "readMemory", "address": address, "length": length})
	if err != nil {
		return nil, err
	}
	data, err := hex.DecodeString(res.Data)
	if err != nil || len(data) != length || res.Address != address {
		return nil, errors.New("the Atari800 helper returned an invalid memory block")
	}
	return data, nil
}

// Stop ends the helper. It asks politely first and kills a helper that does
// not exit, so no child process is ever left behind.
func (s *Session) Stop() error {
	if s.closed.Swap(true) {
		return nil
	}
	if s.mu.TryLock() {
		payload, _ := json.Marshal(map[string]any{"id": s.nextID + 1, "op": "stop"})
		_, _ = s.stdin.Write(append(payload, '\n'))
		_ = s.stdin.Close()
		s.mu.Unlock()
	} else {
		// A request is still waiting for an answer: the helper is busy or
		// stuck, so it cannot be asked to exit.
		_ = s.cmd.Process.Kill()
	}
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
		return errors.New("the Atari800 helper did not exit and was terminated")
	}
}

// ScreenText returns the top rows of a 40-column text screen as ASCII, one
// row per line. It reads the screen memory the OS points to; in a graphics
// mode the result is meaningless but harmless.
func (s *Session) ScreenText(rows int) (string, error) {
	pointer, err := s.ReadMemory(0x58, 2)
	if err != nil {
		return "", err
	}
	if len(pointer) < 2 {
		return "", errors.New("screen pointer unavailable")
	}
	const columns = 40
	address := int(pointer[0]) | int(pointer[1])<<8
	if rows < 1 || address+rows*columns > 0x10000 {
		return "", nil
	}
	memory, err := s.ReadMemory(address, rows*columns)
	if err != nil {
		return "", err
	}
	var text strings.Builder
	for i, code := range memory {
		if i > 0 && i%columns == 0 {
			text.WriteByte('\n')
		}
		// Screen codes: 0-63 are ASCII 32-95, 96-127 are lower case.
		switch code &= 0x7f; {
		case code < 0x40:
			text.WriteByte(code + 0x20)
		case code >= 0x60:
			text.WriteByte(code)
		default:
			text.WriteByte(' ')
		}
	}
	return text.String(), nil
}
