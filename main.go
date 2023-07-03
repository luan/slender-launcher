package main

import (
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/inconshreveable/go-update"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func cacheDirectory(appName string) string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatalf("Error getting cache directory: %v", err)
		return ""
	}
	return filepath.Join(cacheDir, appName)
}

func configDirectory(appName string) string {
	cacheDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("Error getting cache directory: %v", err)
		return ""
	}
	return filepath.Join(cacheDir, appName)
}

func isAppUpdateAvailable(logger *logrus.Logger, url string) (bool, string) {
	resp, err := http.Get(url + ".sha256")
	if err != nil {
		logger.Errorf("Error checking for app update: %s", err)
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Error checking for app update: %s", resp.Status)
		return false, ""
	}
	sha256RemoteBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("Error checking for app update: %s", err)
		return false, ""
	}
	sha256Remote := strings.Fields(string(sha256RemoteBytes))[0]
	currentExecutable, err := os.Executable()
	if err != nil {
		logger.Errorf("Error checking for app update: %s", err)
		return false, ""
	}
	sha256Local, err := sha256Sum(currentExecutable)
	if err != nil {
		logger.Errorf("Error checking for app update: %s", err)
		return false, ""
	}
	logger.Infof("Local SHA256: %s", sha256Local)
	logger.Infof("Remote SHA256: %s", sha256Remote)
	return sha256Local != sha256Remote, sha256Remote
}

func doUpdate(logger *logrus.Logger, url string) error {
	if viper.GetBool("dev") || fileExists("wails.json") {
		logger.Infof("Skipping update check in dev mode")
		return nil
	}
	logger.Infof("Checking for updates at %s", url)
	ok, sha256String := isAppUpdateAvailable(logger, url)
	if !ok {
		return nil
	}
	checksum, err := hex.DecodeString(sha256String)
	if err != nil {
		return err
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logger.Infof("No update available")
		return nil
	}
	original, err := os.Executable()
	if err != nil {
		logger.Errorf("Failed to get executable path: %v", err)
		return err
	}
	logger.Infof("Original executable: %s", original)
	logger.Info("Applying update")

	err = update.Apply(resp.Body, update.Options{Checksum: checksum})
	if err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			logger.Errorf("Failed to rollback from bad update: %v", rerr)
		}
		return err
	}

	logger.Info("Update applied successfully - restarting...")
	logger.Info(original)
	if err := syscall.Exec(original, os.Args, os.Environ()); err != nil {
		logger.Errorf("Failed to restart: %v, attempting regular fork", err)
		cmd := exec.Command(original, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			logger.Errorf("Failed to fork: %v", err)
			return err
		}
		os.Exit(0)
	}
	return nil
}

func main() {
	baseURL := "https://raw.githubusercontent.com/luan/tibia-client/main/"
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path: %v", err)
		os.Exit(1)
	}
	executable = filepath.Base(executable)
	appName := strings.TrimSuffix(executable, filepath.Ext(executable))

	if err := os.MkdirAll(configDirectory(appName), 0755); err != nil {
		fmt.Printf("Failed to create config directory: %v", err)
		os.Exit(1)
	}
	viper.SetConfigFile(filepath.Join(configDirectory(appName), "config.toml"))

	parallel := 64
	if err := viper.ReadInConfig(); err == nil {
		parallel = viper.GetInt("parallel")
		baseURL = viper.GetString("base_url")
	} else {
		viper.Set("parallel", parallel)
		viper.Set("base_url", baseURL)
	}

	if err := viper.WriteConfigAs(filepath.Join(configDirectory(appName), "config.toml")); err != nil {
		fmt.Printf("Failed to write config file: %v", err)
		os.Exit(1)
	}

	if runtime.GOOS == "windows" {
		executable += ".exe"
	} else if runtime.GOOS == "darwin" {
		executable += ".mac"
	}

	if err := os.MkdirAll(cacheDirectory(appName), 0755); err != nil {
		fmt.Printf("Failed to create cache directory: %v", err)
		os.Exit(1)
	}
	f, err := os.OpenFile(filepath.Join(cacheDirectory(appName), "launcher.log"), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}
	defer f.Close()

	logger := logrus.New()
	logger.SetOutput(io.MultiWriter(os.Stdout, f))
	logger.SetLevel(logrus.DebugLevel)

	logLevel := viper.GetString("log_level")
	switch logLevel {
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	if err := doUpdate(logger, baseURL+executable); err != nil {
		logger.Errorf("Failed to update: %v", err)
		os.Exit(1)
	}

	app := NewApp(logger, baseURL, appName, parallel)

	err = wails.Run(&options.App{
		Title:  appName + " Launcher",
		Width:  760,
		Height: 440,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		DisableResize:    true,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 13, G: 25, B: 51, A: 0},
		OnStartup:        app.startup,
		Windows: &windows.Options{
			ZoomFactor:           1.0,
			WebviewIsTransparent: true,
		},
		Mac: &mac.Options{
			WebviewIsTransparent: true,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
