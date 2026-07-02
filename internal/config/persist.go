package config

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "go.yaml.in/yaml/v3"
)

func UpdateNotificationInFile(path string, telegram TelegramConfig, webhook WebhookConfig, email EmailConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root["telegram"] = map[string]any{
		"enabled":   telegram.Enabled,
		"bot_token": telegram.BotToken,
		"chat_id":   telegram.ChatID,
		"admin_id":  telegram.AdminID,
		"base_url":  telegram.BaseURL,
		"proxy":     telegram.Proxy,
	}

	root["webhook"] = map[string]any{
		"enabled":       webhook.Enabled,
		"urls":          webhook.URLs,
		"secret":        webhook.Secret,
		"timeout_ms":    webhook.TimeoutMs,
		"retry_max":     webhook.RetryMax,
		"text_template": webhook.TextTemplate,
		"headers":       webhook.Headers,
	}

	root["email"] = map[string]any{
		"enabled":      email.Enabled,
		"smtp_host":    email.SMTPHost,
		"smtp_port":    email.SMTPPort,
		"username":     email.Username,
		"password":     email.Password,
		"from_address": email.FromAddress,
		"to_addresses": email.ToAddresses,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("写入临时配置文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}

// UpdateWebCredentialsInFile 更新配置文件中的 Web 凭证（用户名和密码）
func UpdateWebCredentialsInFile(path string, username, password string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 更新 web 节点
	root["web"] = map[string]any{
		"username": username,
		"password": password,
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("写入临时配置文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}
