package cli

import (
	"context"
	"encoding/json"
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
	source := fs.String("source", "", "Force demo source (default: detect automatically)")
	configPath := fs.String("suspicion-config", "", "Optional JSON suspicion threshold configuration")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *source != "" {
		if err := api.ValidateDemoSource(constants.DemoSource(*source)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	config := api.DefaultSuspicionConfig()
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err := json.Unmarshal(data, &config); err != nil {
			fmt.Fprintf(os.Stderr, "invalid suspicion config: %v\n", err)
			return 2
		}
	}
	authUser := os.Getenv("CSDA_AUTH_USER")
	authPassword := os.Getenv("CSDA_AUTH_PASSWORD")
	if (authUser == "") != (authPassword == "") {
		fmt.Fprintln(os.Stderr, "set both CSDA_AUTH_USER and CSDA_AUTH_PASSWORD to enable authentication, or neither")
		return 2
	}
	webServer, err := apiweb.NewServer(apiweb.Options{DatabasePath: *database, UploadsPath: *uploads, AssetsPath: *assets, Source: constants.DemoSource(*source), Config: config, AuthUser: authUser, AuthPassword: authPassword})
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
	if authUser != "" {
		fmt.Println("Authentication: enabled (HTTP Basic Auth)")
	} else {
		fmt.Println("Authentication: disabled (set CSDA_AUTH_USER and CSDA_AUTH_PASSWORD to enable)")
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
