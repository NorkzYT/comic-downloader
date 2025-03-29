<p align="center">
    <img src="docs/content/assets/img/comic-downloader-cover-rl.png" width="490" alt="Comic Downloader">
</p>

<p align="center">
  Fast, reliable, and easy-to-use CLI tool for downloading comics (manga, manhwa, and more) from popular websites.
</p>

<div align="center">
  <!-- Contributions Welcome -->
  <a href="CODE_OF_CONDUCT.md" target="_blank">
    <img src="https://img.shields.io/badge/contributions-welcome-brightgreen?logo=github" alt="Contributions Welcome">
  </a>
  <!-- Commits per Month -->
  <a href="https://github.com/NorkzYT/comic-downloader/pulse" target="_blank">
    <img src="https://img.shields.io/github/commit-activity/m/NorkzYT/comic-downloader" alt="Commits per Month">
  </a>
  <!-- License -->
  <a href="https://github.com/NorkzYT/comic-downloader/blob/main/LICENSE" target="_blank">
    <img src="https://img.shields.io/badge/license-GNUv3-purple" alt="License">
  </a>
  <!-- Contributor Covenant -->
  <a href="https://contributor-covenant.org/version/2/1/code_of_conduct/" target="_blank">
    <img src="https://img.shields.io/badge/Contributor%20Covenant-2.1-purple" alt="Contributor Covenant 2.1">
  </a>
  <!-- GitHub Stars -->
  <a href="https://github.com/NorkzYT/comic-downloader/stargazers" target="_blank">
    <img src="https://img.shields.io/github/stars/NorkzYT/comic-downloader" alt="GitHub Stars">
  </a>
</div>

---

## 📚 Table of Contents

<details>
<summary><strong>Expand Table of Contents</strong></summary>

- [Supported Websites](#-supported-websites)
- [Installation](#-installation)
  - [Linux & macOS](#linux--macos)
  - [Windows](#%EF%B8%8F-windows)
  - [Docker](#-docker)
- [Usage](#-usage)
  - [Basic Usage](#basic-usage)
  - [Chapter Range](#chapter-range)
  - [Language Selection](#language-selection)
  - [Bundling Chapters](#bundling-chapters)
  - [Help](#help)
- [Troubleshooting](#%EF%B8%8F-troubleshooting)
- [Contribution](#-contribution)
- [Star History](#-star-history)

</details>

---

## 🌐 Supported Websites

<details>
<summary><strong>Expand Supported Websites</strong></summary>

Currently, the following sites are supported:

- [Asura Scans](https://asuracomic.net)
- [ChapManganato](https://chapmanganato.to)
- [InManga](https://inmanga.com)
- [LHTranslation](https://lhtranslation.net)
- [LSComic](https://lscomic.com)
- [Manga Monks](https://mangamonks.com)
- [Mangabat](https://mangabat.com)
- [Mangadex](https://mangadex.org)
- [Mangakakalot](https://mangakakalot.com) / [.tv](https://mangakakalot.tv)
- [Manganato](https://manganato.com)
- [Manganelo](https://manganelo.com) / [.tv](https://manganelo.tv)
- [Mangapanda](https://mangapanda.in)
- [ReadMangabat](https://readmangabat.com)
- [TCBScans](https://tcbscans.com) / [.net](https://www.tcbscans.net) / [.org](https://www.tcbscans.org)

If a site you use isn't listed, please [open an issue](https://github.com/NorkzYT/comic-downloader/issues) or contribute directly via pull request.

</details>

---

## 🚀 Installation

### Linux & macOS

1. Download the latest binary from the [Releases page](https://github.com/NorkzYT/comic-downloader/releases).
2. Extract the downloaded archive.
3. Move the binary into a directory within your system's `PATH`:

```bash
sudo mv comic-downloader /usr/local/bin/
```

#### macOS Users

To bypass the Gatekeeper security prompt, run:

```bash
sudo spctl --master-disable
```

### 🖥️ Windows

1. Download the latest `.exe` from [Releases](https://github.com/NorkzYT/comic-downloader/releases).
2. Place the `.exe` in a directory in your system's `PATH` (e.g., `C:\Windows\System32`).

Run via Command Prompt:

```cmd
comic-downloader [URL] [range]
```

### 🐳 Docker

Run using Docker:

```bash
docker run --rm -it -v "/opt/comic-downloader:/downloads" NorkzYT/comic-downloader --help
```

> **Note:** Downloads will be saved in your current working directory.

---

## 💻 Usage

### Basic Usage

Interactive download of all chapters:

```bash
comic-downloader [URL]
```

The URL must be the series' main page.

### Chapter Range

Specify specific chapters or ranges:

```bash
comic-downloader [URL] 1-50
```

### Language Selection

Explicitly select a language:

```bash
comic-downloader [URL] 1-10 --language es
```

### Bundling Chapters

Combine chapters into a single `.cbz` file:

```bash
comic-downloader [URL] 1-8 --bundle
```

### Help

View all commands and options:

```bash
comic-downloader --help
```

---

## 🛠️ Troubleshooting

- **"Command not recognized":** Verify the binary is in a PATH-accessible location.
- **macOS unsigned binary error:** Run `sudo spctl --master-disable`.

---

## 🤝 Contribution

Contributions, issues, and pull requests are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## 📈 Star History

<a href="https://star-history.com/#NorkzYT/comic-downloader">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=NorkzYT/comic-downloader&type=Date&theme=dark">
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=NorkzYT/comic-downloader&type=Date">
    <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=NorkzYT/comic-downloader&type=Date">
  </picture>
</a>
