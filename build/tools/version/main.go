// Command version propagates the application version and build number from
// VERSION and BUILD_NUMBER at the repository root into every file that must
// carry them: the Wails build configuration, Windows version resource, macOS
// property lists, Linux package definition and frontend package metadata.
// (The Go binary receives both values at link time.)
//
//	go run ./build/tools/version          rewrite the files
//	go run ./build/tools/version --check  fail if any file is out of date
//	go run ./build/tools/version --print        print the product version
//	go run ./build/tools/version --print-build  print the build number
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var semver = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?$`)
var buildNumberPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

type target struct {
	path    string
	pattern *regexp.Regexp
	// value returns the replacement for the first capture group's content.
	value func(full, numeric string) string
}

func main() {
	root, err := findRoot()
	if err != nil {
		fail(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		fail(err)
	}
	version := strings.TrimSpace(string(raw))
	match := semver.FindStringSubmatch(version)
	if match == nil {
		fail(fmt.Errorf("VERSION %q is not a semantic version (MAJOR.MINOR.PATCH[-prerelease])", version))
	}
	// Windows version resources and Apple bundle versions are numeric only.
	numeric := match[1] + "." + match[2] + "." + match[3]
	buildRaw, err := os.ReadFile(filepath.Join(root, "BUILD_NUMBER"))
	if err != nil {
		fail(err)
	}
	buildNumber := strings.TrimSpace(string(buildRaw))
	if !buildNumberPattern.MatchString(buildNumber) {
		fail(fmt.Errorf("BUILD_NUMBER %q is not a positive integer", buildNumber))
	}
	if len(os.Args) > 1 && os.Args[1] == "--print" {
		fmt.Println(version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--print-build" {
		fmt.Println(buildNumber)
		return
	}
	check := len(os.Args) > 1 && os.Args[1] == "--check"
	if err := validateEditionBranding(root); err != nil {
		fail(err)
	}

	full := func(v, _ string) string { return v }
	num := func(_, n string) string { return n }
	build := func(_, _ string) string { return buildNumber }
	windowsVersion := func(_, n string) string { return n + "." + buildNumber }
	targets := []target{
		{"build/config.yml", regexp.MustCompile(`(?m)^(  version: ")[^"]*(")`), full},
		{"build/windows/info.json", regexp.MustCompile(`("file_version": ")[^"]*(")`), windowsVersion},
		{"build/windows/info.json", regexp.MustCompile(`("ProductVersion": ")[^"]*(")`), full},
		{"build/linux/nfpm/nfpm.yaml", regexp.MustCompile(`(?m)^(version: ")[^"]*(")`), full},
		{"build/linux/nfpm/nfpm.yaml", regexp.MustCompile(`(?m)^(release: ")[^"]*(")`), build},
		{"build/darwin/Info.plist", regexp.MustCompile(`(<key>CFBundleShortVersionString</key>\s*<string>)[^<]*(</string>)`), num},
		{"build/darwin/Info.plist", regexp.MustCompile(`(<key>CFBundleVersion</key>\s*<string>)[^<]*(</string>)`), build},
		{"build/darwin/Info.dev.plist", regexp.MustCompile(`(<key>CFBundleShortVersionString</key>\s*<string>)[^<]*(</string>)`), num},
		{"build/darwin/Info.dev.plist", regexp.MustCompile(`(<key>CFBundleVersion</key>\s*<string>)[^<]*(</string>)`), build},
		{"frontend/package.json", regexp.MustCompile(`(?m)^(  "version": ")[^"]*(")`), full},
		{"frontend/package-lock.json", regexp.MustCompile(`(?s)\A(\{\s*"name": "[^"]*",\s*"version": ")[^"]*(")`), full},
		{"frontend/package-lock.json", regexp.MustCompile(`(?s)("packages": \{\s*"": \{\s*"name": "[^"]*",\s*"version": ")[^"]*(")`), full},
	}

	stale := []string{}
	contents := map[string]string{}
	for _, t := range targets {
		path := filepath.Join(root, filepath.FromSlash(t.path))
		text, loaded := contents[path]
		if !loaded {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				fail(readErr)
			}
			text = string(data)
		}
		if !t.pattern.MatchString(text) {
			fail(fmt.Errorf("%s: no version field matches %s", t.path, t.pattern))
		}
		want := t.value(version, numeric)
		updated := t.pattern.ReplaceAllString(text, "${1}"+want+"${2}")
		if updated != text {
			stale = append(stale, t.path)
		}
		contents[path] = updated
	}
	if check {
		if len(stale) > 0 {
			fail(fmt.Errorf("version %s is not applied to: %s\nRun: go run ./build/tools/version", version, strings.Join(unique(stale), ", ")))
		}
		fmt.Printf("version %s build %s is consistent\n", version, buildNumber)
		return
	}
	for path, text := range contents {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			fail(err)
		}
	}
	fmt.Printf("version %s build %s applied (%d files updated)\n", version, buildNumber, len(unique(stale)))
}

// validateEditionBranding keeps local edition names out of the one public
// release pipeline. Keep the marker split so this guard does not match itself.
func validateEditionBranding(root string) error {
	forbidden := "personal" + "-rick"
	roots := []string{"internal", filepath.Join("frontend", "src")}
	for _, relativeRoot := range roots {
		searchRoot := filepath.Join(root, relativeRoot)
		err := filepath.WalkDir(searchRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(strings.ToLower(string(data)), forbidden) {
				relative, _ := filepath.Rel(root, path)
				return fmt.Errorf("edition-specific branding found in %s", filepath.ToSlash(relative))
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "VERSION")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("VERSION file not found above the current directory")
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "version:", err)
	os.Exit(1)
}
