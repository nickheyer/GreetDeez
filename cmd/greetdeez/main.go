package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"

	greetdeezv1 "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
	"github.com/nickheyer/greetdeez/internal/config"
	"github.com/nickheyer/greetdeez/internal/display"
	"github.com/nickheyer/greetdeez/internal/greetd"
	"github.com/nickheyer/greetdeez/internal/server"
	"github.com/nickheyer/greetdeez/pkg/rpc"
	"github.com/nickheyer/greetdeez/pkg/webview"
	uipkg "github.com/nickheyer/greetdeez/ui"
)

func main() {
	logFile, err := os.OpenFile("/tmp/greetdeez.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	devMode := flag.Bool("dev", false, "Enable dev mode (debug inspector, tolerate missing GREETD_SOCK)")
	devUI := flag.String("dev-ui", "", "Navigate webview to this URL instead of embedded UI (e.g. http://localhost:5173)")
	configPath := flag.String("config", config.DefaultConfigPath, "Path to config file")
	flag.Parse()

	cfg := loadConfig(*configPath)

	client := connectGreetd(*devMode)
	defer client.Close()

	// Fix cage's missing output scale BEFORE GTK init.
	// Cage never sets wl_output.scale, so it defaults to 1 on DRM.
	// We use wlr-randr to set it via the wlr-output-management protocol.
	if !*devMode {
		display.ConfigureOutputScale()
	}

	navURL := *devUI
	if navURL == "" {
		uiFS, err := resolveUIFS(cfg.UI.Path, cfg.UI.Theme)
		if err != nil {
			slog.Error("failed to resolve UI", "error", err)
			os.Exit(1)
		}
		addr, err := serveUI(uiFS)
		if err != nil {
			slog.Error("failed to serve UI", "error", err)
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
		display.Setup(w)
		display.Harden(w)
	}

	srv := server.New(client, &cfg)
	dispatcher := rpc.NewDispatcher(cfg.Debug)
	greetdeezv1.RegisterGreeterServiceServer(dispatcher, srv)
	w.Bind("__greetdeez_rpc__", dispatcher.WebViewHandler())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		w.Terminate()
	}()

	go func() {
		w.Dispatch(func() {
			slog.Info("navigating webview", "url", navURL)
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

// gzipResponseWriter wraps http.ResponseWriter to compress responses.
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// resolveUIFS returns the filesystem to serve: custom path if configured, embedded theme otherwise.
func resolveUIFS(customPath, theme string) (http.FileSystem, error) {
	if customPath != "" {
		info, err := os.Stat(customPath)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("ui path %q is not a directory", customPath)
		}
		slog.Info("using custom UI", "path", customPath)
		return http.Dir(customPath), nil
	}

	sub, err := uipkg.BuildFS(theme)
	if err != nil {
		return nil, fmt.Errorf("embedded fs: %w", err)
	}
	return http.FS(sub), nil
}

// serveUI starts an HTTP server for the given filesystem on a random port.
func serveUI(fsys http.FileSystem) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	fs := http.FileServer(fsys)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)

		if ext == ".js" || ext == ".css" || ext == ".woff2" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

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

	slog.Info("serving UI", "addr", listener.Addr().String())
	return listener.Addr().String(), nil
}
