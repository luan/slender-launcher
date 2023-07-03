# Slender Launcher

This is the launcher for open tibia servers.

## Features

- [x] Windows
- [x] MacOS
- [ ] Linux (launcher can be compiled for linux, but the game client is not available at the moment)
- [x] Auto updater
  - [x] Auto updater for the launcher
  - [x] Auto updater for the game client
- [x] Map downloader
- [x] Settings page
  - [x] Enable/disable local client launcher
  - [ ] Enable/disable test client updater
  - [ ] Change game client path
- [ ] Server status
- [ ] News view

## How to use

You'll have to modify some code to make it work for your server. The launcher is not ready to be used for any server, but it's easy to modify it.
Look for baseURL in `main.go` and change it to a URL that can serve your game client.
