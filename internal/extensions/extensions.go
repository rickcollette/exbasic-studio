package extensions

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Manifest v1: discovery only. Untrusted plugins are not executed by the
// starter. A future WASI runtime must enforce explicit capability grants.
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	APIVersion  string   `json:"apiVersion"`
	EntryPoint  string   `json:"entrypoint"`
	Permissions []string `json:"permissions"`
}
type Entry struct {
	Manifest
	Status string `json:"status"`
}

func Discover(dir string) ([]Entry, error) {
	out := []Entry{}
	dirs, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, d.Name(), "extension.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m Manifest
		if json.Unmarshal(data, &m) != nil || m.ID == "" || m.APIVersion != "1" {
			continue
		}
		out = append(out, Entry{Manifest: m, Status: "discovered · execution disabled in scaffold"})
	}
	return out, nil
}
