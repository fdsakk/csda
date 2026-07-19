package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	apiweb "github.com/fdsakk/csda/pkg/web"
	webassets "github.com/fdsakk/csda/web"
)

const appDirectoryName = "CS Demo Analyzer"

func runApp(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "csda app does not accept arguments")
		return 2
	}

	assets := webassets.Dist()
	if _, err := fs.Stat(assets, "index.html"); err != nil {
		fmt.Fprintln(os.Stderr, "web UI is not included in this build; run the production web build before compiling csda")
		return 1
	}
	dataRoot, err := localAppDataRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dataRoot = filepath.Join(dataRoot, appDirectoryName)
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	webServer, err := apiweb.NewServer(apiweb.Options{
		DatabasePath: filepath.Join(dataRoot, "player-stats.db"),
		UploadsPath:  filepath.Join(dataRoot, "uploads"),
		TrisDir:      filepath.Join(filepath.Dir(executable), "tris"),
		Assets:       assets,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer webServer.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	server := &http.Server{Handler: webServer.Handler(), ReadHeaderTimeout: 15 * time.Second}
	serverErr := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErr <- serveErr
		}
		close(serverErr)
	}()

	url := "http://" + listener.Addr().String()
	fmt.Printf("CS Demo Analyzer: %s\n", url)
	fmt.Printf("Data: %s\n", dataRoot)
	fmt.Println("Close this window or press Ctrl+C to stop the application.")
	if err := openBrowser(url); err != nil {
		fmt.Fprintf(os.Stderr, "could not open the browser automatically: %v\n", err)
	}

	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopped)
	select {
	case <-stopped:
	case err := <-serverErr:
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Stop the analyzer and SSE streams before waiting for HTTP connections;
	// an open dashboard tab otherwise keeps Shutdown blocked until timeout.
	webServer.Close()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
