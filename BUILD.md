# Build- und Release-Dokumentation für Chisel

Dieses Dokument beschreibt die Build-Optionen, das Docker-Image, das Multi-Plattform-Kompilieren und das Deployment in die Container-Registry.

Repository: [https://github.com/panzerdev/chisel](https://github.com/panzerdev/chisel)

---

## Inhaltsverzeichnis
1. [Lokales Docker-Image bauen (Minimales Alpine)](#1-lokales-docker-image-bauen)
2. [Image in Harbor-Registry pushen](#2-image-in-harbor-registry-pushen)
3. [Multi-Plattform-Binaries bauen](#3-multi-plattform-binaries-bauen)
4. [Plattform-Übersicht & Raspberry Pi Kompatibilität](#4-plattform-übersicht--raspberry-pi-kompatibilität)

---

## 1. Lokales Docker-Image bauen

Das [**`Dockerfile`**](Dockerfile) verwendet einen mehrstufigen Build (*Multi-Stage*):
* **Build-Stage (`golang:alpine`)**: Kompiliert Chisel statisch ohne CGO (`CGO_ENABLED=0`) und mit gestrippten Symbolen (`-ldflags "-s -w"`). Nutzt Dependency-Caching für `go.mod` und `go.sum`.
* **Runtime-Stage (`alpine:3.21`)**: Minimales Image (unter **9 MB** Download / ca. **31 MB** entpackt) mit `ca-certificates` und `tzdata`.
* **Sicherheit**: Läuft standardmäßig als unprivilegierter Non-Root-Benutzer (`USER chisel`).

### Befehle:
```bash
# Über Makefile:
make docker

# Oder direkt mit Docker:
docker build -t chisel:latest .
```

### Container starten:

**Als Server mit Admin-Web-Interface:**
```bash
docker run -d --name chisel-server \
  -p 8080:8080 -p 9000:9000 \
  chisel:latest server --port 8080 --admin 0.0.0.0:9000 --reverse
```

**Als Client verbinden:**
```bash
docker run -d --name chisel-client \
  chisel:latest client http://<server-ip>:8080 3000:80
```

---

## 2. Image in Harbor-Registry pushen

Für den Build und Push in die private Registry steht das Skript [**`build-and-push.sh`**](build-and-push.sh) zur Verfügung. Es taggt das Image automatisch mit dem gewünschten Versions-Tag **und** `latest`.

* **Registry-Pfad:** `harbor.panzer.zone/images/chisel`

### Vorbereitung:
Vor dem ersten Push an der Registry anmelden:
```bash
docker login harbor.panzer.zone
```

### Ausführung:
```bash
# Über Makefile:
make docker-push TAG=v1.0.0

# Oder direkt über das Skript:
./build-and-push.sh v1.0.0
```

Dabei werden folgende Tags gebaut und gepusht:
* `harbor.panzer.zone/images/chisel:v1.0.0`
* `harbor.panzer.zone/images/chisel:latest`

---

## 3. Multi-Plattform-Binaries bauen

Über [**`Dockerfile.build`**](Dockerfile.build) und das Skript [**`build-all.sh`**](build-all.sh) können alle Binaries für sämtliche gängigen Betriebssysteme und Architekturen in einem Durchgang kompiliert werden:

* Der Build läuft komplett isoliert im Docker-Container – **keine lokalen Go-Toolchains oder Cross-Compiler auf dem Host erforderlich**.
* Verwendet BuildKit (`--output build/`), sodass die fertigen Binaries direkt auf dem Host im Ordner `build/` mit den Berechtigungen des aktuellen Benutzers landen.
* Der Ordner `build/` ist in der [`.gitignore`](.gitignore) eingetragen und wird nicht versioniert.
* Erzeugt automatisch eine `SHA256SUMS.txt` mit Prüfsummen aller Binaries.

### Befehle:
```bash
# Automatisch Git-Tag / Commit als Versionsnummer verwenden:
make build-all

# Oder mit explizitem Versions-Tag:
make build-all VERSION=v1.0.0

# Direkt via Skript:
./build-all.sh v1.0.0
```

---

## 4. Plattform-Übersicht & Raspberry Pi Kompatibilität

Der Multi-Plattform-Build deckt alle gängigen Architekturen ab:

| Binary | Betriebssystem | Architektur | Typische Zielgeräte |
| :--- | :--- | :--- | :--- |
| **`chisel_linux_armv6`** | Linux | ARMv6 (`GOARM=6`) | **Raspberry Pi Zero, Zero W, Raspberry Pi 1** (BCM2835) |
| **`chisel_linux_armv7`** | Linux | ARMv7 (`GOARM=7`) | Raspberry Pi 2, 3, 4 (32-Bit Raspberry Pi OS) |
| **`chisel_linux_arm64`** | Linux | ARM64 (`aarch64`) | **Raspberry Pi Zero 2 W**, Pi 3/4/5 (64-Bit OS), AWS Graviton |
| **`chisel_linux_armv5`** | Linux | ARMv5 (`GOARM=5`) | Ältere Embedded-Boards & IoT-Geräte |
| **`chisel_linux_amd64`** | Linux | x86_64 / 64-Bit | Standard Linux-Server, PCs, Cloud-VMs |
| **`chisel_linux_386`** | Linux | x86 / 32-Bit | Ältere x86-Systeme |
| **`chisel_linux_ppc64le`**| Linux | PowerPC 64-Bit LE | IBM POWER Server |
| **`chisel_linux_s390x`** | Linux | IBM Z Mainframe | s390x Mainframe-Instanzen |
| **`chisel_linux_mips*`** | Linux | MIPS / MIPSLE | Router & Access Points (OpenWrt, etc.) |
| **`chisel_darwin_arm64`**| macOS | Apple Silicon | Apple M1, M2, M3, M4 Macs |
| **`chisel_darwin_amd64`**| macOS | Intel 64-Bit | Intel-basierte Macs |
| **`chisel_windows_amd64.exe`** | Windows | x86_64 / 64-Bit | Moderne 64-Bit Windows-PCs & Server |
| **`chisel_windows_arm64.exe`** | Windows | ARM64 | Windows on ARM (Surface Pro X, etc.) |
| **`chisel_windows_386.exe`**   | Windows | x86 / 32-Bit | Ältere 32-Bit Windows-Systeme |
| **`chisel_freebsd_amd64`**| FreeBSD | x86_64 / 64-Bit | FreeBSD-Server, pfSense / OPNsense |
| **`chisel_freebsd_arm64`**| FreeBSD | ARM64 | FreeBSD auf ARM64 |
| **`chisel_freebsd_386`**  | FreeBSD | x86 / 32-Bit | 32-Bit FreeBSD |

> [!TIP]
> **Hinweis für Raspberry Pi Zero & Zero W (v1.x):**
> Auf dem originalen Pi Zero / Zero W muss zwingend **`chisel_linux_armv6`** verwendet werden. Neuere Binaries (`armv7` oder `arm64`) schlagen dort mit dem Fehler `Illegal instruction` fehl. Auf dem neueren **Raspberry Pi Zero 2 W** (64-Bit Cortex-A53) kann direkt `chisel_linux_arm64` (oder `armv7` bei 32-Bit-OS) eingesetzt werden.
