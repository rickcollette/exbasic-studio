package emulator

import (
	"path/filepath"
	"testing"
)

func TestCartridgeSlotsAreNormalizedAndMigrated(t *testing.T) {
	store := NewProfileStore(filepath.Join(t.TempDir(), "profiles.json"))

	// A profile written before the slots existed: two flags.
	old, err := store.Save(Profile{Name: "Old EOS", EOS: true, Basic: true, SIOPatch: true})
	if err != nil {
		t.Fatal(err)
	}
	if old.Cartridge.Kind != CartEOS || old.Cartridge2.Kind != CartBasic || !old.EOS || !old.Basic {
		t.Fatalf("flags were not migrated to slots: %+v", old)
	}

	sdx, err := store.Save(Profile{Name: "SDX", Cartridge: CartridgeSlot{Kind: "SDX", Image: " sdx.rom ", Type: 43}, Cartridge2: CartridgeSlot{Kind: CartBasic}})
	if err != nil {
		t.Fatal(err)
	}
	if sdx.Cartridge != (CartridgeSlot{Kind: CartSDX, Image: "sdx.rom", Type: 43}) || !sdx.Basic || sdx.EOS {
		t.Fatalf("SpartaDOS X with BASIC: %+v", sdx)
	}

	// Clearing the slots clears the derived flags.
	cleared := old
	cleared.Cartridge, cleared.Cartridge2 = CartridgeSlot{}, CartridgeSlot{}
	cleared.EOS, cleared.Basic = false, false
	if cleared, err = store.Save(cleared); err != nil || cleared.EOS || cleared.Basic {
		t.Fatalf("cleared profile: %+v, %v", cleared, err)
	}

	// A lone secondary cartridge moves to the primary slot.
	moved, err := store.Save(Profile{Name: "Moved", Cartridge2: CartridgeSlot{Kind: CartEOS}})
	if err != nil || moved.Cartridge.Kind != CartEOS || moved.Cartridge2.Kind != CartNone || !moved.EOS {
		t.Fatalf("lone secondary cartridge: %+v, %v", moved, err)
	}

	// EOS drives the machine only from the primary slot.
	second, err := store.Save(Profile{Name: "Second", Cartridge: CartridgeSlot{Kind: CartSDX, Image: "sdx.car"}, Cartridge2: CartridgeSlot{Kind: CartEOS}})
	if err != nil || second.EOS {
		t.Fatalf("EOS in the secondary slot: %+v, %v", second, err)
	}

	for name, bad := range map[string]Profile{
		"image kind without a file": {Name: "x", Cartridge: CartridgeSlot{Kind: CartImage}},
		"unknown kind":              {Name: "x", Cartridge: CartridgeSlot{Kind: "floppy"}},
		"type out of range":         {Name: "x", Cartridge: CartridgeSlot{Kind: CartImage, Image: "a.rom", Type: MaxCartridgeType + 1}},
	} {
		if _, err = store.Save(bad); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
