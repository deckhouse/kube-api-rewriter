package app

import (
	log "log/slog"
	"net/http"
	"os"

	logutil "github.com/deckhouse/kube-api-rewriter/pkg/log"
	"github.com/deckhouse/kube-api-rewriter/pkg/middleware/auth"
	"github.com/deckhouse/kube-api-rewriter/pkg/monitoring/healthz"
	"github.com/deckhouse/kube-api-rewriter/pkg/monitoring/metrics"
	"github.com/deckhouse/kube-api-rewriter/pkg/monitoring/profiler"
	"github.com/deckhouse/kube-api-rewriter/pkg/proxy"
	"github.com/deckhouse/kube-api-rewriter/pkg/rewriter"
	"github.com/deckhouse/kube-api-rewriter/pkg/server"
	"github.com/deckhouse/kube-api-rewriter/pkg/target"
)

// StartFromEnv starts application using settings from environment variables.
func StartFromEnv(rewriteRules *rewriter.RewriteRules) {
	settings, err := SettingsFromEnv()
	if err != nil {
		log.Error("failed to get settings from env", logutil.SlogErr(err))
		os.Exit(1)
	}
	Start(settings, rewriteRules)
}

func Start(settings *AppSettings, rewriteRules *rewriter.RewriteRules) {
	SetupLogging(settings)

	// Init rewriter rules.
	if rewriteRules == nil {
		log.Error("RewriteRules are required")
		os.Exit(1)
	}
	rewriteRules.Init()

	RegisterAllMetrics()

	config, err := target.NewKubernetesTarget()
	if err != nil {
		log.Error("Load Kubernetes REST", logutil.SlogErr(err))
		os.Exit(1)
	}

	httpServers := HTTPServers{
		Monitoring:   CreateMonitoringServer(settings, config),
		Pprof:        CreatePprofServer(settings),
		ClientProxy:  CreateClientProxy(settings, rewriteRules, config),
		WebhookProxy: CreateWebhookProxy(settings, rewriteRules),
	}

	exitCode := RunServers(httpServers)
	os.Exit(exitCode)
}

func SetupLogging(settings *AppSettings) {
	// Set options for the default logger: level, format and output.
	logutil.SetupDefaultLoggerFromEnv(logutil.Options{
		Level:  settings.LogLevel,
		Format: settings.LogFormat,
		Output: settings.LogOutput,
	})
}

// RegisterAllMetrics init prometheus client and register common and application specific metrics.
func RegisterAllMetrics() {
	metrics.Init()
	proxy.RegisterMetrics()
}

// CreateMonitoringServer returns a monitoring server with metrics and healthz probes.
func CreateMonitoringServer(settings *AppSettings, config *target.Kubernetes) *server.HTTPServer {
	lAddr := settings.MonitoringBindAddress
	if lAddr == "" {
		lAddr = DefaultMonitoringBindAddress
	}

	monMux := http.NewServeMux()
	healthz.AddHealthzHandler(monMux)

	metricsHandler := metrics.NewHandler()
	if settings.MonitoringAuth != nil {
		metricsHandler = auth.NewMiddlewareFromKubeClient(config.KubeClient, *settings.MonitoringAuth).Handler(metricsHandler)
	}
	metrics.AddMetricsHandler(monMux, metricsHandler)

	return &server.HTTPServer{
		InstanceDesc: "Monitoring handlers",
		ListenAddr:   lAddr,
		RootHandler:  monMux,
		CertManager:  nil,
		Err:          nil,
	}
}

// CreatePprofServer returns a pprof server if bind address is specified.
func CreatePprofServer(settings *AppSettings) *server.HTTPServer {
	if settings.PprofBindAddress == "" {
		return nil
	}

	pprofHandler := profiler.NewPprofHandler()

	return &server.HTTPServer{
		InstanceDesc: "Pprof",
		ListenAddr:   settings.PprofBindAddress,
		RootHandler:  pprofHandler,
	}
}

// CreateClientProxy returns a rewriter proxy that listens for requests from the controller and sends them to the Kubernetes API server.
func CreateClientProxy(settings *AppSettings, rewriteRules *rewriter.RewriteRules, config *target.Kubernetes) *server.HTTPServer {
	if settings.ClientProxy == "no" {
		log.Info("Configured to not start client rewriter proxy")
		return nil
	}

	lAddr := server.ConstructListenAddr(
		settings.ClientProxyAddress,
		settings.ClientProxyPort,
		loopbackAddr,
		defaultAPIClientProxyPort,
	)
	rwr := &rewriter.RuleBasedRewriter{
		Rules: rewriteRules,
	}
	proxyHandler := &proxy.Handler{
		Name:         "kube-api",
		TargetClient: config.Client,
		TargetURL:    config.APIServerURL,
		ProxyMode:    proxy.ToRenamed,
		Rewriter:     rwr,
	}
	proxyHandler.Init()
	return &server.HTTPServer{
		InstanceDesc: "API Client proxy",
		ListenAddr:   lAddr,
		RootHandler:  proxyHandler,
	}

}

// CreateWebhookProxy returns a rewriter proxy that listens for requests from the Kubernetes API server and sends them to the local webhook server.
func CreateWebhookProxy(settings *AppSettings, rewriteRules *rewriter.RewriteRules) *server.HTTPServer {
	if settings.WebhookTargetSettings.Address == "" {
		log.Info("Configured to not start webhook rewriter proxy: no address for the target webhook server")
		return nil
	}

	config, err := target.NewWebhookTarget(settings.WebhookTargetSettings)
	if err != nil {
		log.Error("Configure webhook client", logutil.SlogErr(err))
		os.Exit(1)
	}
	lAddr := server.ConstructListenAddr(
		settings.WebhookProxyAddress,
		settings.WebhookProxyPort,
		anyAddr,
		defaultWebhookProxyPort,
	)
	rwr := &rewriter.RuleBasedRewriter{
		Rules: rewriteRules,
	}
	proxyHandler := &proxy.Handler{
		Name:         "webhook",
		TargetClient: config.Client,
		TargetURL:    config.URL,
		ProxyMode:    proxy.ToOriginal,
		Rewriter:     rwr,
	}
	proxyHandler.Init()
	return &server.HTTPServer{
		InstanceDesc: "Webhook proxy",
		ListenAddr:   lAddr,
		RootHandler:  proxyHandler,
		CertManager:  config.CertManager,
	}
}

type HTTPServers struct {
	Monitoring   *server.HTTPServer
	Pprof        *server.HTTPServer
	ClientProxy  *server.HTTPServer
	WebhookProxy *server.HTTPServer
}

func RunServers(httpServers HTTPServers) int {
	if httpServers.ClientProxy == nil && httpServers.WebhookProxy == nil {
		log.Info("No rewriter proxies enabled, exit. Check CLIENT_PROXY, CLIENT_ADDRESS, WEBHOOK_PROXY, and WEBHOOK_ADDRESS environment variables.")
		return 0
	}

	servers := []*server.HTTPServer{
		httpServers.ClientProxy,
		httpServers.WebhookProxy,
		httpServers.Monitoring,
		httpServers.Pprof,
	}
	// Start all registered servers and block the main process until at least one server stops.
	group := server.NewRunnableGroup()
	for i := range servers {
		if servers[i] == nil {
			continue
		}
		group.Add(servers[i])
	}
	// Block while servers are running.
	group.Start()

	// Log errors for each instance and exit.
	exitCode := 0
	for _, srv := range servers {
		if srv.Err != nil {
			log.Error(srv.InstanceDesc, logutil.SlogErr(srv.Err))
			exitCode = 1
		}
	}
	return exitCode
}
