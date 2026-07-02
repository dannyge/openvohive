package device

import (
	"github.com/openvohive/openvohive/internal/config"
)

// 配额限制已移除，保留空壳函数防止编译错误。
const DefaultFreeDeviceLimit = 0

func FreeDeviceLimitReached(count int) bool { return false }

func FreeDeviceAddLimitMessage() string { return "" }

func FreeDeviceWorkerLimitMessage() string { return "" }

func FreeDeviceLimitAllowsConfiguredDevice(_ []config.DeviceConfig, _ string) bool { return true }
