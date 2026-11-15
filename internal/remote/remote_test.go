package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveUsesOpenSSHConfig(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("OpenSSH is not installed")
	}
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	text := "Host devbox\n  HostName 192.0.2.10\n  User builder\n  Port 2222\n  IdentityFile ~/.ssh/id_ed25519\n  ForwardAgent yes\n  ServerAliveInterval 17\n  ServerAliveCountMax 4\n"
	if err := os.WriteFile(config, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve("devbox", config)
	if err != nil {
		t.Fatal(err)
	}
	if got.HostName != "192.0.2.10" || got.User != "builder" || got.Port != 2222 {
		t.Fatalf("bad resolved config: %+v", got)
	}
	if !got.ForwardAgent || got.ServerAliveInterval != 17 || got.ServerAliveCountMax != 4 {
		t.Fatalf("options not typed: %+v", got)
	}
	if len(got.IdentityFiles) == 0 || !filepath.IsAbs(got.IdentityFiles[0]) {
		t.Fatalf("identity path not expanded: %+v", got.IdentityFiles)
	}
}

func TestHostsFollowsIncludes(t *testing.T) {
	dir := t.TempDir()
	include := filepath.Join(dir, "work.conf")
	if err := os.WriteFile(include, []byte("Host buildbox *.wild !blocked\n  HostName build.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config")
	if err := os.WriteFile(config, []byte("Include work.conf\nHost personal\n  HostName home.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	hosts, err := Hosts(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0].Alias != "buildbox" || hosts[1].Alias != "personal" {
		t.Fatalf("unexpected hosts: %+v", hosts)
	}
}
