<script lang="ts">
  import Select from 'svelte-select';
  import logo from "./assets/images/logo-universal.png";
  import {
    ActiveDownload,
    DownloadMaps,
    DownloadPercent,
    DownloadedBytes,
    DownloadedFiles,
    LocalEnabled,
    NeedsUpdate,
    Play,
    Revision,
    TotalBytes,
    TotalFiles,
    Update,
    Version,
  } from "../wailsjs/go/main/App.js";
  import { onMount } from "svelte";
  import PlayIcon from "./PlayIcon.svelte";
  import UpdateIcon from "./UpdateIcon.svelte";
  import DownloadIcon from "./DownloadIcon.svelte";
  import SettingsIcon from "./SettingsIcon.svelte";

  export let openSettings: () => void;

  let version: string = "";
  let revision: number = 0;
  let updating: boolean = false;
  let ready: boolean = false;
  let needsUpdate: boolean = false;

  let progress: number = 0;
  let totalFiles: number = 0;
  let totalBytes: number = 0;
  let downloadedFiles: number = 0;
  let downloadedBytes: number = 0;
  let activeDownload: string = "";

  let mapKind = 0;

  let hasLocal = false;

  onMount(async () => {
    revision = await Revision();
    version = await Version();
    needsUpdate = await NeedsUpdate();
    ready = true;
    hasLocal = await LocalEnabled();
  });

  function update() {
    totalFiles = 0;
    totalBytes = 0;
    downloadedBytes = 0;
    downloadedFiles = 0;
    void Update();
    updating = true;

    const interval = setInterval(async () => {
      totalFiles = await TotalFiles();
      totalBytes = await TotalBytes();
      downloadedBytes = await DownloadedBytes();
      downloadedFiles = await DownloadedFiles();
      activeDownload = await ActiveDownload();
      progress = await DownloadPercent();

      if (downloadedFiles === totalFiles) {
        updating = false;
        needsUpdate = false;
        clearInterval(interval);
      }
    }, 1000);
  }

  function downloadMaps() {
    totalFiles = 0;
    totalBytes = 0;
    downloadedBytes = 0;
    downloadedFiles = 0;
    void DownloadMaps(mapKind);
    updating = true;

    const interval = setInterval(async () => {
      totalFiles = await TotalFiles();
      totalBytes = await TotalBytes();
      downloadedBytes = await DownloadedBytes();
      downloadedFiles = await DownloadedFiles();
      activeDownload = await ActiveDownload();
      progress = await DownloadPercent();

      if (progress === 100) {
        updating = false;
        needsUpdate = false;
        clearInterval(interval);
      }
    }, 1000);
  }

  function formatBytes(bytes: number, decimals = 2) {
    if (!+bytes) return "0 Bytes";

    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = [
      "Bytes",
      "KiB",
      "MiB",
      "GiB",
      "TiB",
      "PiB",
      "EiB",
      "ZiB",
      "YiB",
    ];

    const i = Math.floor(Math.log(bytes) / Math.log(k));

    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
  }

  function play() {
    ready = false;
    Play(false);
  }

  function playLocal() {
    ready = false;
    Play(true);
  }

  const mapTypes = [
    { value: 0, label: 'Full w/ markers' },
    { value: 1, label: 'Full w/o markers' },
    { value: 2, label: 'Overlayed w/ markers' },
    { value: 3, label: 'Overlayed w/o markers' },
    { value: 4, label: 'Overlayed w/ markers (+PoI)' },
  ]

</script>

<button class="settings" on:click={openSettings} disabled={updating}>
  <SettingsIcon />
</button>
<div>
  <img alt="Logo" id="logo" src={logo} />
  <div class="actions">
    <div>
      <h3>Play</h3>
      {#if updating}
        <button class="update" disabled>
          <div>
            {downloadedFiles} / {totalFiles} files
          </div>
          <div>
            {formatBytes(downloadedBytes)} / {formatBytes(totalBytes)}
          </div>
        </button>
      {:else if needsUpdate}
        <div>
          <button class="update" on:click={update} disabled={!ready}>
            <UpdateIcon />
          </button>
          Update available.
        </div>
      {:else}
        <div>
          <div class="row">
            <button
              class="play"
              class:withLocal={hasLocal}
              disabled={!ready}
              on:click={play}
            >
              <PlayIcon />
              {#if hasLocal}
                Remote
              {:else}
                {version} + rev.{revision}
              {/if}
            </button>
            {#if hasLocal}
              <button
                class="play"
                class:local={hasLocal}
                disabled={!ready}
                on:click={playLocal}
              >
                <PlayIcon />
                Local
              </button>
            {/if}
          </div>
          {#if ready}Up to date.{:else}Loading...{/if}
        </div>
      {/if}
    </div>
    <div class="maps">
      <h3>Maps</h3>
      <div class="map-select">
        <Select bind:value={mapKind} items={mapTypes} />
      </div>
      <button disabled={!ready || needsUpdate} on:click={downloadMaps}>
        <DownloadIcon />
        Download + Install Maps
      </button>
    </div>
  </div>

  {#if updating}
    <div class="progress-section">
      <div class="progress-bar">
        <div class="progress" style="width: {progress}%" />
        <div class="active-download">{activeDownload}</div>
      </div>
    </div>
  {/if}
</div>

<style>
  .progress-section {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }

  div.progress-bar {
    position: relative;
    align-items: start;
    justify-content: start;
    width: 512px;
    height: 32px;
    background-color: #333333;
    border-radius: 8px;
    margin: 8px 0;
  }

  .active-download {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    color: white;
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    padding: 0 8px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .progress {
    height: 100%;
    background-color: #016f4e;
    border-radius: 8px;
    transition: width 0.5s ease-in-out;
  }

  div {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
  }

  button {
    background: none;
    border: none;
    cursor: pointer;
    padding: 8px;
    width: 200px;
    height: 74px;
    color: white;
    border-radius: 8px;
    box-shadow: #333333 0px 0px 4px 0px;
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: center;
  }

  button.update {
    flex-direction: column;
    background-color: #f4b343;
  }
  button.update:disabled {
    flex-direction: column;
    padding: 12px;
  }

  button:disabled {
    color: #ccc;
    background-color: #333333;
    opacity: 0.75;
  }

  button.play {
    background-color: #016f4e;
  }

  #logo {
    display: block;
    width: 148px;
    height: 148px;
    margin: auto;
    padding: 3% 0 0;
    background-position: center;
    background-repeat: no-repeat;
    background-size: 100% 100%;
    background-origin: content-box;
  }

  .actions {
    display: flex;
    flex-direction: row;
    align-items: start;
    gap: 8px;
    width: 100%;
  }

  .maps {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .map-select {
    width: 200px;
    --border-radius: 16px;
    --list-border-radius: 16px;
    --item-color: #4e3bf5;
    --item-hover-color: #4e3bf5;
    --placeholder-color: #4e3bf5;
    --selected-item-color: #4e3bf5;
  }

  h3 {
    margin: 0;
    padding: 0;
    font-size: 16px;
  }

  .maps button {
    width: 100%;
    height: 24px;
    background-color: #4e3bf5;
  }

  .withLocal {
    width: 90px;
    display: flex;
    flex-direction: column;
  }

  .play.local {
    background-color: #ba3bf5;
    width: 90px;
    display: flex;
    flex-direction: column;
  }

  .row {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  button.settings {
    position: absolute;
    top: 0;
    right: 0;
    width: 48px;
    height: 48px;
    margin: 8px;
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: center;
    box-shadow: none;
  }
</style>
