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
var verificationCodePatterns = []*regexp.Regexp{
	// 中文：验证码[是为:]\s*(\d{4,8})
	regexp.MustCompile(`验证码[是为：:]?\s*(\d{4,8})`),
	// 英文：code[is:=]\s*(\d{4,8})（不区分大小写在编译时已设）
	regexp.MustCompile(`(?i)code[\s:=]+(\d{4,8})`),
	// 独立的 4-8 位数字（前后不是数字）
	regexp.MustCompile(`(?:^|[^\d])(\d{4,8})(?:[^\d]|$)`),
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
func extractSMSContent(text string) string {
	// 尝试提取"内容"字段
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "内容") {
			// 去掉 "内容  " 前缀
			content := strings.TrimPrefix(strings.TrimSpace(line), "内容")
			content = strings.TrimSpace(strings.TrimPrefix(content, " "))
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

	// 构建 copy 内容：优先验证码，其次短信正文
	copyContent := code
	if copyContent == "" {
		copyContent = smsContent
	}

	payload := barkPushPayload{
		DeviceKey: c.deviceKey,
		Title:     title,
		Body:      ctx.Text,
		Copy:      copyContent,
		AutoCopy:  "1",
		Group:     c.group,
		Sound:     c.sound,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bark payload 序列化失败: %w", err)
	}

	url := fmt.Sprintf("%s/push", c.serverURL)

	// 最多尝试 2 次（1 次原始请求 + 1 次重试），每次重新构建 request
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("bark 请求创建失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			logger.Warn("Bark 推送失败，重试中", "attempt", attempt+1, "err", err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if code != "" {
				logger.Info("Bark 推送成功（已提取验证码）", "title", title)
			} else {
				logger.Info("Bark 推送成功", "title", title)
			}
			return nil
		}

		lastErr = fmt.Errorf("bark 返回状态码 %d: %s", resp.StatusCode, string(respBody))
		logger.Warn("Bark 推送返回非 2xx", "status", resp.StatusCode, "body", string(respBody))
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	return lastErr
}

func (c *BarkChannel) RegisterCommand(cmd string, handler CommandHandler) {
	// Bark 是纯推送渠道，不支持接收指令
}

func (c *BarkChannel) Start() error {
	return nil
}

func (c *BarkChannel) Close() error {
	return nil
}
