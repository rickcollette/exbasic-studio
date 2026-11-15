package diskimage

import (
	"reflect"
	"testing"

	"exbasicstudio/internal/project"
)

func TestArgumentsForBootableSDFS(t *testing.T) {
	cfg := project.ATRConfig{Filesystem: "sparta", Boot: true, BootPage: 7}
	got := Arguments(cfg, "demo.atr", "demo.xex", []string{"readme.txt"})
	want := []string{"demo.atr", "-b", "demo.xex", "readme.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}

func TestArgumentsForEOSFS(t *testing.T) {
	cfg := project.ATRConfig{Filesystem: "eos", ExactSize: true, MinimumSize: 92160, BootPage: 12}
	got := Arguments(cfg, "demo.atr", "demo.xex", nil)
	want := []string{"-f", "eos", "-x", "-s", "92160", "-B", "12", "demo.atr", "demo.xex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}
