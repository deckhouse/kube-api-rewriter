package app

import (
	"os"

	"github.com/deckhouse/kube-api-rewriter/pkg/target"
	"github.com/deckhouse/kube-api-rewriter/pkg/middleware/auth"
	"encoding/json"
	"fmt"
)

// AppSettings configures kube-api-rewriter application.
type AppSettings struct {
	// ClientProxy is a flag that may disable client proxy. Client proxy can be disabled at runtime with CLIENT_PROXY=no environment variable.
	ClientProxy string
	// ClientProxyAddress is an address to listen for incoming client requests. Default is 127.0.0.1. Can be set at runtime with CLIENT_PROXY_ADDRESS environment variable.
	ClientProxyAddress string
	// ClientProxyPort is a port to listen for incoming client requests. Default is 23915. Can be set at runtime with CLIENT_PROXY_ADDRESS environment variable.
	ClientProxyPort string

	// WebhookProxyAddress is an address to listen for incoming webhook requests from Kubernetes API server. Default is 0.0.0.0. Can be set at runtime with WEBHOOK_PROXY_ADDRESS environment variable.
	WebhookProxyAddress string
	// WebhookProxyPort is a port to listen for incoming client requests from Kubernetes API server. Default is 24192. Can be set at runtime with WEBHOOK_PROXY_ADDRESS environment variable.
	WebhookProxyPort string

	// WebhookTargetSettings holds settings for the target end of the webhook proxy.
	WebhookTargetSettings target.WebhookTargetSettings

	// LogLevel is a level of logging (debug, info, error, warn). Default is info. Can be set at runtime with LOG_LEVEL environment variable.
	LogLevel string
	// LogFormat is a format of logging: json, text, or pretty. Default is json. Can be set at runtime with LOG_FORMAT environment variable.
	LogFormat string
	// LogOutput is a level of logging: stderr, stdout, discard. Default is stdout. Can be set at runtime with LOG_OUTPUT environment variable.
	LogOutput string

	// MonitoringBindAddress is a listen address for the metrics server. Default is :9090. Can be set at runtime with MONITORING_BIND_ADDRESS environment variable.
	MonitoringBindAddress string

	// MonitoringAuth holds settings for the authentication via kubernetes rbac
	MonitoringAuth *auth.ResourceAttributes

	// PprofBindAddress is a listen address for the pprof server. Should be set to enable pprof. Can be set at runtime with PPROF_BIND_ADDRESS environment variable.
	PprofBindAddress string
}

func SettingsFromEnv() (*AppSettings, error) {
	monitoringAuth, err := monitoringAuthFromEnv()
	if err != nil {
		return nil, err
	}
	return &AppSettings{
		ClientProxy:           os.Getenv(ClientProxyEnv),
		ClientProxyAddress:    os.Getenv(ClientProxyAddressEnv),
		ClientProxyPort:       os.Getenv(ClientProxyPortEnv),
		WebhookProxyAddress:   os.Getenv(WebhookListenAddrEnv),
		WebhookProxyPort:      os.Getenv(WebhookListenPortEnv),
		WebhookTargetSettings: target.WebhookTargetSettingsFromEnv(),
		LogLevel:              os.Getenv(LogLevelEnv),
		LogFormat:             os.Getenv(LogFormatEnv),
		LogOutput:             os.Getenv(LogOutputEnv),
		MonitoringBindAddress: os.Getenv(MonitoringBindAddressEnv),
		MonitoringAuth:        monitoringAuth,
		PprofBindAddress:      os.Getenv(PprofBindAddressEnv),
	}, nil
}

func monitoringAuthFromEnv() (*auth.ResourceAttributes, error) {
	if env, ok := os.LookupEnv(MonitoringAuthEnv); ok {
		monitoringAuth := &auth.ResourceAttributes{}
		if err := json.Unmarshal([]byte(env), monitoringAuth); err != nil {
			return nil, fmt.Errorf("failed to parse monitoring auth env: %w", err)
		}
		if err := monitoringAuth.Validate(); err != nil {
			return nil, fmt.Errorf("invalid monitoring auth env: %w", err)
		}
		return monitoringAuth, nil
	}
	return nil, nil
}
