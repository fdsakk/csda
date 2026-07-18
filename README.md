# CS Demo Analyzer

A CLI to analyze and export CS2/CS:GO demos.

## Usage

Ready-to-use binaries are available on the [releases page](https://github.com/akiver/cs-demo-analyzer/releases).

### Options

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

### Examples

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

Build the web interface and start the local Go server:

```bash
cd web
npm install
npm run build
cd ..
csda web --db=player-stats.db --uploads=uploads --assets=web/dist --source=valve
```

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

	"github.com/akiver/cs-demo-analyzer/pkg/api"
	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
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

	"github.com/akiver/cs-demo-analyzer/pkg/api"
	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
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

	"github.com/akiver/cs-demo-analyzer/pkg/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
```

### Node.js API

A Node.js module called `@akiver/cs-demo-analyzer` is available on NPM.  
It exposes a function that under the hood is a wrapper around the Go CLI.  
The module also exports TypeScript types and constants.

```js
import { analyzeDemo, DemoSource, ExportFormat } from '@akiver/cs-demo-analyzer';

async function main() {
  await analyzeDemo({
    demoPath: './myDemo.dem',
    outputFolderPath: '.',
    format: ExportFormat.JSON,
    source: DemoSource.Valve,
    analyzePositions: false,
    minify: false,
    onStderr: console.error,
    onStdout: console.log,
    onStart: () => {
      console.log('Starting!');
    },
    onEnd: () => {
      console.log('Done!');
    },
  });
}

main();
```

## Developing

### Requirements

- [Go](https://golang.org/dl/)
- [Make](https://www.gnu.org/software/make/)

### Build

#### Windows

`make build-windows`

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

[MIT](https://github.com/akiver/cs-demo-analyzer/blob/main/LICENSE.md)
