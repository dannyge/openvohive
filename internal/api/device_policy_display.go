package api

import (
	"errors"

	"github.com/openvohive/openvohive/internal/db"
)

// currentEffectiveDevicePolicy 返回设备保存前的旧有效策略（用于开关转换判断）：
// 在线取 worker 投影(已是有效值)，离线退回 card_policies 解析。同时返回解析到的 ICCID。
func (s *Server) currentEffectiveDevicePolicy(deviceID string) (iccid string, network bool, ipVersion, apn string) {
	if s.pool != nil {
		if w := s.pool.GetWorker(deviceID); w != nil {
			return w.CurrentICCID(), w.Config.NetworkEnabled, w.Config.IPVersion, w.Config.APN
		}
	}
	off := resolveOfflineDevicePolicy(deviceID)
	return db.CurrentICCIDForDevice(deviceID), off.NetworkEnabled, off.IPVersion, off.APN
}

// offlineDevicePolicy 是离线设备(无运行中 worker)用于展示的有效卡策略。
type offlineDevicePolicy struct {
	NetworkEnabled bool
	SMSEnabled     bool
	IPVersion      string
	APN            string
}

// resolveOfflineDevicePolicy 解析离线设备的有效策略用于 UI 展示：
// device → 当前 ICCID → card_policies。无 ICCID 或无策略记录时返回安全默认。
// SMS 恒为启用（写死系统语义），与 card_policies 无关。
func resolveOfflineDevicePolicy(deviceID string) offlineDevicePolicy {
	out := offlineDevicePolicy{SMSEnabled: true, IPVersion: "v4"}
	iccid := db.CurrentICCIDForDevice(deviceID)
	if iccid == "" {
		return out
	}
	pol, err := db.GetCardPolicy(iccid)
	if err != nil {
		if errors.Is(err, db.ErrCardPolicyNotFound) {
			return out
		}
		return out
	}
	out.NetworkEnabled = pol.NetworkEnabled
	if pol.IPVersion != "" {
		out.IPVersion = pol.IPVersion
	}
	out.APN = pol.APN
	return out
}
