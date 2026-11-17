package studio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"exbasicstudio/internal/emulator"
	"exbasicstudio/internal/tools"
	"exbasicstudio/internal/tools/toolstest"
)

func TestCartridgeTypeOfAnImage(t *testing.T) {
	dir := t.TempDir()
	file := func(name string, size int, header string) string {
		t.Helper()
		data := make([]byte, size)
		copy(data, header)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cases := []struct {
		name string
		slot emulator.CartridgeSlot
		want int
		fail string
	}{
		{"a .car names its own type", emulator.CartridgeSlot{Kind: emulator.CartImage, Image: file("a.car", 8208, "CART"), Type: 2}, 0, ""},
		{"an explicit type wins for a raw image", emulator.CartridgeSlot{Kind: emulator.CartImage, Image: file("b.rom", 32<<10, ""), Type: 12}, 12, ""},
		{"64K SpartaDOS X", emulator.CartridgeSlot{Kind: emulator.CartSDX, Image: file("sdx64.rom", 64<<10, "")}, 11, ""},
		{"128K SpartaDOS X", emulator.CartridgeSlot{Kind: emulator.CartSDX, Image: file("sdx128.rom", 128<<10, "")}, 43, ""},
		{"standard 8K", emulator.CartridgeSlot{Kind: emulator.CartImage, Image: file("c.rom", 8<<10, "")}, 1, ""},
		{"standard 16K", emulator.CartridgeSlot{Kind: emulator.CartImage, Image: file("d.rom", 16<<10, "")}, 2, ""},
		{"an ambiguous raw size needs a choice", emulator.CartridgeSlot{Kind: emulator.CartImage, Image: file("e.rom", 64<<10, "")}, 0, "choose its cartridge type"},
		{"a missing file", emulator.CartridgeSlot{Kind: emulator.CartImage, Image: filepath.Join(dir, "none.rom")}, 0, "open the cartridge image"},
	}
	for _, c := range cases {
		got, err := cartridgeType(c.slot)
		if c.fail != "" {
			if err == nil || !strings.Contains(err.Error(), c.fail) {
				t.Errorf("%s: error = %v, want one containing %q", c.name, err, c.fail)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: type = %d, %v; want %d", c.name, got, err, c.want)
		}
	}
}

func TestCartridgeSlotsBecomeLaunchSettings(t *testing.T) {
	toolstest.Tool(t, "atari800-studio")
	eos, _ := tools.Asset("EOS.car")
	raw := filepath.Join(t.TempDir(), "sdx.rom")
	if err := os.WriteFile(raw, make([]byte, 128<<10), 0o644); err != nil {
		t.Fatal(err)
	}

	var launch emulator.Launch
	if err := applyCartridges(&launch, emulator.Profile{Cartridge: emulator.CartridgeSlot{Kind: emulator.CartEOS}}); err != nil || launch.EOSCar != eos || launch.Cart != "" {
		t.Fatalf("EOS in the primary slot: %+v, %v", launch, err)
	}

	launch = emulator.Launch{}
	profile := emulator.Profile{Cartridge: emulator.CartridgeSlot{Kind: emulator.CartSDX, Image: raw}, Cartridge2: emulator.CartridgeSlot{Kind: emulator.CartEOS}}
	if err := applyCartridges(&launch, profile); err != nil {
		t.Fatal(err)
	}
	if launch.EOSCar != "" || launch.Cart != raw || launch.CartType != 43 || launch.Cart2 != eos || launch.Cart2Type != 0 {
		t.Fatalf("SpartaDOS X with EOS on top: %+v", launch)
	}

	// A project packaged as an EOSFS disk keeps EOS in the primary slot.
	launch = emulator.Launch{EOSCar: eos}
	if err := applyCartridges(&launch, profile); err != nil || launch.EOSCar != eos || launch.Cart != "" {
		t.Fatalf("EOS chosen by the project: %+v, %v", launch, err)
	}

	launch = emulator.Launch{}
	if err := applyCartridges(&launch, emulator.Profile{Cartridge: emulator.CartridgeSlot{Kind: emulator.CartBasic}}); err != nil || launch.Cart != "" || launch.EOSCar != "" {
		t.Fatalf("Atari BASIC is a machine switch, not an image: %+v, %v", launch, err)
	}
}

// bootUntil runs the machine until the text screen contains want.
func bootUntil(t *testing.T, launch emulator.Launch, want string) string {
	t.Helper()
	session, err := emulator.Start(launch)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Stop() })
	screen := ""
	for attempt := 0; attempt < 60; attempt++ {
		for range 10 {
			if _, err = session.Frame(); err != nil {
				t.Fatal(err)
			}
		}
		if screen, err = session.ScreenText(24); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToUpper(screen), strings.ToUpper(want)) {
			return screen
		}
	}
	t.Fatalf("%q never appeared; screen:\n%s", want, screen)
	return screen
}

// A cartridge image in the primary slot is inserted by the helper and takes
// control of the machine. The bundled EOS image stands in for any image.
func TestCartridgeImageBootsFromThePrimarySlot(t *testing.T) {
	helper := toolstest.Tool(t, "atari800-studio")
	image, ok := tools.Asset("EOS.car")
	if !ok {
		t.Skip("EOS.car is not available")
	}
	profile := emulator.Profile{Machine: "xl", Memory: "64", Video: "ntsc", SIOPatch: true, CPUSpeed: 100, Cartridge: emulator.CartridgeSlot{Kind: emulator.CartImage, Image: image}}
	launch := emulator.Launch{Helper: helper, Profile: profile}
	if err := applyCartridges(&launch, profile); err != nil {
		t.Fatal(err)
	}
	if launch.Cart != image || launch.EOSCar != "" {
		t.Fatalf("launch = %+v", launch)
	}
	bootUntil(t, launch, "Extendable Operating System")
}

// Atari BASIC in a slot switches on the BASIC of the machine.
func TestAtariBasicSlotStartsBasic(t *testing.T) {
	helper := toolstest.Tool(t, "atari800-studio")
	store := emulator.NewProfileStore(filepath.Join(t.TempDir(), "profiles.json"))
	profile, err := store.Save(emulator.Profile{Name: "BASIC", Machine: "xl", SIOPatch: true, Cartridge: emulator.CartridgeSlot{Kind: emulator.CartBasic}})
	if err != nil {
		t.Fatal(err)
	}
	launch := emulator.Launch{Helper: helper, Profile: profile}
	if err = applyCartridges(&launch, profile); err != nil {
		t.Fatal(err)
	}
	bootUntil(t, launch, "READY")
}
