package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/nickheyer/greetdeez/internal/config"
	"github.com/nickheyer/greetdeez/internal/greetd"
	"github.com/nickheyer/greetdeez/internal/sessions"
	"github.com/nickheyer/greetdeez/pkg/binds"
	embed "github.com/nickheyer/greetdeez/ui/greetdeez"
	webview "github.com/webview/webview_go"
)

type result struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func main() {
	devMode := flag.Bool("dev", false, "Enable dev mode (debug inspector, tolerate missing GREETD_SOCK)")
	devUI := flag.String("dev-ui", "", "Navigate webview to this URL instead of embedded UI (e.g. http://localhost:5173)")
	configPath := flag.String("config", config.DefaultConfigPath, "Path to config file")
	flag.Parse()

	initLogger(*devMode)

	cfg := loadConfig(*configPath)

	client := connectGreetd(*devMode)
	defer client.Close()

	navURL := *devUI
	if navURL == "" {
		addr, err := serveEmbeddedUI()
		if err != nil {
			slog.Error("failed to serve embedded UI", "error", err)
			os.Exit(1)
		}
		navURL = fmt.Sprintf("http://%s", addr)
	}

	w := webview.New(*devMode)
	defer w.Destroy()

	binds.Fullscreen(w.Window())
	// w.SetTitle(cfg.Window.Title)
	// w.SetSize(cfg.Window.Width, cfg.Window.Height, webview.HintNone)

	bindFunctions(w, client, &cfg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		w.Terminate()
	}()

	slog.Info("navigating webview", "url", navURL)
	w.Navigate(navURL)
	w.Run()
}

func initLogger(devMode bool) {
	level := slog.LevelInfo
	if devMode {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))
}

func loadConfig(path string) config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		slog.Error("config unmarshal failed", "path", path, "error", err)
		os.Exit(1)
	}
	return cfg
}

func connectGreetd(devMode bool) *greetd.Client {
	client, err := greetd.NewClient()
	if err != nil {
		if devMode {
			slog.Warn("greetd unavailable, running in dev mode", "error", err)
		} else {
			slog.Error("greetd connection failed", "error", err)
		}
		return nil
	}
	slog.Info("connected to greetd")
	return client
}

func bindFunctions(w webview.WebView, client *greetd.Client, cfg *config.Config) {
	w.Bind("getSessions", func() []sessions.Session {
		return sessions.List(cfg.Sessions.Dirs)
	})

	w.Bind("login", func(username, password string) result {
		if client == nil {
			slog.Debug("dev: login", "username", username)
			return result{OK: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Auth.Timeout())
		defer cancel()
		if err := client.Authenticate(ctx, username, password); err != nil {
			slog.Warn("authentication failed", "username", username, "error", err)
			return result{OK: false, Error: err.Error()}
		}
		slog.Info("authenticated", "username", username)
		return result{OK: true}
	})

	w.Bind("startSession", func(cmd []string) result {
		if client == nil {
			slog.Debug("dev: startSession", "cmd", cmd)
			return result{OK: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Auth.Timeout())
		defer cancel()
		resp, err := client.StartSession(ctx, cmd)
		if err != nil {
			slog.Error("session start failed", "cmd", cmd, "error", err)
			return result{OK: false, Error: err.Error()}
		}
		if resp.Type != "success" {
			slog.Error("session start rejected", "cmd", cmd, "type", resp.Type)
			return result{OK: false, Error: "session start failed"}
		}
		slog.Info("session started", "cmd", cmd)
		return result{OK: true}
	})

	w.Bind("getHostname", func() string {
		name, _ := os.Hostname()
		return name
	})

	w.Bind("powerAction", func(action string) result {
		if !cfg.Power.Enabled {
			return result{OK: false, Error: "power actions disabled"}
		}
		if client == nil {
			slog.Debug("dev: powerAction", "action", action)
			return result{OK: true}
		}

		var args []string
		switch action {
		case "poweroff":
			args = cfg.Power.PoweroffCmd
		case "reboot":
			args = cfg.Power.RebootCmd
		case "suspend":
			args = cfg.Power.SuspendCmd
		default:
			return result{OK: false, Error: "unknown action: " + action}
		}

		if len(args) == 0 {
			return result{OK: false, Error: "no command configured for: " + action}
		}

		slog.Info("executing power action", "action", action, "cmd", args)
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			slog.Error("power action failed", "action", action, "error", err)
			return result{OK: false, Error: err.Error()}
		}
		return result{OK: true}
	})

	w.Bind("getConfig", func() config.Config {
		return *cfg
	})
}

// serveEmbeddedUI starts an HTTP server for the embedded SvelteKit build on a random port.
func serveEmbeddedUI() (string, error) {
	sub, err := embed.BuildFS()
	if err != nil {
		return "", fmt.Errorf("embedded fs: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	fs := http.FileServer(http.FS(sub))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hashed assets get long-lived cache; HTML does not.
		if ext := path.Ext(r.URL.Path); ext == ".js" || ext == ".css" || ext == ".woff2" {
			if strings.Contains(r.URL.Path, ".") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}
		fs.ServeHTTP(w, r)
	})

	go func() {
		if err := http.Serve(listener, handler); err != nil {
			slog.Error("http server error", "error", err)
		}
	}()

	slog.Info("serving embedded UI", "addr", listener.Addr().String())
	return listener.Addr().String(), nil
}
