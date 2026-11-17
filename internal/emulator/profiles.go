package emulator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Machine     string `json:"machine"`
	Memory      string `json:"memory"`
	Video       string `json:"video"`
	CPUSpeed    int    `json:"cpuSpeed"`
	Basic       bool   `json:"basic"`
	SIOPatch    bool   `json:"sioPatch"`
	EOS         bool   `json:"eos"`
	Ultimate1MB bool   `json:"ultimate1MB"`
	// Cartridge is the primary (left) slot and Cartridge2 the secondary one:
	// the pass-through connector on top of a SpartaDOS X cartridge.
	// Basic and EOS are derived from the slots; they remain in the stored
	// form so profiles written before the slots existed still load.
	Cartridge  CartridgeSlot `json:"cartridge"`
	Cartridge2 CartridgeSlot `json:"cartridge2"`
	// HostDir is the folder the Atari sees as H: (D8: under EOS). Blank
	// means the H folder of the open project. A relative path is taken from
	// the project folder.
	HostDir   string        `json:"hostDir"`
	Scale     int           `json:"scale"`
	Media     string        `json:"media"`
	Drives    []DriveConfig `json:"drives"`
	RDevice   RDeviceConfig `json:"rDevice"`
	IDE       IDEConfig     `json:"ide"`
	Arguments []string      `json:"arguments"`
}

// Cartridge kinds.
const (
	CartNone  = ""
	CartBasic = "basic" // Atari BASIC (the built-in ROM on XL/XE machines)
	CartEOS   = "eos"   // the EOS cartridge bundled with Studio
	CartSDX   = "sdx"   // a SpartaDOS X image supplied by the user
	CartImage = "image" // any other cartridge image
)

// MaxCartridgeType is the highest cartridge type number Atari800 knows.
const MaxCartridgeType = 160

// CartridgeSlot describes what is plugged into one cartridge slot.
type CartridgeSlot struct {
	Kind string `json:"kind"`
	// Image is the cartridge file for the sdx and image kinds.
	Image string `json:"image"`
	// Type is the Atari800 cartridge type of a raw image. Zero means
	// automatic: taken from a .car header, or from the file size where that
	// is unambiguous.
	Type int `json:"type"`
}

func normalizeSlot(slot CartridgeSlot, name string) (CartridgeSlot, error) {
	slot.Kind = strings.ToLower(strings.TrimSpace(slot.Kind))
	slot.Image = strings.TrimSpace(slot.Image)
	switch slot.Kind {
	case CartNone, "none":
		return CartridgeSlot{}, nil
	case CartBasic, CartEOS:
		return CartridgeSlot{Kind: slot.Kind}, nil
	case CartSDX, CartImage:
		if slot.Image == "" {
			return CartridgeSlot{}, fmt.Errorf("the %s cartridge slot needs an image file", name)
		}
		if slot.Type < 0 || slot.Type > MaxCartridgeType {
			return CartridgeSlot{}, fmt.Errorf("unsupported cartridge type %d in the %s slot", slot.Type, name)
		}
		return slot, nil
	}
	return CartridgeSlot{}, fmt.Errorf("unsupported cartridge %q in the %s slot", slot.Kind, name)
}

type DriveConfig struct {
	Image    string `json:"image"`
	Enabled  bool   `json:"enabled"`
	ReadOnly bool   `json:"readOnly"`
}

type RDeviceConfig struct {
	Mode        string `json:"mode"`
	ListenPort  int    `json:"listenPort"`
	ConnectBaud int    `json:"connectBaud"`
	SerialPort  string `json:"serialPort"`
	Modem       bool   `json:"modem"`
}

type IDEConfig struct {
	Enabled      bool   `json:"enabled"`
	Image        string `json:"image"`
	CompactFlash bool   `json:"compactFlash"`
}

type ProfileStore struct {
	path string
}

func NewProfileStore(path string) *ProfileStore { return &ProfileStore{path: path} }

func DefaultProfileStore() *ProfileStore {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return NewProfileStore(filepath.Join(dir, "exbasic-studio", "atari800-profiles.json"))
}

func defaultProfiles() []Profile {
	return []Profile{
		{ID: "atari-800xl-ntsc", Name: "Atari 800XL (NTSC)", Machine: "xl", Memory: "64", Video: "ntsc", CPUSpeed: 100, SIOPatch: true, Scale: 2, Drives: blankDrives(), RDevice: defaultRDevice(), Arguments: []string{}},
		{ID: "eos-800xl-ntsc", Name: "ExtendedOS / EOS", Machine: "xl", Memory: "64", Video: "ntsc", CPUSpeed: 100, SIOPatch: true, EOS: true, Cartridge: CartridgeSlot{Kind: CartEOS}, Scale: 2, Drives: blankDrives(), RDevice: defaultRDevice(), Arguments: []string{}},
	}
}

func blankDrives() []DriveConfig { return make([]DriveConfig, 4) }

func defaultRDevice() RDeviceConfig {
	return RDeviceConfig{Mode: "disabled", ListenPort: 9000, ConnectBaud: 2400}
}

func normalizeProfile(profile Profile) (Profile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	if profile.Name == "" {
		return Profile{}, errors.New("profile name is required")
	}
	profile.Machine = strings.ToLower(strings.TrimSpace(profile.Machine))
	if profile.Machine == "" {
		profile.Machine = "xl"
	}
	switch profile.Machine {
	case "atari", "xl", "xe", "5200":
	default:
		return Profile{}, fmt.Errorf("unsupported Atari machine %q", profile.Machine)
	}
	profile.Memory = strings.ToLower(strings.TrimSpace(profile.Memory))
	if profile.Memory == "320" {
		profile.Memory = "320-rambo"
	}
	if profile.Ultimate1MB {
		profile.Memory = "1088"
	}
	if profile.Memory == "" {
		switch profile.Machine {
		case "xe":
			profile.Memory = "128"
		default:
			profile.Memory = "64"
		}
	}
	switch profile.Memory {
	case "64", "128", "256", "320-compy", "320-rambo", "1088":
	default:
		return Profile{}, fmt.Errorf("unsupported Atari memory configuration %q", profile.Memory)
	}
	if profile.Machine == "5200" {
		profile.Memory = "16"
	}
	if profile.CPUSpeed == 0 {
		profile.CPUSpeed = 100
	}
	switch profile.CPUSpeed {
	case 100, 150, 200, 300, 500:
	default:
		return Profile{}, fmt.Errorf("unsupported CPU speed %d%%", profile.CPUSpeed)
	}
	profile.Video = strings.ToLower(strings.TrimSpace(profile.Video))
	if profile.Video == "" {
		profile.Video = "ntsc"
	}
	if profile.Video != "ntsc" && profile.Video != "pal" {
		return Profile{}, fmt.Errorf("unsupported video standard %q", profile.Video)
	}
	if profile.Scale < 1 || profile.Scale > 4 {
		profile.Scale = 2
	}
	// Profiles saved before the cartridge slots existed carried two flags.
	if strings.TrimSpace(profile.Cartridge.Kind) == "" && strings.TrimSpace(profile.Cartridge2.Kind) == "" {
		switch {
		case profile.EOS:
			profile.Cartridge.Kind = CartEOS
			if profile.Basic {
				profile.Cartridge2.Kind = CartBasic
			}
		case profile.Basic:
			profile.Cartridge.Kind = CartBasic
		}
	}
	var slotErr error
	if profile.Cartridge, slotErr = normalizeSlot(profile.Cartridge, "primary"); slotErr != nil {
		return Profile{}, slotErr
	}
	if profile.Cartridge2, slotErr = normalizeSlot(profile.Cartridge2, "secondary"); slotErr != nil {
		return Profile{}, slotErr
	}
	if profile.Cartridge.Kind == CartNone && profile.Cartridge2.Kind != CartNone {
		// Nothing can sit in the secondary slot on its own.
		profile.Cartridge, profile.Cartridge2 = profile.Cartridge2, CartridgeSlot{}
	}
	if profile.Cartridge.Kind == profile.Cartridge2.Kind && (profile.Cartridge.Kind == CartBasic || profile.Cartridge.Kind == CartEOS) {
		profile.Cartridge2 = CartridgeSlot{}
	}
	profile.Basic = profile.Cartridge.Kind == CartBasic || profile.Cartridge2.Kind == CartBasic
	// EOS drives the machine only from the primary slot.
	profile.EOS = profile.Cartridge.Kind == CartEOS
	profile.Media = strings.TrimSpace(profile.Media)
	profile.HostDir = strings.TrimSpace(profile.HostDir)
	if strings.ContainsRune(profile.HostDir, 0) {
		return Profile{}, errors.New("the H: folder path is not valid")
	}
	drives := blankDrives()
	for i := 0; i < len(profile.Drives) && i < len(drives); i++ {
		drives[i] = profile.Drives[i]
		drives[i].Image = strings.TrimSpace(drives[i].Image)
		if drives[i].Image == "" {
			drives[i].Enabled = false
		}
	}
	profile.Drives = drives
	profile.RDevice.Mode = strings.ToLower(strings.TrimSpace(profile.RDevice.Mode))
	if profile.RDevice.Mode == "" {
		profile.RDevice.Mode = "disabled"
	}
	switch profile.RDevice.Mode {
	case "disabled", "850", "rverter":
	default:
		return Profile{}, fmt.Errorf("unsupported R: device mode %q", profile.RDevice.Mode)
	}
	// R-Verter is a modem interface by definition. Keep this invariant in the
	// persisted model too, so older profiles and non-UI callers cannot launch a
	// nominal R-Verter with modem responses disabled.
	if profile.RDevice.Mode == "rverter" {
		profile.RDevice.Modem = true
	}
	if profile.RDevice.ListenPort == 0 {
		profile.RDevice.ListenPort = 9000
	}
	if profile.RDevice.ListenPort < 1 || profile.RDevice.ListenPort > 65532 {
		return Profile{}, errors.New("R: base listen port must leave room for R1:-R4:")
	}
	if profile.RDevice.ConnectBaud == 0 {
		profile.RDevice.ConnectBaud = 2400
	}
	switch profile.RDevice.ConnectBaud {
	case 300, 1200, 2400, 4800, 9600, 19200:
	default:
		return Profile{}, fmt.Errorf("unsupported modem connect speed %d", profile.RDevice.ConnectBaud)
	}
	profile.RDevice.SerialPort = strings.TrimSpace(profile.RDevice.SerialPort)
	profile.IDE.Image = strings.TrimSpace(profile.IDE.Image)
	if profile.IDE.Enabled && profile.IDE.Image == "" {
		return Profile{}, errors.New("IDE Plus 2.0 emulation requires a disk image")
	}
	cleanArgs := make([]string, 0, len(profile.Arguments))
	for _, arg := range profile.Arguments {
		if value := strings.TrimSpace(arg); value != "" {
			cleanArgs = append(cleanArgs, value)
		}
	}
	profile.Arguments = cleanArgs
	return profile, nil
}

func (s *ProfileStore) Load() ([]Profile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultProfiles(), nil
	}
	if err != nil {
		return nil, err
	}
	var profiles []Profile
	if err = json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("read Atari800 profiles: %w", err)
	}
	if len(profiles) == 0 {
		return defaultProfiles(), nil
	}
	for i := range profiles {
		profiles[i], err = normalizeProfile(profiles[i])
		if err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

var invalidID = regexp.MustCompile(`[^a-z0-9]+`)

func profileID(name string) string {
	id := strings.Trim(invalidID.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if id == "" {
		id = fmt.Sprintf("profile-%d", time.Now().Unix())
	}
	return id
}

func (s *ProfileStore) Save(profile Profile) (Profile, error) {
	var err error
	profile, err = normalizeProfile(profile)
	if err != nil {
		return Profile{}, err
	}
	profiles, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	if strings.TrimSpace(profile.ID) == "" {
		base := profileID(profile.Name)
		profile.ID = base
		for suffix := 2; profileExists(profiles, profile.ID); suffix++ {
			profile.ID = fmt.Sprintf("%s-%d", base, suffix)
		}
	}
	replaced := false
	for i := range profiles {
		if profiles[i].ID == profile.ID {
			profiles[i] = profile
			replaced = true
			break
		}
	}
	if !replaced {
		profiles = append(profiles, profile)
	}
	return profile, s.write(profiles)
}

func profileExists(profiles []Profile, id string) bool {
	for _, profile := range profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

func (s *ProfileStore) Delete(id string) error {
	profiles, err := s.Load()
	if err != nil {
		return err
	}
	next := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ID != id {
			next = append(next, profile)
		}
	}
	if len(next) == 0 {
		return errors.New("at least one Atari800 profile is required")
	}
	return s.write(next)
}

func (s *ProfileStore) Find(id string) (Profile, error) {
	profiles, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	if id == "" {
		return profiles[0], nil
	}
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("Atari800 profile %q was not found", id)
}

func (s *ProfileStore) write(profiles []Profile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	temporary := s.path + ".tmp"
	if err = os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, s.path)
}
