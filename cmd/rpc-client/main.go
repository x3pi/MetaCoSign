package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/config"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/internal/proxy"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/setup"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
)

var defaultLogsDir string

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func main() {
	log.SetOutput(os.Stdout)
	var configPath string
	var tcpConfigPath string
	flag.StringVar(&defaultLogsDir, "logs-root", "./logs", "Root directory to store rpc-client logs (YYYY/MM/DD)")
	flag.StringVar(&configPath, "config", "config-rpc.json", "Path to the RPC configuration file")
	flag.StringVar(&tcpConfigPath, "tcp-config", "config-client-tcp.json", "Path to the TCP client configuration file")
	flag.Parse()

	if err := setup.SetupLogging(defaultLogsDir); err != nil {
		log.Fatalf("FATAL: Failed to setup logging: %v", err)
	}
	cfg, tcpCfg, err := config.Load(configPath, tcpConfigPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}
	go func() {
		logger.Info("Starting pprof server on localhost:6060")
		logger.Error(http.ListenAndServe("localhost:6060", nil))
	}()
	appCtx, err := app.New(cfg, tcpCfg)
	if err != nil {
		logger.Error("Failed to initialize application context: %v", err)
		log.Fatalf("FATAL: Application context initialization failed: %v", err)
	}
	// Initialize proxy
	rpcProxy, err := proxy.New(appCtx)
	if err != nil {
		logger.Error("Failed to initialize proxy: %v", err)
		log.Fatalf("FATAL: Proxy initialization failed: %v", err)
	}
	defer func() {
		if err := rpcProxy.Close(); err != nil {
			logger.Error("Error closing proxy resources: %v", err)
		}
	}()
	logger.Info("RPC Reverse Proxy initialized successfully")

	httpServer := setupHTTPServer(rpcProxy, cfg, defaultLogsDir)
	// TLS setup
	var tlsConfig *tls.Config
	useTLS := cfg.CertFile != "" && cfg.KeyFile != ""
	if useTLS {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			log.Fatalf("FATAL: Failed to load TLS certificate/key: %v", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		logger.Info("TLS enabled using cert: %s, key: %s", cfg.CertFile, cfg.KeyFile)
	}

	var wg sync.WaitGroup
	serverRunning := false

	if cfg.ServerPort != "" {
		serverRunning = true
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("Starting HTTP server on port %s", cfg.ServerPort)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("FATAL: HTTP server failed: %v", err)
			}
		}()
	}

	if useTLS && cfg.HTTPSPort != "" {
		serverRunning = true
		wg.Add(1)
		go func() {
			defer wg.Done()
			httpsServer := &http.Server{
				Addr:              cfg.HTTPSPort,
				Handler:           httpServer.Handler,
				TLSConfig:         tlsConfig,
				ReadTimeout:       120 * time.Second,
				WriteTimeout:      120 * time.Second,
				IdleTimeout:       360 * time.Second,
				MaxHeaderBytes:    1 << 20, // 1MB
				ReadHeaderTimeout: 10 * time.Second,
			}
			logger.Info("Starting HTTPS server on port %s", cfg.HTTPSPort)
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("FATAL: HTTPS server failed: %v", err)
			}
		}()
	}

	if !serverRunning {
		log.Fatalf("FATAL: No HTTP or HTTPS server configured to run. Exiting.")
	}

	logger.Info("RPC Reverse Proxy started successfully.")
	wg.Wait()
	logger.Info("All servers have shut down. Exiting.")
}
func setupHTTPServer(rpcProxy *proxy.RpcReverseProxy, cfg *config.Config, logsDir string) *http.Server {
	mux := http.NewServeMux()
	webPathPrefix := "/register-bls-key/"
	fs := http.FileServer(http.Dir("./dist"))
	mux.Handle(webPathPrefix, http.StripPrefix(webPathPrefix, fs))

	mux.HandleFunc("/debug/logs/list", setup.HandleRPCLogList(logsDir))
	mux.HandleFunc("/debug/logs/content", setup.HandleRPCLogContent(logsDir))
	mux.HandleFunc("/trigger-event", rpcProxy.HandleTriggerEvent)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// WebSocket upgrade handling
		if websocket.IsWebSocketUpgrade(r) {
			var finalTargetURL string
			// Readonly WebSocket endpoint
			if strings.HasPrefix(r.URL.Path, "/readonly") {
				logger.Info("2. WebSocket upgrade requested for %s, routing to READONLY target", r.URL.Path)
				if rpcProxy.ReadonlyWSSServerURL != "" {
					targetBaseURL, _ := url.Parse(rpcProxy.ReadonlyWSSServerURL)
					targetHost := targetBaseURL.Scheme + "://" + targetBaseURL.Host
					pathForTarget := strings.TrimPrefix(r.URL.Path, "/readonly")
					finalTargetURL = targetHost + pathForTarget
					logger.Debug("Routing WebSocket upgrade for %s to READONLY target %s", r.URL.Path, finalTargetURL)
				} else {
					logger.Error("Readonly WebSocket upgrade requested for %s, but no readonly_wss_server_url configured", r.URL.Path)
					http.Error(w, "Readonly WebSocket endpoint is not configured", http.StatusServiceUnavailable)
					return
				}
			} else {
				logger.Info("2. WebSocket upgrade requested for %s, routing to DEFAULT target", r.URL.Path)
				// Default WebSocket endpoint
				finalTargetURL = rpcProxy.AppCtx.ClientRpc.UrlWS
				logger.Debug("Routing WebSocket upgrade for %s to DEFAULT target %s", r.URL.Path, finalTargetURL)
			}

			if finalTargetURL == "" {
				logger.Error("WebSocket upgrade requested for %s, but target URL is empty", r.URL.Path)
				http.Error(w, "Target WebSocket endpoint is not configured", http.StatusServiceUnavailable)
				return
			}
			logger.Info("3. WebSocket upgrade requested for %s, routing to target %s", r.URL.Path, finalTargetURL)
			rpcProxy.ServeWebSocket(w, r, finalTargetURL)
			return
		}

		// HTTP Readonly endpoint
		if strings.HasPrefix(r.URL.Path, "/readonly") {
			if rpcProxy.ReadonlyReverseProxy != nil {
				logger.Debug("Forwarding HTTP request to READONLY target: %s", r.URL.Path)
				r.URL.Path = strings.TrimPrefix(r.URL.Path, "/readonly")
				if !strings.HasPrefix(r.URL.Path, "/") {
					r.URL.Path = "/" + r.URL.Path
				}
				rpcProxy.ReadonlyReverseProxy.ServeHTTP(w, r)
			} else {
				logger.Error("Received HTTP request for /readonly but readonly proxy is not configured")
				http.Error(w, "Readonly endpoint is not configured", http.StatusNotImplemented)
			}
			return
		}

		// Redirect root GET to BLS registration UI
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			http.Redirect(w, r, webPathPrefix, http.StatusMovedPermanently)
			return
		}

		// Default: Forward to main RPC proxy
		rpcProxy.ServeHTTP(w, r)
	})
	handler := setup.CORSMiddleware(mux)
	handler = setup.LoggingMiddleware(handler)

	return &http.Server{
		Addr:              cfg.ServerPort,
		Handler:           handler,
		ReadTimeout:       300 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
		ReadHeaderTimeout: 10 * time.Second,
	}
}
