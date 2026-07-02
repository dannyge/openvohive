package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const (
	ESIMTransportAT            = "at"
	ESIMTransportQMI           = "qmi"
	ESIMTransportMBIM          = "mbim"
	MBIMTransportAuto          = "auto"
	MBIMTransportProxy         = "proxy"
	MBIMTransportDirect        = "direct"
	DefaultWebhookTextTemplate = "{{device_label}} {{text}}"
)

func NormalizeESIMTransport(in string) string {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "", ESIMTransportAT:
		return ESIMTransportAT
	case ESIMTransportQMI:
		return ESIMTransportQMI
	case ESIMTransportMBIM:
		return ESIMTransportMBIM
	default:
		return strings.ToLower(strings.TrimSpace(in))
	}
}

func ValidateESIMTransport(in string) error {
	switch NormalizeESIMTransport(in) {
	case ESIMTransportAT, ESIMTransportQMI, ESIMTransportMBIM:
		return nil
	default:
		return fmt.Errorf("invalid esim transport: %q", strings.TrimSpace(in))
	}
}

func NormalizeMBIMTransport(in string) string {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "", MBIMTransportAuto:
		return MBIMTransportAuto
	case MBIMTransportProxy:
		return MBIMTransportProxy
	case MBIMTransportDirect:
		return MBIMTransportDirect
	default:
		return MBIMTransportAuto
	}
}

// ResolveIPFamily parses DeviceConfig.IPVersion into IPv4/IPv6 enable flags.
// Empty input preserves the legacy IPv4-only behavior.
func ResolveIPFamily(in string) (enableV4 bool, enableV6 bool, err error) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "", "v4", "ipv4":
		return true, false, nil
	case "v6", "ipv6":
		return false, true, nil
	case "v4v6", "v6v4", "dual", "ipv4v6":
		return true, true, nil
	default:
		return false, false, fmt.Errorf("无效的 ip_version: %q (允许 v4|v6|v4v6)", in)
	}
}

type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Devices      []DeviceConfig     `mapstructure:"devices"`
	Telegram     TelegramConfig     `mapstructure:"telegram"`
	Webhook      WebhookConfig      `mapstructure:"webhook"`
	Email        EmailConfig        `mapstructure:"email"`
	Web          WebConfig          `mapstructure:"web"`
}


type WebConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type ServerConfig struct {
	Port  string `mapstructure:"port"`
	Debug bool   `mapstructure:"debug"`
}

type ESIMSwitchConfig struct {
	// UseRefreshTrue uses refresh=true for the main switch path. Default false preserves current behavior.
	UseRefreshTrue bool `mapstructure:"use_refresh_true"`
	// EventGatedConverge uses UIM indication events to gate post-switch convergence. Default false.
	EventGatedConverge bool `mapstructure:"event_gated_converge"`
	// RadioCycle performs LowPower -> Online radio cycling around switch. Default false.
	RadioCycle bool `mapstructure:"radio_cycle"`
	// ReinitWindowMS is the expected UIM reinitialization window in milliseconds. Default 0 disables the window.
	// Only effective when EventGatedConverge=true; ReinitWindow marks the period during which GetUIMReadiness
	// timeouts do not trigger whole-core recovery (to avoid triggering on firmware reinitialization stalls).
	// If EventGatedConverge=false, ReinitWindowMS is silently ignored.
	ReinitWindowMS int `mapstructure:"reinit_window_ms"`
	// NASAttachTimeoutMS bounds optional attach waiting after Online in milliseconds. Default 0 means do not block.
	NASAttachTimeoutMS int `mapstructure:"nas_attach_timeout_ms"`
}

type DeviceConfig struct {
	ID            string `mapstructure:"id"`
	Name          string `mapstructure:"name"` // 设备显示名称
	ModemIMEI     string `mapstructure:"modem_imei"`
	USBPath       string `mapstructure:"-"`              // Deprecated: 运行时按 IMEI 现解析,绝不从文件读取
	ATPort        string `mapstructure:"-"`              // Deprecated: 运行时解析;AT 终端用 Worker.ResolvedATPort()
	ManagePort    string `mapstructure:"-"`              // Deprecated: 运行时解析,绝不从文件读取
	Interface     string `mapstructure:"-"`              // Deprecated: 运行时解析,绝不从文件读取
	QMIDevice     string `mapstructure:"-"`              // Deprecated: 运行时解析,绝不从文件读取
	ControlDevice string `mapstructure:"-"`              // Deprecated: 运行时按 IMEI 现解析,绝不从文件读取
	MBIMTransport string `mapstructure:"mbim_transport"` // MBIM 传输: auto|proxy|direct，默认 auto
	QMIUseProxy   bool   `mapstructure:"qmi_use_proxy"`  // 是否通过 libqmi qmi-proxy 打开 QMI 控制口
	// 可选：qmi-proxy abstract socket 名称和可执行文件路径。留空使用 quectel-qmi-go 默认值。
	QMIProxyPath       string `mapstructure:"qmi_proxy_path"`
	QMIProxyExecutable string `mapstructure:"qmi_proxy_executable"`
	ESIMTransport      string `mapstructure:"esim_transport"` // eSIM 传输通道: at|qmi|mbim，默认 at
	DeviceBackend      string `mapstructure:"device_backend"` // 设备后端模式: at|qmi|mbim|auto，默认 at
	USBNetMode         *int   `mapstructure:"usbnet_mode"`    // 可选：用于校验/设置 Quectel USBNET 模式
	// ESIMSwitch controls deterministic eSIM switch behavior. Zero values preserve current behavior.
	ESIMSwitch ESIMSwitchConfig `mapstructure:"esim_switch"`

	OperatorSelectionMode string `mapstructure:"operator_selection_mode"`
	OperatorSelectionPLMN string `mapstructure:"operator_selection_plmn"`
	OperatorSelectionRAT  string `mapstructure:"operator_selection_rat"`

	// Serial config
	BaudRate int    `mapstructure:"baud_rate"`
	DataBits int    `mapstructure:"data_bits"`
	StopBits int    `mapstructure:"stop_bits"`
	Parity   string `mapstructure:"parity"`

	// 以下为运行时有效策略（投影自 card_policies，按 ICCID），不再从配置文件加载
	APN             string `mapstructure:"-"`
	NetworkEnabled  bool   `mapstructure:"-"`
	IPVersion       string `mapstructure:"-"`
	AirplaneEnabled bool   `mapstructure:"-"`
	SMSEnabled      bool   `mapstructure:"-"` // SMS 恒开，运行时强制 true

	// USB Audio (自动发现，无需手动配置)
	AudioDevice string `mapstructure:"-"` // Deprecated: 运行时解析,绝不从文件读取
}

type TelegramConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
	ChatID   int64  `mapstructure:"chat_id"`
	AdminID  int64  `mapstructure:"admin_id"`
	BaseURL  string `mapstructure:"base_url"` // 反向代理地址 (例如 https://api.telegram.org/bot%s/%s)
	Proxy    string `mapstructure:"proxy"`    // HTTP 代理地址 (例如 http://127.0.0.1:7890)
}

type WebhookConfig struct {
	Enabled      bool              `mapstructure:"enabled"`
	URLs         []string          `mapstructure:"urls"`
	Secret       string            `mapstructure:"secret"`
	TimeoutMs    int               `mapstructure:"timeout_ms"`
	RetryMax     int               `mapstructure:"retry_max"`
	TextTemplate string            `mapstructure:"text_template"`
	Headers      map[string]string `mapstructure:"headers,omitempty" json:"headers,omitempty"`
}

type EmailConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	UseSSL      bool     `mapstructure:"use_ssl"`
	SMTPHost    string   `mapstructure:"smtp_host"`
	SMTPPort    int      `mapstructure:"smtp_port"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
	FromAddress string   `mapstructure:"from_address"`
	ToAddresses []string `mapstructure:"to_addresses"`
}

func Load(path string) (*Config, error) {
	if err := migrateLegacyManagedNetworkField(path); err != nil {
		return nil, err
	}
	if err := migrateDeprecatedRuntimePathFields(path); err != nil {
		return nil, err
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// 默认值设置
	viper.SetDefault("server.port", 7575)
	viper.SetDefault("webhook.timeout_ms", 5000)
	viper.SetDefault("webhook.retry_max", 3)
	viper.SetDefault("webhook.text_template", DefaultWebhookTextTemplate)

	viper.SetDefault("email.enabled", false)
	viper.SetDefault("email.use_ssl", false)
	viper.SetDefault("web.username", "admin")
	viper.SetDefault("web.password", "admin")

	// 官方默认推送秘钥与用户 (留空则不执行 Push)

	// 环境变量覆盖支持 (例如 PROXY_DEVICES_0_APN)
	viper.SetEnvPrefix("PROXY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 兼容 server.port 格式 (例如: 7575 和 :7575)
	if cfg.Server.Port != "" && !strings.Contains(cfg.Server.Port, ":") {
		cfg.Server.Port = ":" + cfg.Server.Port
	}

	return &cfg, nil
}
