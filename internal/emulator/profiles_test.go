package emulator

import (
	"path/filepath"
	"testing"
)

func TestProfileStoreLifecycle(t *testing.T) {
	store := NewProfileStore(filepath.Join(t.TempDir(), "profiles.json"))
	profiles, err := store.Load()
	if err != nil || len(profiles) < 2 {
		t.Fatalf("load defaults: profiles=%d err=%v", len(profiles), err)
	}
	saved, err := store.Save(Profile{Name: "PAL Test", Machine: "xe", Video: "pal", SIOPatch: true, Scale: 3, Arguments: []string{" -sound ", ""}})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != "pal-test" || len(saved.Arguments) != 1 || saved.Arguments[0] != "-sound" {
		t.Fatalf("unexpected saved profile: %#v", saved)
	}
	loaded, err := store.Find(saved.ID)
	if err != nil || loaded.Video != "pal" || loaded.Scale != 3 {
		t.Fatalf("find saved profile: %#v err=%v", loaded, err)
	}
	if err = store.Delete(saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Find(saved.ID); err == nil {
		t.Fatal("deleted profile still exists")
	}
}

func TestProfileHardwareConfiguration(t *testing.T) {
	store := NewProfileStore(filepath.Join(t.TempDir(), "profiles.json"))
	saved, err := store.Save(Profile{
		Name:        "BBS Lab",
		Machine:     "xe",
		Memory:      "320-rambo",
		Video:       "ntsc",
		CPUSpeed:    300,
		Ultimate1MB: false,
		Drives: []DriveConfig{
			{Image: "bbs.atr", Enabled: true},
			{},
			{Image: "utils.atr", Enabled: true, ReadOnly: true},
		},
		RDevice: RDeviceConfig{Mode: "rverter", ListenPort: 9000, ConnectBaud: 19200},
		IDE:     IDEConfig{Enabled: true, Image: "bbs-hard-disk.img", CompactFlash: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Memory != "320-rambo" || saved.CPUSpeed != 300 || len(saved.Drives) != 4 {
		t.Fatalf("hardware profile was not preserved: %#v", saved)
	}
	if !saved.Drives[2].ReadOnly || saved.RDevice.Mode != "rverter" || saved.RDevice.ConnectBaud != 19200 || !saved.RDevice.Modem || !saved.IDE.CompactFlash {
		t.Fatalf("device profile was not preserved: %#v", saved)
	}

	for _, speed := range []int{100, 150, 200, 300, 500} {
		if _, err := normalizeProfile(Profile{Name: "speed", Machine: "xl", Memory: "64", CPUSpeed: speed}); err != nil {
			t.Fatalf("CPU speed %d should be supported: %v", speed, err)
		}
	}
	if _, err := normalizeProfile(Profile{Name: "bad speed", Machine: "xl", Memory: "64", CPUSpeed: 250}); err == nil {
		t.Fatal("unsupported CPU speed was accepted")
	}
	if _, err := normalizeProfile(Profile{Name: "old console", Machine: "5200", Memory: "16", CPUSpeed: 100}); err == nil {
		t.Fatal("Atari 5200 profile was accepted even though ex800 does not support it")
	}
	for _, memory := range []string{"64", "128", "256", "320-rambo", "320-compy", "1088"} {
		if _, err := normalizeProfile(Profile{Name: "memory", Machine: "xl", Memory: memory, CPUSpeed: 100}); err != nil {
			t.Fatalf("memory configuration %s should be supported: %v", memory, err)
		}
	}
}
