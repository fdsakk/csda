# CS Demo Analyzer

CS Demo Analyzer is a local application for analyzing CS2 and CS:GO demo files.
Its browser dashboard builds a persistent player database, aggregates statistics
across many matches, and highlights timing or accuracy evidence worth reviewing.
It also includes a command-line analyzer and Go/Node.js APIs for exporting raw
demo data.

The application runs locally. Uploaded `.dem` files are processed on your
machine, while the resulting statistics are stored in SQLite. The review score
is a prioritization aid, not a cheating probability or an automatic verdict.

## Installation and startup

Choose one of the methods below. Docker is the simplest cross-platform option;
the Windows installer is the simplest native desktop option.

### Linux

#### Option 1: Docker Compose

Requirements:

- A 64-bit Linux system
- Docker Engine with the Docker Compose plugin
- Port `8080` available locally

Steps:

1. Clone the repository and enter it:

   ```bash
   git clone https://github.com/fdsakk/csda.git
   cd csda
   ```

2. Build and start the application:

   ```bash
   docker compose up --build -d
   ```

3. Open [http://localhost:8080](http://localhost:8080) in a browser.
4. Drop one or more `.dem` files into the upload area.

The database is persisted in the Docker volume `csda-data`. Useful commands:

```bash
# Follow application logs
docker compose logs -f

# Stop the application without deleting its database
docker compose down

# Rebuild after pulling a new version
git pull
docker compose up --build -d
```

#### Option 2: Native development startup with `run.sh`

This method builds the dashboard and runs the Go server directly on Linux.

Requirements:

- Git
- Go 1.23 or newer
- Bun 1.3.0 or newer
- Port `8080` available locally

Steps:

1. Clone the repository and enter it:

   ```bash
   git clone https://github.com/fdsakk/csda.git
   cd csda
   ```

2. Start the application:

   ```bash
   ./run.sh
   ```

   If the executable bit was lost while copying the repository, restore it with
   `chmod +x run.sh` first.

3. Open [http://127.0.0.1:8080](http://127.0.0.1:8080).
4. Stop the server with `Ctrl+C`.

Extra server arguments are forwarded by the script. For example:

```bash
./run.sh --addr=127.0.0.1:9000 --db=player-stats.db --uploads=uploads
```

The native method stores `player-stats.db` and `uploads/` in the repository by
default.

### Windows

#### Option 1: Windows installer

Requirements:

- 64-bit Windows 10 or Windows 11
- Permission to install applications
- A modern web browser

Steps:

1. Open the [latest release](https://github.com/fdsakk/csda/releases/latest).
2. Download `csda-windows-x64-setup.exe`.
3. Run the downloaded installer and approve the Windows security prompt.
4. Choose the installation directory and optionally enable the desktop shortcut.
5. Finish the wizard, then launch **CS Demo Analyzer** from the Start Menu or
   desktop shortcut.
6. The application starts a local server on a free loopback port and opens the
   dashboard in your default browser. Keep its console window open while using
   the application; closing it stops the server.

The installer places the program under Program Files and creates an uninstaller.
The database and temporary uploads are kept under
`%LOCALAPPDATA%\CS Demo Analyzer`. Updating or uninstalling the application does
not delete that database.

The same release also provides `windows-x64.zip`. For portable use, download the
archive, extract the complete archive to a writable directory, and double-click
`csda.exe`. Keep the included `tris` directory next to the executable; it contains
the map geometry required for demo analysis.

#### Option 2: Docker Desktop

Requirements:

- 64-bit Windows 10 or Windows 11
- Docker Desktop using Linux containers (WSL 2 backend recommended)
- Git
- Port `8080` available locally

Steps in PowerShell:

1. Start Docker Desktop and wait until its engine is running.
2. Clone and enter the repository:

   ```powershell
   git clone https://github.com/fdsakk/csda.git
   cd csda
   ```

3. Build and start the application:

   ```powershell
   docker compose up --build -d
   ```

4. Open [http://localhost:8080](http://localhost:8080).
5. To stop it later without deleting the database, run:

   ```powershell
   docker compose down
   ```

Docker stores the database in the `csda-data` volume.

### Optional authentication

By default, the dashboard is unauthenticated and only intended for local use.
To enable HTTP Basic Auth for either Docker or `run.sh`:

1. Copy `.env.example` to `.env`.
2. Set both values to strong credentials:

   ```dotenv
   CSDA_AUTH_USER=admin
   CSDA_AUTH_PASSWORD=replace-with-a-strong-password
   ```

3. Restart the application.

Only `/api/health` remains unauthenticated. Put the service behind HTTPS before
exposing it outside a trusted network.

## CLI and API reference

Ready-to-use Windows binaries are available on the
[releases page](https://github.com/fdsakk/csda/releases). On Linux and macOS,
use Docker or build the application from source.

### Command-line usage

#### Options

```
csda -help

Usage of csda:
  -demo-path string
        Demo file path (mandatory)
  -format string
        Export format, valid values: [csv,json,csdm] (default "csv")
  -minify
        Minify JSON file, it has effect only when -format is set to json
  -output string
        Output folder or file path, must be a folder when exporting to CSV (mandatory)
  -positions
        Include entities (players, grenades...) positions (default false)
  -source string
        Force demo's source, valid values: [challengermode,ebot,esea,esl,esportal,faceit,fastcup,5eplay,perfectworld,popflash,valve]
```

#### Examples

Export a demo into CSV files in the current folder.

`csda -demo-path=myDemo.dem -output=.`

Export a demo in a specific folder into a minified JSON file including entities positions.

`csda -demo-path=/path/to/myDemo.dem -output=/path/to/folder -format=json -positions -minify`

### Multi-demo player statistics

Build a persistent SQLite database from individual demos and folders. Players are merged by SteamID and demos are deduplicated by checksum.

```bash
csda stats ingest --db=player-stats.db --demo=match1.dem --demo-dir=./demos
csda stats report --db=player-stats.db --output=./stats-report --format=csv
```

`stats build` combines both operations. The report contains aggregated player and weapon statistics, estimated Time to Damage (TTD), estimated Reaction Time, a 0–100 evidence score with two review tiers (`watch` = grey zone, `cheater` = not humanly reproducible over many games), and evidence rows pointing to demo rounds and ticks. Timing is required to produce a non-zero review score; accuracy, head-hit rate and K/D can only add bounded support because they are also explained by legitimate skill. It is intended to prioritize manual review and is not a cheating probability or automatic verdict.

TTD measures the first spotted tick to first damage. Reaction Time measures the first spotted tick to first shot. In both cases three consecutive spotted ticks are required to validate the exposure, but timing starts at the first tick; samples outside `0–1000 ms` are excluded. The primary multi-demo value is the round-weighted average of each demo's median, while pooled median and P10 remain available in JSON/CSV. These are demo-derived estimates, not geometry-backed line-of-sight measurements.

Each analyzed demo also receives a conservative timing-quality check. If low TTD and reaction medians appear across at least four sufficiently sampled players, the demo is automatically disabled in aggregates. It remains visible in the Demos dialog and can be manually enabled. Existing databases are checked once during migration.

Use `--config=thresholds.json` (or `csda web --suspicion-config=thresholds.json`) to override fields returned by `api.DefaultSuspicionConfig()`. A player receives a status after at least three demos and 100 firearm shots by default.

### Local React dashboard

Windows release builds contain the React dashboard inside `csda.exe`. Double-click
the executable (or launch it without arguments) to start the server on a free
`127.0.0.1` port and open the default browser. The database and temporary upload
directory are stored under `%LOCALAPPDATA%\CS Demo Analyzer`; closing the console
window stops the application.

The portable release contains `csda.exe` and a `tris` directory with the required
map geometry. The installer installs both under Program Files, creates a Start Menu
shortcut and an uninstaller, and optionally creates a desktop shortcut. Updating
or uninstalling the program does not remove the database in `%LOCALAPPDATA%`.

To start the embedded dashboard explicitly on any supported platform:

```bash
csda app
```

For a fixed address, custom database paths, authentication, or an external web
build, use server mode:

```bash
cd web
bun install --frozen-lockfile
bun run build
cd ..
csda web --db=player-stats.db --uploads=uploads --assets=web/dist --source=valve
```

The `--assets` option is optional. Without it, `csda web` serves the dashboard
embedded at compile time.

Open `http://127.0.0.1:8080`. Drop one or more `.dem` files into the upload area; uploads are analyzed sequentially in the background, deduplicated by checksum, and added to the same SQLite player database. Job progress is streamed over Server-Sent Events and the table refreshes automatically after each completed job; failed jobs stay listed with their error until dismissed.

Uploaded `.dem` files are deleted automatically once their analysis finishes — the statistics live in the database. Re-analyzing a demo after an algorithm update therefore requires uploading the file again.

The Demos dialog distinguishes demos analyzed from a `.dem` file on this instance (`Analyzed`) from demos merged in through a stats export (`Imported`).

#### Authentication

The web UI is unauthenticated by default and binds to `127.0.0.1`. To protect an instance exposed to a network, set both variables (see `.env.example`; Docker Compose picks up a `.env` file automatically):

```bash
CSDA_AUTH_USER=admin
CSDA_AUTH_PASSWORD=change-me
```

The browser then shows its standard login prompt (HTTP Basic Auth). Only `/api/health` stays open for container health checks. Use HTTPS (e.g. a reverse proxy) when exposing the instance beyond a trusted network, as Basic Auth sends credentials with every request.

## API

### GO API

This API exposes functions to analyze/export a demo using the Go language.

#### Analyze

This function analyzes the demo located at the given path and returns a `Match`.

```go
package main

import (
	"fmt"
	"os"

	"github.com/fdsakk/csda/pkg/api"
	"github.com/fdsakk/csda/pkg/api/constants"
)

func main() {
	match, err := api.AnalyzeDemo("./myDemo.dem", api.AnalyzeDemoOptions{
		IncludePositions: true,
		Source:           constants.DemoSourceValve,
	})

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, kill := range match.Kills {
		fmt.Printf("(%d): %s killed %s with %s\n", kill.Tick, kill.KillerName, kill.VictimName, kill.WeaponName)
	}
}
```

#### Analyze and export

This function analyzes and exports a demo into the given output path.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fdsakk/csda/pkg/api"
	"github.com/fdsakk/csda/pkg/api/constants"
)

func main() {
	exePath, _ := os.Executable()
	outputPath := filepath.Dir(exePath)
	err := api.AnalyzeAndExportDemo("./myDemo.dem", outputPath, api.AnalyzeAndExportDemoOptions{
		IncludePositions: false,
		Source:           constants.DemoSourceValve,
		Format:           constants.ExportFormatJSON,
		MinifyJSON:       true,
	})

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Demo analyzed and exported in " + outputPath)
}
```

### CLI

This API exposes the command-line interface.

```go
package main

import (
	"os"

	"github.com/fdsakk/csda/pkg/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
```

## Developing

### Requirements

- [Go](https://golang.org/dl/)
- [Make](https://www.gnu.org/software/make/)

### Build

#### Windows

Build the portable executable (including the production React dashboard):

```bash
make build-windows
```

The result is `bin/windows-x64/csda.exe`. To also build
`bin/windows-x64/csda-windows-x64-setup.exe`, install Inno Setup 6 and run:

```bash
make build-windows-installer
```

Set `ISCC` when the Inno Setup compiler is not available as `iscc`, for example:

```bash
make ISCC='/c/Program Files (x86)/Inno Setup 6/ISCC.exe' build-windows-installer
```

#### macOS

`make build-darwin` / `make build-darwin-arm64`

#### Linux

`make build-linux` / `make build-linux-arm64`

### Tests

Tests compare the analyzer output against the JSON snapshots stored in `tests/snapshots`.

Every demo is an entry in the `demoTestCases` table in `tests/demos_test.go` and runs as a parallel subtest of `TestDemos`, named `TestDemos/<game>/<name>` (e.g. `TestDemos/cs2/Esportal_6008132_2023_Mirage`). To add a demo, add a row to the table rather than a new file.

First, download the demos used by the tests (one-time setup):

```bash
./download-demos.sh
```

#### Running tests

```bash
# Run all tests
make test

# Only CS:GO or only CS2 tests
make test-csgo
make test-cs2
```

Pass extra flags to `go test` through the `ARGS` variable. The `-run` value is a regex matched against the slash-separated subtest path:

```bash
# Run a single demo
make test ARGS="-run TestDemos/cs2/Esportal_6008132_2023_Mirage"

# Run every demo whose name contains Esportal, in verbose mode
make test ARGS="-run TestDemos/.*/Esportal -v"
```

#### Updating snapshots

When the analyzer output legitimately changes, regenerate the snapshots by passing `-update`:

```bash
# Regenerate all snapshots
make test ARGS="-update"

# Regenerate the snapshot for a single demo
make test ARGS="-update -run TestDemos/cs2/Esportal_6008132_2023_Mirage"
```

Review the resulting snapshot diff before committing.

### VSCode debugger

1. Inside the `.vscode` folder, copy/paste the file `launch.template.json` and name it `launch.json`
2. Place a demo in the `debug` folder
3. Update the `-demo-path` argument to point to the demo you just placed
4. Adjust the other arguments as you wish
5. Start the debugger from VSCode

## Acknowledgements

This project uses the demo parser [demoinfocs-golang](https://github.com/markus-wa/demoinfocs-golang) created by [@markus-wa](https://github.com/markus-wa) and maintained by him and [@akiver](https://github.com/akiver).

## License

[MIT](LICENSE.md)
