package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/akiver/cs-demo-analyzer/pkg/api"
	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
	apiweb "github.com/akiver/cs-demo-analyzer/pkg/web"
)

func runWeb(args []string) int {
	fs := flag.NewFlagSet("csda web", flag.ContinueOnError)
	address := fs.String("addr", "127.0.0.1:8080", "HTTP listen address")
	database := fs.String("db", "player-stats.db", "SQLite player statistics database path")
	uploads := fs.String("uploads", "uploads", "Folder used to store uploaded demos")
	assets := fs.String("assets", "web/dist", "Built React assets folder")
	source := fs.String("source", "valve", "Default demo source")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := api.ValidateDemoSource(constants.DemoSource(*source)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	webServer, err := apiweb.NewServer(apiweb.Options{DatabasePath: *database, UploadsPath: *uploads, AssetsPath: *assets, Source: constants.DemoSource(*source)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer webServer.Close()
	server := &http.Server{Addr: *address, Handler: webServer.Handler(), ReadHeaderTimeout: 15 * time.Second}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopped)
	go func() {
		<-stopped
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	fmt.Printf("CS Demo Analyzer UI: http://%s\n", *address)
	fmt.Printf("Database: %s | uploads: %s\n", *database, *uploads)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
