# Slender Launcher

This is the launcher for open tibia servers.

## Features

- [x] Windows
- [x] MacOS
- [x] Linux
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
Look for baseURL in `main.go` and change it to a URL that can serve your game client. The client needs to be repacked in the correct format, you can use this [client-editor](https://github.com/luan/client-editor) to do that. Checkout https://github.com/luan/tibia-client for an example packed client.

## Screenshots

<img width="761" alt="image" src="https://github.com/luan/slender-launcher/assets/223760/3a486665-3b4f-440c-acea-70ec9262c188">

<img width="761" alt="image" src="https://github.com/luan/slender-launcher/assets/223760/f49993e4-d107-470e-ae78-5fb68176a180">

<img width="761" alt="image" src="https://github.com/luan/slender-launcher/assets/223760/479cec63-b314-4db8-97b1-5c8632a2cd12">

<img width="761" alt="image" src="https://github.com/luan/slender-launcher/assets/223760/0a44735c-1f87-4e40-8a82-e80c4c48c041">

<img width="761" alt="image" src="https://github.com/luan/slender-launcher/assets/223760/2ca942ca-88b4-45b6-a0ff-70049f9cc009">
