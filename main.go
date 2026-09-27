// Command db-manager is a desktop database client built with Wails v2.
//
// The Go binary is the whole application: it embeds the built React frontend,
// hosts the database drivers and exposes them to the webview through Wails'
// binding layer.
package main

import (
	"embed"
	"errors"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	// Registers every database driver into the registry.
	_ "dbmanager/internal/drivers/all"
	"dbmanager/internal/singleinstance"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// One copy at a time. A second launch must not open the same profiles, change
	// log and state file as the copy that is running, so it asks that copy to come
	// to the front and leaves — the window is usually hidden behind the
	// notification-area icon, and a launch that did nothing at all would look like
	// a launch that failed. The claim is taken before anything of the store is
	// opened, so a copy that starts is the only one holding it open.
	instance, err := singleinstance.Acquire()
	if errors.Is(err, singleinstance.ErrAlreadyRunning) {
		if err := singleinstance.Activate(); err != nil {
			log.Printf("db-manager: %v", err)
		}
		return
	}
	if err != nil {
		log.Fatalf("db-manager: %v", err)
	}

	app, err := NewApp()
	if err != nil {
		log.Fatalf("db-manager: %v", err)
	}
	app.instance = instance

	err = wails.Run(&options.App{
		Title:     appName,
		Width:     1360,
		Height:    860,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Matches the default (light) frontend surface so the window does not
		// flash white before React mounts.
		BackgroundColour: &options.RGBA{R: 238, G: 240, B: 243, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		// Closing the window hides it behind the notification-area icon rather
		// than quitting; the tray menu's Quit is the way out.
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		log.Fatalf("db-manager: %v", err)
	}
}
