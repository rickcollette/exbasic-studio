package terminal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// cmdSpecial are the characters cmd.exe interprets even inside quotes.
const cmdSpecial = "\"%!^&|<>\r\n"

// CommandLine builds the Windows command line that runs program with args.
// A batch file (.cmd or .bat, such as an npm launcher) can only run through
// cmd.exe, which rereads the line with its own rules: arguments for one must
// not contain cmd.exe metacharacters.
func CommandLine(program string, args []string) (string, error) {
	ext := strings.ToLower(filepath.Ext(program))
	if ext != ".cmd" && ext != ".bat" {
		parts := make([]string, 0, len(args)+1)
		parts = append(parts, windows.EscapeArg(program))
		for _, arg := range args {
			parts = append(parts, windows.EscapeArg(arg))
		}
		return strings.Join(parts, " "), nil
	}
	parts := make([]string, 0, len(args)+1)
	for _, arg := range append([]string{program}, args...) {
		if strings.ContainsAny(arg, cmdSpecial) {
			return "", errors.New("an argument for a batch file contains characters cmd.exe would interpret")
		}
		parts = append(parts, `"`+arg+`"`)
	}
	interpreter := os.Getenv("COMSPEC")
	if interpreter == "" {
		interpreter = "cmd.exe"
	}
	// /s with the whole line quoted makes cmd.exe strip exactly the outer
	// quotes and keep the quoted program path and arguments intact.
	return windows.EscapeArg(interpreter) + ` /d /s /c "` + strings.Join(parts, " ") + `"`, nil
}
