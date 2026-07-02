package logger

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)


func envEnabled(name string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// ShouldLogSMSContent 返回是否允许输出短信明文。
func ShouldLogSMSContent() bool {
	return envEnabled("VOHIVE_SMS_LOG_CONTENT")
}

// RedactSMSContent 统一短信内容脱敏；开启 VOHIVE_SMS_LOG_CONTENT 时返回明文。
func RedactSMSContent(content string) string {
	if ShouldLogSMSContent() {
		return content
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "[REDACTED len=0]"
	}
	return fmt.Sprintf("[REDACTED len=%d]", utf8.RuneCountInString(content))
}

