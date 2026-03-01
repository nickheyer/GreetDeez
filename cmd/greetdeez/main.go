package main

import (
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/nickheyer/greetdeez/internal/config"
	"github.com/nickheyer/greetdeez/internal/greetd"
	"github.com/nickheyer/greetdeez/internal/sessions"
	"github.com/nickheyer/greetdeez/internal/state"
	"github.com/nickheyer/greetdeez/pkg/binds"
	"github.com/nickheyer/greetdeez/pkg/logs"
	"github.com/nickheyer/greetdeez/pkg/webview"
	uiembed "github.com/nickheyer/greetdeez/ui/greetdeez"
)

type result struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type loginResult struct {
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`
	Messages []string `json:"messages,omitempty"`
}

func main() {
	devMode := flag.Bool("dev", false, "Enable dev mode (debug inspector, tolerate missing GREETD_SOCK)")
	devUI := flag.String("dev-ui", "", "Navigate webview to this URL instead of embedded UI (e.g. http://localhost:5173)")
	configPath := flag.String("config", config.DefaultConfigPath, "Path to config file")
	flag.Parse()

	logs := logs.InitLogger(*devMode)

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

	if *devMode {
		w.SetTitle("GreetDeez [dev]")
		w.SetSize(cfg.Window.Width, cfg.Window.Height, webview.HintNone)
	} else {
		binds.Fullscreen(w.Window())
	}

	bindFunctions(w, client, &cfg, *devMode, logs)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		w.Terminate()
	}()

	// Show a dark splash immediately to mask webkit init latency.
	// The real Svelte app loads over it once the HTTP server is ready.
	w.Navigate("data:text/html," + url.PathEscape(uiembed.SplashHTML))

	// Navigate to the real app on a goroutine so w.Run() can start the event loop.
	go func() {
		slog.Info("navigating webview", "url", navURL)
		w.Dispatch(func() {
			w.Navigate(navURL)
		})
	}()

	w.Run()
}

func loadConfig(path string) config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		slog.Error("config unmarshal failed", "path", path, "error", err)
		os.Exit(1)
	}
	validatePowerCmds(&cfg)
	return cfg
}

func validatePowerCmds(cfg *config.Config) {
	if !cfg.Power.Enabled {
		return
	}
	check := func(name string, cmd []string) {
		if len(cmd) == 0 {
			slog.Warn("no command configured for power action", "action", name)
			return
		}
		if _, err := exec.LookPath(cmd[0]); err != nil {
			slog.Warn("power command not found", "action", name, "cmd", cmd[0])
		}
	}
	check("poweroff", cfg.Power.PoweroffCmd)
	check("reboot", cfg.Power.RebootCmd)
	check("suspend", cfg.Power.SuspendCmd)
}

func connectGreetd(devMode bool) *greetd.Client {
	client, err := greetd.NewClient()
	if err != nil {
		if devMode {
			slog.Warn("greetd unavailable, running in dev mode", "error", err)
			return nil
		}
		slog.Error("greetd connection failed", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to greetd")
	return client
}

func bindFunctions(w webview.WebView, client *greetd.Client, cfg *config.Config, devMode bool, logs *logs.LogCapture) {
	w.Bind("getSessions", func() []sessions.Session {
		return sessions.List(cfg.Sessions.Dirs)
	})

	w.Bind("login", func(username, password string) loginResult {
		if client == nil {
			slog.Debug("dev: login", "username", username)
			return loginResult{OK: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Auth.Timeout())
		defer cancel()
		authResult, err := client.Authenticate(ctx, username, password)
		if err != nil {
			slog.Warn("authentication failed", "username", username, "error", err)
			return loginResult{OK: false, Error: err.Error()}
		}
		slog.Info("authenticated", "username", username)
		var msgs []string
		if authResult != nil {
			msgs = authResult.Messages
		}
		return loginResult{OK: true, Messages: msgs}
	})

	w.Bind("startSession", func(sess sessions.Session) result {
		env := buildSessionEnv(sess)
		cmd := sess.Cmd

		// Wrap X11 sessions with the configured wrapper (default: startx /usr/bin/env).
		if sess.Type == "x11" && len(cfg.Sessions.X11Wrapper) > 0 {
			cmd = append(append([]string{}, cfg.Sessions.X11Wrapper...), cmd...)
		}

		if devMode && client == nil {
			slog.Debug("dev: startSession (no-op)", "cmd", cmd, "env", env)
			return result{OK: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Auth.Timeout())
		defer cancel()
		resp, err := client.StartSession(ctx, cmd, env)
		if err != nil {
			slog.Error("session start failed", "cmd", cmd, "error", err)
			return result{OK: false, Error: err.Error()}
		}
		if resp.Type != "success" {
			slog.Error("session start rejected", "cmd", cmd, "type", resp.Type)
			return result{OK: false, Error: "session start failed"}
		}
		slog.Info("session started", "cmd", cmd, "env", env)
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
		if devMode {
			slog.Debug("dev: powerAction (no-op)", "action", action)
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

	w.Bind("getLastState", func() state.State {
		return state.Load()
	})

	w.Bind("saveState", func(s state.State) result {
		if err := state.Save(s); err != nil {
			slog.Warn("failed to save state", "error", err)
			return result{OK: false, Error: err.Error()}
		}
		return result{OK: true}
	})

	w.Bind("getLogs", func() []string {
		return logs.Lines()
	})
}

// buildSessionEnv constructs environment variables for a session based on its
// type and desktop names, matching what tuigreet/regreet do.
func buildSessionEnv(sess sessions.Session) []string {
	env := []string{"XDG_SESSION_TYPE=" + sess.Type}

	desktop := strings.ReplaceAll(sess.Desktop, ";", ":")
	desktop = strings.TrimRight(desktop, ":")
	if desktop == "" {
		desktop = strings.ToLower(sess.Name)
	}

	env = append(env,
		"XDG_SESSION_DESKTOP="+desktop,
		"XDG_CURRENT_DESKTOP="+desktop,
		"DESKTOP_SESSION="+desktop,
	)
	return env
}

// gzipResponseWriter wraps http.ResponseWriter to compress responses.
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// serveEmbeddedUI starts an HTTP server for the embedded SvelteKit build on a random port.
func serveEmbeddedUI() (string, error) {
	sub, err := uiembed.BuildFS()
	if err != nil {
		return "", fmt.Errorf("embedded fs: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	fs := http.FileServer(http.FS(sub))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)

		// Hashed assets get long-lived cache; HTML does not.
		if ext == ".js" || ext == ".css" || ext == ".woff2" {
			if strings.Contains(r.URL.Path, ".") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}

		// Gzip-compress text assets if the client supports it.
		if (ext == ".js" || ext == ".css" || ext == ".html" || ext == "") &&
			strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			fs.ServeHTTP(&gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
			return
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
