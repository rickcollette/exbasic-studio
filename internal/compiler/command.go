package compiler

import (
	"exbasicstudio/internal/project"
	"exbasicstudio/internal/tools"
)

// CommandLine returns the compiler and arguments that build the project, as
// Build runs them but without the metadata file Studio reads afterwards. It
// lets someone (or an assistant) build from a terminal exactly as Studio does.
func CommandLine(p *project.Project) (string, []string, error) {
	cfg := p.Config
	if err := project.ValidateConfig(cfg); err != nil {
		return "", nil, err
	}
	args, err := compilerArgs(p, true)
	if err != nil {
		return "", nil, err
	}
	entry, err := p.Resolve(cfg.Entry)
	if err != nil {
		return "", nil, err
	}
	output, err := p.Resolve(cfg.Output)
	if err != nil {
		return "", nil, err
	}
	args = append(args, "-o", output, entry)
	for _, file := range cfg.Assembly {
		path, resolveErr := p.Resolve(file)
		if resolveErr != nil {
			return "", nil, resolveErr
		}
		args = append(args, path)
	}
	return tools.Path(cfg.Compiler), args, nil
}
