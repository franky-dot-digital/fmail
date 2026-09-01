package main

import (
	"database/sql"
	"embed"
	_ "embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	// WICHTIG: Treiberwahl. modernc.org/sqlite ist eine reine Go-Implementierung
	// (kein CGO), was saubere Cross-Compiles für macOS/Windows/openSUSE aus
	// einer Codebasis erlaubt — genau der Grund, warum es hier steht statt
	// z. B. mattn/go-sqlite3. Der Rest der App (store.go) spricht nur mit
	// database/sql, ist also unabhängig vom konkreten Treiber.
	_ "modernc.org/sqlite"

	"fmail/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func openDB() (*sql.DB, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(configDir, "fmail")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(appDir, "fmail.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func main() {
	db, err := openDB()
	if err != nil {
		log.Fatalf("Datenbank konnte nicht geöffnet werden: %v", err)
	}
	defer db.Close()

	st, err := store.New(db)
	if err != nil {
		log.Fatalf("Datenbankschema konnte nicht angelegt werden: %v", err)
	}

	app := application.New(application.Options{
		Name:        "f:mail",
		Description: "Ein lokaler, aufgeräumter Mailclient",
		Services: []application.Service{
			application.NewService(NewMailService(st)),
			application.NewService(NewAccountService(st)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ProgramName: "digital.franky.Fmail",
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "f:mail",
		Width:            1180,
		Height:           780,
		MinWidth:         860,
		MinHeight:        560,
		BackgroundColour: application.NewRGB(247, 246, 243),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
