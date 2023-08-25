package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/ulikunitz/xz/lzma"
)

type File struct {
	LocalFile    string `json:"localfile"`
	PackedHash   string `json:"packedhash"`
	PackedSize   int    `json:"packedsize"`
	URL          string `json:"url"`
	UnpackedHash string `json:"unpackedhash"`
	UnpackedSize int    `json:"unpackedsize"`
}

type AssetsInfo struct {
	Files   []File `json:"files"`
	Version int    `json:"version"`
}

type ClientInfo struct {
	Revision   int    `json:"revision"`
	Version    string `json:"version"`
	Files      []File `json:"files"`
	Executable string `json:"executable"`
	Generation string `json:"generation"`
	Variant    string `json:"variant"`
}

type App struct {
	ctx     context.Context
	logger  *logrus.Logger
	baseURL string
	appName string

	clientInfo ClientInfo
	assetsInfo AssetsInfo

	totalBytes      int64
	totalFiles      int64
	downloadedBytes int64
	downloadedFiles int64

	parallel int

	activeDownloads map[string]struct{}
	mutex           sync.Mutex

	queue  chan File
	cancel chan struct{}
}

func NewApp(logger *logrus.Logger, baseURL string, appName string, parallel int) *App {
	return &App{
		logger:          logger,
		baseURL:         baseURL,
		queue:           make(chan File, 16),
		cancel:          make(chan struct{}),
		activeDownloads: make(map[string]struct{}),
		parallel:        parallel,
		appName:         appName,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) OpenClientLocation() {
	fmt.Println("Opening client location")
	if runtime.GOOS == "darwin" {
		exec.Command("open", a.appDirectory()).Start()
	} else if runtime.GOOS == "windows" {
		exec.Command("explorer", a.appDirectory()).Start()
	} else if runtime.GOOS == "linux" {
		exec.Command("xdg-open", a.appDirectory()).Start()
	}
}

func (a *App) Exit() {
	os.Exit(0)
}

func (a *App) remoteClientJSON() string {
	return "client." + a.OS() + ".json"
}

func (a *App) remoteAssetsJSON() string {
	return "assets." + a.OS() + ".json"
}

func (a *App) refreshManifests() {
	err := a.downloadFile(a.baseURL+a.remoteClientJSON(), "client.json", false)
	if err != nil {
		a.logger.Errorf("Error downloading %s: %v", a.remoteClientJSON(), err)
	}

	err = readJSON(filepath.Join(a.appDirectory(), "client.json"), &a.clientInfo)
	if err != nil {
		a.logger.Errorf("Error reading %s: %v", "client.json", err)
	}

	err = a.downloadFile(a.baseURL+a.remoteAssetsJSON(), "assets.json", false)
	if err != nil {
		a.logger.Errorf("Error downloading %s: %v", a.remoteAssetsJSON(), err)
	}

	err = readJSON(filepath.Join(a.appDirectory(), "assets.json"), &a.assetsInfo)
	if err != nil {
		a.logger.Errorf("Error reading %s: %v", "assets.json", err)
	}
}

func (a *App) Version() string {
	a.refreshManifests()
	return a.clientInfo.Version
}

func (a *App) Revision() int {
	a.refreshManifests()
	return a.clientInfo.Revision
}

func (a *App) DownloadPercent() float64 {
	if a.totalBytes == 0 {
		return 0
	}
	percent := float64(a.downloadedBytes) / float64(a.totalBytes) * 100
	a.logger.Infof("Downloaded %d/%d files |  %d/%d bytes (%.2f%%)", a.downloadedFiles, a.totalFiles, a.downloadedBytes, a.totalBytes, percent)
	return percent
}

func (a *App) TotalFiles() int64 {
	return a.totalFiles
}

func (a *App) TotalBytes() int64 {
	return a.totalBytes
}

func (a *App) DownloadedFiles() int64 {
	return a.downloadedFiles
}

func (a *App) DownloadedBytes() int64 {
	return a.downloadedBytes
}

func (a *App) ToggleLocal(value bool) {
	a.logger.Infof("Setting enableLocal to %v", value)
	viper.Set("enableLocal", value)
	a.saveConfig()
}

func (a *App) saveConfig() {
	if err := viper.WriteConfigAs(filepath.Join(configDirectory(a.appName), "config.toml")); err != nil {
		a.logger.Errorf("Error writing config: %v", err)
	}
}

func (a *App) LocalEnabled() bool {
	return viper.GetBool("enableLocal")
}

func (a *App) OS() string {
	os := runtime.GOOS
	if os == "darwin" {
		return "mac"
	}
	return os
}

func (a *App) ActiveDownload() string {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for url := range a.activeDownloads {
		return url
	}
	return ""
}

func (a *App) Update() {
	files, err := a.filesToUpdate()
	if err != nil {
		a.logger.Errorf("Error checking for updates: %v", err)
	}

	a.totalFiles = int64(len(files))
	a.totalBytes = 0
	a.downloadedFiles = 0
	a.downloadedBytes = 0
	for _, file := range files {
		a.totalBytes += int64(file.PackedSize)
	}

	for i := 0; i < a.parallel; i++ {
		go func() {
			for {
				select {
				case <-a.cancel:
					return
				case <-a.ctx.Done():
					return
				case file := <-a.queue:
					a.mutex.Lock()
					a.activeDownloads[file.URL] = struct{}{}
					a.mutex.Unlock()
					err := a.downloadFile(a.baseURL+file.URL, file.LocalFile, true)
					a.mutex.Lock()
					delete(a.activeDownloads, file.URL)
					a.mutex.Unlock()
					if err != nil {
						a.logger.Errorf("Error downloading %s: %v", file.URL, err)
						return
					}
					a.logger.Debugf("Downloaded %s", file.URL)
				}
			}
		}()
	}

	for _, file := range files {
		a.queue <- file
	}
}

var mapKinds = map[int]string{
	0: "https://tibiamaps.github.io/tibia-map-data/minimap-with-markers.zip",
	1: "https://tibiamaps.github.io/tibia-map-data/minimap-without-markers.zip",
	2: "https://tibiamaps.github.io/tibia-map-data/minimap-with-grid-overlay-and-markers.zip",
	3: "https://tibiamaps.io/downloads/minimap-with-grid-overlay-without-markers",
	4: "https://tibiamaps.github.io/tibia-map-data/minimap-with-grid-overlay-and-poi-markers.zip",
}

var mapLocations = map[string]string{
	"mac":     "Contents/Resources/minimap",
	"windows": "minimap",
	"linux":   "minimap",
}

func (a *App) DownloadMaps(kind int) {
	a.totalBytes = 0
	a.downloadedBytes = 0
	a.totalFiles = 1
	a.downloadedFiles = 0
	a.logger.Infof("Downloading %s", mapKinds[kind])
	err := a.downloadZip(mapKinds[kind], mapLocations[a.OS()], true)
	if err != nil {
		a.logger.Errorf("Error downloading %s: %v", mapKinds[kind], err)
		return
	}
}

func (a *App) NeedsUpdate() bool {
	a.refreshManifests()
	files, err := a.filesToUpdate()
	if err != nil {
		a.logger.Errorf("Error checking for updates: %v", err)
		return false
	}
	return len(files) > 0
}

func (a *App) appDirectory() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		a.logger.Errorf("Error getting config directory: %v", err)
		return ""
	}
	appName := a.appName
	if a.OS() == "mac" {
		appName = a.appName + ".app"
	}
	return filepath.Join(configDir, appName)
}

func (a *App) filesToUpdate() ([]File, error) {
	var files []File
	filesTocheck := append(a.assetsInfo.Files, a.clientInfo.Files...)

	mutex := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(filesTocheck))

	for _, file := range filesTocheck {
		go func(file File) {
			defer wg.Done()

			localFilePath := filepath.Join(a.appDirectory(), file.LocalFile)
			if !fileExists(localFilePath) {
				a.logger.Infof("File %s does not exist", localFilePath)
				mutex.Lock()
				files = append(files, file)
				mutex.Unlock()
			} else {
				localHash, err := sha256Sum(localFilePath)
				if err != nil {
					a.logger.Errorf("Error reading local file: %s\n", err)
					return
				}

				if localHash != file.UnpackedHash {
					a.logger.Infof("File %s has changed (local: %s, remote: %s)", localFilePath, string(localHash), file.UnpackedHash)
					mutex.Lock()
					files = append(files, file)
					mutex.Unlock()
				}
			}
		}(file)
	}

	wg.Wait()

	return files, nil
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func readJSON(s string, d interface{}) error {
	contents, err := os.ReadFile(s)
	if err != nil {
		return err
	}
	err = json.Unmarshal(contents, &d)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) downloadZip(url, dst string, progress bool) error {
	dst = filepath.Join(a.appDirectory(), dst)
	err := os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	out, err := os.Create(filepath.Join(os.TempDir(), filepath.Base(dst)))
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return err
	}

	a.totalBytes = resp.ContentLength

	var reader io.Reader = resp.Body
	if progress {
		reader = io.TeeReader(reader, a)
	}
	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}
	out.Close()

	err = unzip(out.Name(), filepath.Dir(dst))
	if err != nil {
		return err
	}

	a.downloadedFiles++

	return nil
}

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			err := os.MkdirAll(filepath.Join(dst, f.Name), 0755)
			if err != nil {
				return err
			}
			continue
		}

		err := os.MkdirAll(filepath.Join(dst, filepath.Dir(f.Name)), 0755)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(filepath.Join(dst, f.Name))
		if err != nil {
			return err
		}

		_, err = io.Copy(out, rc)
		if err != nil {
			return err
		}

		out.Close()
		rc.Close()
	}

	return nil
}

func (a *App) downloadFile(url, dst string, progress bool) error {
	a.logger.Infof("Downloading %s to %s", url, dst)
	dst = filepath.Join(a.appDirectory(), dst)
	err := os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return err
	}

	var reader io.Reader = resp.Body
	if progress {
		reader = io.TeeReader(reader, a)
	}

	if filepath.Ext(dst) != ".lzma" && filepath.Ext(url) == ".lzma" {
		lzmaReader, err := lzma.NewReader(reader)
		if err != nil {
			return err
		}
		reader = lzmaReader
	}

	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}

	atomic.AddInt64(&a.downloadedFiles, 1)

	return nil
}

func (a *App) localExecutable() string {
	name := "Contents/MacOS/client-local"
	if a.OS() == "windows" {
		name = "bin/client-local.exe"
	}
	if a.OS() == "linux" {
		name = "bin/client-local"
	}
	return filepath.Join(a.appDirectory(), name)
}

func (a *App) executable() string {
	return filepath.Join(a.appDirectory(), a.clientInfo.Executable)
}

func (a *App) Play(local bool) {
	executable := a.executable()
	if local {
		executable = a.localExecutable()
	}
	a.logger.Infof("Launching %s", executable)
	os.Chmod(a.executable(), 0755)
	if err := syscall.Exec(executable, []string{"--battleeye"}, os.Environ()); err != nil {
		a.logger.Errorf("Failed to launch %s: %s | attempting regular fork", executable, err)
		cmd := exec.Command(executable, "--battleeye")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		if err := cmd.Start(); err != nil {
			a.logger.Errorf("Failed to launch %s: %s", executable, err)
		}
		os.Exit(0)
	}
}

func (a *App) Write(p []byte) (n int, err error) {
	n = len(p)
	atomic.AddInt64(&a.downloadedBytes, int64(n))
	return
}

func sha256Sum(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
