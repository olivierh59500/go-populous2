// Package mobile exposes the independent Go game to Ebitengine's Android view.
package mobile

import (
	"fmt"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"
	runtimeassets "go-populous2/assets/runtime"
	"go-populous2/internal/app"
)

var populousGame *app.Game

func init() {
	files, err := runtimeassets.FS()
	if err != nil {
		panic(fmt.Errorf("embedded Populous II assets: %w", err))
	}
	assets, err := app.LoadAssets(files)
	if err != nil {
		panic(fmt.Errorf("load Populous II assets: %w", err))
	}
	populousGame, err = app.NewMobile(assets)
	if err != nil {
		panic(fmt.Errorf("create mobile Populous II game: %w", err))
	}
	// Preserve the original PAL clock; physics advances every four updates.
	ebiten.SetTPS(50)
	enginemobile.SetGame(populousGame)
}

// SetFilesDir selects Android's app-private save directory before the view starts.
func SetFilesDir(path string) {
	if populousGame != nil {
		if path == "" {
			populousGame.SetMobileSavePath("")
		} else {
			populousGame.SetMobileSavePath(filepath.Join(path, "go-populous2.json"))
		}
	}
}

// CancelInput discards unfinished gestures after activity focus or pause changes.
func CancelInput() {
	if populousGame != nil {
		populousGame.CancelInput()
	}
}

// SetBluetoothAvailable publishes the platform adapter's availability.
func SetBluetoothAvailable(available bool) {
	if populousGame != nil {
		populousGame.SetBluetoothAvailable(available)
	}
}

// PollBluetoothCommand returns one nonblocking Android transport request.
func PollBluetoothCommand() string {
	if populousGame == nil {
		return ""
	}
	return populousGame.PollPlatformCommand()
}

// BluetoothStatus queues platform progress for the Go update loop.
func BluetoothStatus(status string) {
	if populousGame != nil {
		populousGame.PostBluetoothStatus(status)
	}
}

// BluetoothClientReady supplies the authenticated, private loopback proxy.
func BluetoothClientReady(address, name string) {
	if populousGame != nil {
		populousGame.PostBluetoothReady(address, name)
	}
}

// BluetoothFailed reports permission, discovery and transport failures.
func BluetoothFailed(message string) {
	if populousGame != nil {
		populousGame.PostBluetoothFailure(message)
	}
}

// CancelBluetooth releases the corresponding Go transport reservation.
func CancelBluetooth() {
	if populousGame != nil {
		populousGame.CancelBluetooth()
	}
}

// Dummy ensures gomobile includes the package in its generated bindings.
func Dummy() {}
