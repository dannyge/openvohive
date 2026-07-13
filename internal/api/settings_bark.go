package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openvohive/openvohive/internal/config"
	"github.com/openvohive/openvohive/internal/notify"
)

type testBarkRequest struct {
	Enabled   bool   `json:"enabled"`
	ServerURL string `json:"server_url"`
	DeviceKey string `json:"device_key"`
	Title     string `json:"title"`
	Group     string `json:"group"`
	Sound     string `json:"sound"`
	TimeoutMs int    `json:"timeout_ms"`
}

func (s *Server) handleTestBarkNotification(c *gin.Context) {
	var req testBarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
		return
	}

	if !req.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请先启用 Bark 后再测试"})
		return
	}

	// server_url 为空时用官方默认
	serverURL := strings.TrimSpace(req.ServerURL)
	if serverURL == "" {
		serverURL = "https://api.day.app"
	}

	// device_key 为空是参数错误（400），而非服务端故障（500）
	deviceKey := strings.TrimSpace(req.DeviceKey)
	if deviceKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请填写 Bark Device Key 后再测试"})
		return
	}

	ch, err := notify.NewBarkChannel(config.BarkConfig{
		Enabled:   true,
		ServerURL: serverURL,
		DeviceKey: deviceKey,
		Title:     strings.TrimSpace(req.Title),
		Group:     strings.TrimSpace(req.Group),
		Sound:     strings.TrimSpace(req.Sound),
		TimeoutMs: req.TimeoutMs,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "初始化 Bark 测试发送器失败: " + err.Error()})
		return
	}

	now := time.Now()
	ctx := notify.NotificationContext{
		Event:      "测试",
		Text:       "这是一条来自 Vohive 的 Bark 测试通知，收到说明您的 Bark 配置正确！",
		DeviceID:   "test_device_001",
		DeviceName: "测试设备",
		Timestamp:  now,
	}

	sendErr := ch.SendWithContext(ctx)
	if sendErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": "测试 Bark 推送失败: " + sendErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "测试 Bark 推送已发送",
	})
}
