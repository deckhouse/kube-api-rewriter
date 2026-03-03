package app

const (
	loopbackAddr              = "127.0.0.1"
	anyAddr                   = "0.0.0.0"
	defaultAPIClientProxyPort = "23915"
	defaultWebhookProxyPort   = "24192"
)

const (
	ClientProxyEnv        = "CLIENT_PROXY"
	ClientProxyAddressEnv = "CLIENT_PROXY_ADDRESS"
	ClientProxyPortEnv    = "CLIENT_PROXY_PORT"
	WebhookListenAddrEnv  = "WEBHOOK_LISTEN_ADDRESS"
	WebhookListenPortEnv  = "WEBHOOK_LISTEN_PORT"
)

const (
	LogLevelEnv  = "LOG_LEVEL"
	LogFormatEnv = "LOG_FORMAT"
	LogOutputEnv = "LOG_OUTPUT"
)

const (
	MonitoringBindAddressEnv     = "MONITORING_BIND_ADDRESS"
	DefaultMonitoringBindAddress = ":9090"
	PprofBindAddressEnv          = "PPROF_BIND_ADDRESS"
)
