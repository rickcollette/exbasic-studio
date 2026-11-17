// Package app wires the exBASIC Studio services into the Wails runtime.
package app

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"exbasicstudio/internal/studio"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	name        = "exBASIC Studio"
	description = "A desktop IDE for exBASIC on Atari 8-bit systems"
)

// Run creates the native application and blocks until it exits.
func Run(assets fs.FS) error {
	frontend, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return fmt.Errorf("load embedded frontend: %w", err)
	}

	studioService := studio.New()
	wailsApp := application.New(application.Options{
		Name:        name,
		Description: description,
		Services: []application.Service{
			application.NewService(studioService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(frontend),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{AdditionalBrowserArgs: debugBrowserArgs()},
	})

	// Operation, terminal and workspace events flow through one emitter.
	studio.Attach(studioService, func(name string, data any) { wailsApp.Event.Emit(name, data) })

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            name,
		Width:            1400,
		Height:           900,
		MinWidth:         920,
		MinHeight:        620,
		URL:              "/",
		Frameless:        true,
		BackgroundColour: application.NewRGB(24, 24, 24),
		EnableFileDrop:   true,
	})
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		wailsApp.Event.Emit("files-dropped", map[string]any{
			"files":   event.Context().DroppedFiles(),
			"details": event.Context().DropTargetDetails(),
		})
	})
	window.Show()

	if err := wailsApp.Run(); err != nil {
		return fmt.Errorf("run application: %w", err)
	}
	return nil
}

// debugBrowserArgs lets developers and the end-to-end tests attach to the
// real window. It is off unless EXBASIC_STUDIO_DEBUG_PORT names a port; the
// web view then listens on the loopback interface only.
func debugBrowserArgs() []string {
	port, err := strconv.Atoi(os.Getenv("EXBASIC_STUDIO_DEBUG_PORT"))
	if err != nil || port < 1024 || port > 65535 {
		return nil
	}
	return []string{"--remote-debugging-port=" + strconv.Itoa(port)}
}
