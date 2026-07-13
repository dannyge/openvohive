package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/openvohive/openvohive/internal/config"
	"github.com/openvohive/openvohive/pkg/logger"
)

// barkPushPayload 定义推送给 Bark 服务器的 JSON 结构
// 字段名遵循 Bark API V2 文档（https://github.com/Finb/bark-server/blob/master/docs/API_V2.md）
type barkPushPayload struct {
	DeviceKey string `json:"device_key"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body"`
	Copy      string `json:"copy,omitempty"`
	AutoCopy  string `json:"autoCopy,omitempty"` // 值为 "1" 时自动复制 copy 字段到剪贴板
	Group     string `json:"group,omitempty"`
	Sound     string `json:"sound,omitempty"`
}

// 验证码提取的正则模式（按优先级排序）
// 所有模式都加 (?:\D|$) 结束边界，避免从长数字串中截取前 8 位
var verificationCodePatterns = []*regexp.Regexp{
	// 中文：验证码[是为:]\s*(\d{4,8})
	regexp.MustCompile(`验证码[是为：:]?\s*(\d{4,8})(?:\D|$)`),
	// 中文变体：动态码/校验码/验证码
	regexp.MustCompile(`(?:动态码|校验码|验证码)[：:=\s]*(\d{4,8})(?:\D|$)`),
	// 英文：code[is:=]\s*(\d{4,8})
	regexp.MustCompile(`(?i)code[\s:=]+(\d{4,8})(?:\D|$)`),
	// 英文：OTP
	regexp.MustCompile(`(?i)OTP[\s:=]+(\d{4,8})(?:\D|$)`),
}

// extractVerificationCode 从短信文本中提取验证码
// 返回提取到的验证码，未找到则返回空字符串
// 仅匹配 4-8 位数字（验证码常见长度），不额外过滤年份/手机号
func extractVerificationCode(text string) string {
	for _, p := range verificationCodePatterns {
		matches := p.FindStringSubmatch(text)
		if len(matches) >= 2 {
			code := matches[1]
			// 仅接受 4-8 位数字（验证码常见长度范围）
			if len(code) >= 4 && len(code) <= 8 {
				return code
			}
		}
	}
	return ""
}

// extractSMSContent 从通知文本中提取纯短信内容
// 通知格式: "收到新短信 / 蜂窝\n设备  quectel-1\n号码  10086\n时间  ...\n内容  短信正文"
// 短信正文可能跨多行，匹配到"内容"行后拼接其后的所有行
func extractSMSContent(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "内容") {
			// 去掉 "内容" 前缀和空白
			content := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "内容"))
			// 拼接后续行（多行短信正文）
			if i+1 < len(lines) {
				remaining := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
				if remaining != "" {
					content = content + "\n" + remaining
				}
			}
			return content
		}
	}
	return text
}

// BarkChannel 实现 Channel 接口的 Bark 推送通知渠道
// Bark 是 iOS 推送工具，通过 HTTP API 将通知推送到 iPhone
type BarkChannel struct {
	serverURL string
	deviceKey string
	title     string
	group     string
	sound     string
	client    *http.Client
}

// NewBarkChannel 根据配置创建 Bark 渠道
func NewBarkChannel(cfg config.BarkConfig) (*BarkChannel, error) {
	if cfg.DeviceKey == "" {
		return nil, errors.New("bark device_key 未配置")
	}

	serverURL := strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	if serverURL == "" {
		serverURL = "https://api.day.app"
	}

	title := strings.TrimSpace(cfg.Title)
	if title == "" {
		title = "openvohive"
	}

	timeoutMs := cfg.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	return &BarkChannel{
		serverURL: serverURL,
		deviceKey: cfg.DeviceKey,
		title:     title,
		group:     strings.TrimSpace(cfg.Group),
		sound:     strings.TrimSpace(cfg.Sound),
		client: &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		},
	}, nil
}

func (c *BarkChannel) Name() string {
	return "bark"
}

func (c *BarkChannel) Send(text string) error {
	return c.SendWithContext(NotificationContext{Event: "通知", Text: text})
}

// SendWithContext 构建 Bark 推送并发送
// 自动提取验证码设置 copy 字段，实现收到推送后自动复制验证码
func (c *BarkChannel) SendWithContext(ctx NotificationContext) error {
	smsContent := extractSMSContent(ctx.Text)

	// 构建标题：含设备名
	title := c.title
	if label := ctx.DeviceLabel(); label != "" && label != "未知设备" {
		title = fmt.Sprintf("%s %s", c.title, label)
	}

	// 从纯短信内容中提取验证码（不从完整通知文本提取，避免匹配到时间/号码等干扰数字）
	code := extractVerificationCode(smsContent)

	// 构建 copy 和 autoCopy：仅在提取到验证码时才设 autoCopy（避免覆盖用户剪贴板）
	copyContent := ""
	autoCopy := ""
	if code != "" {
		copyContent = code
		autoCopy = "1"
	}

	payload := barkPushPayload{
		DeviceKey: c.deviceKey,
		Title:     title,
		Body:      ctx.Text,
		Copy:      copyContent,
		AutoCopy:  autoCopy,
		Group:     c.group,
		Sound:     c.sound,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bark payload 序列化失败: %w", err)
	}

	url := fmt.Sprintf("%s/push", c.serverURL)

	// 重试策略说明：
	//
	// bark-server 实际只返回三种状态码（源码 route_push.go + apns/apns.go）：
	//   200 — 推送成功（route_push.go:275, apns.go:149）
	//   400 — 客户端错误：device_key 为空、数据库找不到 token、请求体解析失败等
	//         （route_push.go:251,261；均为确定性错误，重发结果不变）
	//   500 — APNs 网络错误或 APNs 返回非 200（apns.go:144, route_push.go:273）
	//
	// 注意：APNs 自身返回的 400/403/410/429 等错误码被 route_push.go:273 统一包装成 500
	// 返回给客户端，所以客户端收到的 500 是一个混合体——既可能是临时故障，也可能
	// 是不可恢复的 token 失效。但考虑到：
	//   1. 推送没有副作用（失败就是失败，再试不会更糟）
	//   2. 漏掉一条短信验证码通知的代价远大于多发一次请求
	// 所以 500 选择重试一次。
	//
	// 另外，bark-server 无限流中间件，自身不产生 429，也不返回 Retry-After header。
	// 但自建节点前面可能套了反向代理（nginx 等），反代可能返回 429/502/503，
	// 这些也应重试。
	//
	// 源码参考：https://github.com/Finb/bark-server/blob/master/route_push.go
	//          https://github.com/Finb/bark-server/blob/master/apns/apns.go

	const maxAttempts = 2
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("bark 请求创建失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")

		resp, err := c.client.Do(req)
		if err != nil {
			// 网络错误（超时、连接失败）→ 可重试
			lastErr = err
			if attempt+1 < maxAttempts {
				logger.Warn("Bark 推送失败，重试中", "attempt", attempt+1, "err", err)
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// 2xx → 成功
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if code != "" {
				logger.Info("Bark 推送成功（已提取验证码）", "title", title)
			} else {
				logger.Info("Bark 推送成功", "title", title)
			}
			return nil
		}

		lastErr = fmt.Errorf("bark 返回状态码 %d: %s", resp.StatusCode, string(respBody))

		// 正向判断：只有可重试的状态码才继续，否则直接返回
		if !isRetryableBarkStatus(resp.StatusCode) {
			return lastErr
		}

		if attempt+1 < maxAttempts {
			logger.Warn("Bark 推送返回可重试状态码，重试中", "status", resp.StatusCode, "body", string(respBody))
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	return lastErr
}

// isRetryableBarkStatus 判断 bark 返回的 HTTP 状态码是否值得重试
//   5xx → 是（可能是 APNs 临时故障、反代 502/503）
//   429 → 是（限流，bark 自身不产生，反向代理可能返回）
//   其它 4xx → 否（device_key 错误、参数错误等，重发结果不变）
func isRetryableBarkStatus(code int) bool {
	return code >= 500 || code == 429
}

func (c *BarkChannel) RegisterCommand(cmd string, handler CommandHandler) {
	// Bark 是纯推送渠道，不支持接收指令
}

func (c *BarkChannel) Start() error {
	return nil
}

func (c *BarkChannel) Close() error {
	if c.client != nil {
		c.client.CloseIdleConnections()
	}
	return nil
}
