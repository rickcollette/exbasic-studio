package emulator

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os/exec"
	"sync"

	processutil "exbasicstudio/internal/process"
)

type Frame struct {
	Running          bool   `json:"running"`
	Image            string `json:"image,omitempty"`
	Number           int    `json:"number,omitempty"`
	HostDevice       bool   `json:"hostDevice"`
	HostDeviceName   string `json:"hostDeviceName,omitempty"`
	WorkspaceChanged bool   `json:"workspaceChanged,omitempty"`
	Error            string `json:"error,omitempty"`
}

type InputState struct {
	KeyChar int  `json:"keychar"`
	KeyCode int  `json:"keycode"`
	Shift   bool `json:"shift"`
	Control bool `json:"control"`
}

type Launch struct {
	Helper         string  `json:"helper"`
	Artifact       string  `json:"artifact"`
	EOSCar         string  `json:"eosCar"`
	HostDir        string  `json:"hostDir"`
	HostTool       string  `json:"hostTool,omitempty"`
	HostDisk       string  `json:"hostDisk,omitempty"`
	HostDeviceName string  `json:"hostDeviceName,omitempty"`
	Profile        Profile `json:"profile"`
}

type State struct {
	Running        bool   `json:"running"`
	ProfileID      string `json:"profileId"`
	Artifact       string `json:"artifact"`
	HostDeviceName string `json:"hostDeviceName,omitempty"`
}

type request struct {
	ID   int    `json:"id"`
	Op   string `json:"op"`
	Key  string `json:"key,omitempty"`
	Down bool   `json:"down,omitempty"`
}

type response struct {
	ID         int    `json:"id"`
	OK         bool   `json:"ok"`
	Frame      string `json:"frame"`
	Pixels     string `json:"pixels"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Number     int    `json:"number"`
	HostDevice bool   `json:"hostDevice"`
	Error      string `json:"error"`
	KeyChar    int    `json:"keychar"`
	KeyCode    int    `json:"keycode"`
	Shift      int    `json:"shift"`
	Control    int    `json:"control"`
}

type Session struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	scan           *bufio.Scanner
	stderr         bytes.Buffer
	nextID         int
	closed         bool
	hostDeviceName string
}

func Start(launch Launch) (*Session, error) {
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
	}
	if launch.HostDir != "" {
		args = append(args, "-host-dir", launch.HostDir)
	}
	for _, arg := range launch.Profile.Arguments {
		args = append(args, "-arg", arg)
	}
	cmd := exec.Command(launch.Helper, args...)
	processutil.Background(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	s := &Session{cmd: cmd, stdin: stdin, scan: bufio.NewScanner(stdout), hostDeviceName: launch.HostDeviceName}
	s.scan.Buffer(make([]byte, 4096), 16<<20)
	cmd.Stderr = &s.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start embedded Atari800: %w", err)
	}
	return s, nil
}

func (s *Session) call(op, key string, down bool) (response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return response{}, errors.New("Atari800 session is stopped")
	}
	s.nextID++
	payload, _ := json.Marshal(request{ID: s.nextID, Op: op, Key: key, Down: down})
	payload = append(payload, '\n')
	if _, err := s.stdin.Write(payload); err != nil {
		return response{}, fmt.Errorf("write Atari800 command: %w", err)
	}
	var res response
	for {
		if !s.scan.Scan() {
			message := s.stderr.String()
			if message == "" {
				message = "emulator helper stopped"
			}
			return response{}, errors.New(message)
		}
		if json.Unmarshal(s.scan.Bytes(), &res) == nil && res.ID == s.nextID {
			break
		}
	}
	if !res.OK {
		return res, errors.New(res.Error)
	}
	return res, nil
}

func (s *Session) Frame() (Frame, error) {
	res, err := s.call("frame", "", false)
	if err != nil {
		return Frame{Error: err.Error()}, err
	}
	if res.Pixels != "" {
		raw, decodeErr := base64.StdEncoding.DecodeString(res.Pixels)
		if decodeErr != nil {
			return Frame{}, fmt.Errorf("decode Atari800 pixels: %w", decodeErr)
		}
		if res.Width <= 0 || res.Height <= 0 || len(raw) != res.Width*res.Height*3 {
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
	return Frame{Running: true, Image: res.Frame, Number: res.Number, HostDevice: hostDevice, HostDeviceName: hostDeviceName}, nil
}

func (s *Session) Key(key string, down bool) error {
	_, err := s.KeyState(key, down)
	return err
}

func (s *Session) KeyState(key string, down bool) (InputState, error) {
	res, err := s.call("key", key, down)
	return InputState{KeyChar: res.KeyChar, KeyCode: res.KeyCode, Shift: res.Shift != 0, Control: res.Control != 0}, err
}

func (s *Session) ReleaseKeys() error {
	_, err := s.call("releaseKeys", "", false)
	return err
}

// DismountHostDisk closes D8 before Studio rebuilds the EOS project disk.
// Keeping this in the helper protocol avoids replacing an image while the
// emulator still has it open, which is especially important on Windows.
func (s *Session) DismountHostDisk() error {
	_, err := s.call("dismountD8", "", false)
	return err
}

// MountHostDisk reopens the configured D8 image after Studio has refreshed it.
func (s *Session) MountHostDisk() error {
	_, err := s.call("mountD8", "", false)
	return err
}

func (s *Session) Stop() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	payload, _ := json.Marshal(request{ID: s.nextID + 1, Op: "stop"})
	_, _ = s.stdin.Write(append(payload, '\n'))
	_ = s.stdin.Close()
	s.mu.Unlock()
	return s.cmd.Wait()
}
