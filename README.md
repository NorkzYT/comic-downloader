<p align="center">
    <img src="Docs/content/assets/img/comic-downloader-cover-rl.png" width="490">
</p>

<p align="center">Comic Downloader is a command-line tool that simplifies downloading comics from various websites into `.cbz` files, allowing convenient reading on your favorite e-reader or comic reading application.</p>
<div align="center">
  <!-- Contributions Welcome Badge -->
  <a href="CODE_OF_CONDUCT.md" target="_blank">
    <img src="https://img.shields.io/badge/contributions-welcome-brightgreen?logo=github" alt="Contributions Welcome">
  </a>
  <!-- Commits per Month -->
  <a href="https://github.com/NorkzYT/comic-downloader/pulse">
    <img src="https://img.shields.io/github/commit-activity/m/NorkzYT/comic-downloader" alt="Commits-per-month">
  </a>
  <!-- License Badge -->
  <a href="https://github.com/NorkzYT/comic-downloader/blob/main/LICENSE" target="_blank">
    <img src="https://img.shields.io/badge/license-GNUv3-purple" alt="License">
  </a>
  <!-- Contributor Covenant Badge -->
  <a href="https://contributor-covenant.org/version/2/1/code_of_conduct/" target="_blank">
    <img src="https://img.shields.io/badge/Contributor%20Covenant-2.1-purple" alt="Contributor Covenant 2.1">
  </a>
  <!-- Github Stars Badge -->
  <a href="https://github.com/NorkzYT/comic-downloader/stargazers" target="_blank">
    <img src="https://img.shields.io/github/stars/NorkzYT/comic-downloader" alt="Github Stars">
  </a>
</div>

---

## Table of Contents

- [Supported Websites](#supported-websites)
- [Installation](#installation)
  - [Linux and Mac](#linux-and-mac)
  - [Windows](#windows)
  - [Docker](#docker)
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Specifying Chapter Range](#specifying-chapter-range)
  - [Language Selection](#language-selection)
  - [Bundling Chapters](#bundling-chapters)
  - [Displaying Help](#displaying-help)
- [Troubleshooting](#troubleshooting)
- [Contribution](#contribution)
- [License](#license)
- [Star History](#star-history)

---

## Supported Websites

Currently supported websites include:

- [Asura Scans](https://asuracomic.net)
- [ChapManganato](https://chapmanganato.to)
- [InManga](https://inmanga.com)
- [LHTranslation](https://lhtranslation.net)
- [LSComic](https://lscomic.com/)
- [Manga Monks](https://mangamonks.com)
- [Mangabat](https://mangabat.com)
- [Mangadex](https://mangadex.org)
- [Mangakakalot](https://mangakakalot.com) / [.tv](https://mangakakalot.tv)
- [Manganato](https://manganato.com)
- [Manganelo](https://manganelo.com) / [.tv](https://manganelo.tv)
- [Mangapanda](https://mangapanda.in)
- [ReadMangabat](https://readmangabat.com)
- [TCBScans](https://tcbscans.com) / [.net](https://www.tcbscans.net) / [.org](https://www.tcbscans.org)

> **Note:** If a website isn't supported, please [open an issue](https://github.com/NorkzYT/comic-downloader/issues) or submit a pull request with the new implementation.

---

## Installation

### Linux and Mac

1. Download the binary from the .
2. Unarchive the file.
3. Move the binary to a folder included in your system's `PATH` to execute it from anywhere:

```bash
sudo mv comic-downloader /usr/local/bin/
```

#### Mac Users

To bypass Gatekeeper for unsigned binaries, run:

```bash
sudo spctl --master-disable
```

### Windows

1. Download the `.exe` file from .
2. Place the `.exe` file in a folder that's part of your system `PATH` (e.g., `C:\Windows\System32`).

After this, you can run the downloader directly from Command Prompt:

```cmd
comic-downloader [URL] [range]
```

### Docker

Use Docker to run the tool easily:

```bash
docker run --rm -it -v "$PWD:/downloads" NorkzYT/comic-downloader --help
```

> **Note:** Ensure `-v "$PWD:/downloads"` is set to save downloads in the current directory.

---

## Usage

### Basic Usage

Download all chapters interactively:

```bash
comic-downloader [URL]
```

The URL should point to a series index page, not individual chapters. By default, it prompts for confirmation (`y`) before downloading.

### Specifying Chapter Range

To specify exact chapters or ranges (e.g., 1,3,5-10):

```bash
comic-downloader [URL] 1-50
```

This example downloads chapters 1 to 50.

### Language Selection

Specify a language explicitly (useful for multilingual sites like MangaDex):

```bash
comic-downloader [URL] 1-10 --language es
```

This downloads chapters 1 to 10 in Spanish.

> **Note:** Parameters can be provided in any order.

### Bundling Chapters

Combine downloaded chapters into one `.cbz` file:

```bash
comic-downloader [URL] 1-8 --bundle
```

### Displaying Help

To display available commands and arguments:

```bash
comic-downloader help
```

---

## Troubleshooting

- **Issue:** Program not recognized.
  - **Solution:** Ensure binary is correctly placed in a PATH-defined directory.
- **Issue:** "Binary unsigned" error on Mac.
  - **Solution:** Run `sudo spctl --master-disable` to bypass Gatekeeper.

---

## Contribution

Contributions are welcome. Feel free to submit issues or pull requests to improve functionality or add support for new websites.


## Star history

<a href="https://star-history.com/#NorkzYT/comic-downloader">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=NorkzYT/comic-downloader&type=Date&theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=NorkzYT/comic-downloader&type=Date" />
    <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=NorkzYT/comic-downloader&type=Date" />
  </picture>
</a>
