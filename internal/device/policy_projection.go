package device

import (
	"strings"

	"github.com/openvohive/openvohive/internal/backend"
	"github.com/openvohive/openvohive/internal/cardpolicy"
	"github.com/openvohive/openvohive/pkg/logger"
)

// applyPolicyToWorker 把卡策略投影进 worker.Config 的运行时有效字段。
func applyPolicyToWorker(w *Worker, p cardpolicy.Policy) {
	if w == nil {
		return
	}
	w.Config.NetworkEnabled = p.NetworkEnabled
	w.Config.AirplaneEnabled = p.AirplaneEnabled
	w.Config.IPVersion = strings.TrimSpace(p.IPVersion)
	if w.Config.IPVersion == "" {
		w.Config.IPVersion = "v4"
	}
	w.Config.APN = strings.TrimSpace(p.APN)
	w.Config.SMSEnabled = true // SMS 恒开
}

type policyApplyResult struct {
	Applied bool
	ICCID   string
	Reason  string
}

// resolveAndApplyPolicy 解析 worker 当前 ICCID 的策略，投影并复用现有 apply 路径。
func (p *Pool) resolveAndApplyPolicy(worker *Worker, reason string) policyApplyResult {
	if p == nil || worker == nil || p.policyResolver == nil {
		return policyApplyResult{}
	}
	iccid := worker.CurrentICCID()
	if iccid == "" {
		logger.Info("跳过策略投影：ICCID 未就绪", "device", worker.ID, "reason", reason)
		return policyApplyResult{Reason: "iccid_empty"}
	}
	pol, err := p.policyResolver.Resolve(iccid)
	if err != nil {
		logger.Warn("解析卡策略失败", "device", worker.ID, "iccid", iccid, "err", err)
		return policyApplyResult{ICCID: iccid, Reason: "resolve_failed"}
	}
	applyPolicyToWorker(worker, pol)
	logger.Info("已投影卡策略", "device", worker.ID, "iccid", iccid,
		"network", pol.NetworkEnabled, "airplane", worker.Config.AirplaneEnabled, "reason", reason)

	switch {
	case pol.AirplaneEnabled:
		p.enterAirplaneModeFromPolicy(worker, reason)
	default:
		p.exitAirplaneModeIfNeeded(worker, reason)
		if err := p.applyNetworkPreference(worker); err != nil {
			logger.Warn("应用网络偏好失败", "device", worker.ID, "err", err)
		}
	}
	return policyApplyResult{Applied: true, ICCID: iccid, Reason: reason}
}

// enterAirplaneModeFromPolicy 按策略进入纯飞行：先断数据网，再把射频切到 RFOff。
func (p *Pool) enterAirplaneModeFromPolicy(w *Worker, reason string) {
	if w == nil {
		return
	}
	if nc := w.NetworkController(); nc != nil && nc.IsConnected() {
		_ = w.StopNetwork()
	}
	w.clearCachedIP()
	ctrl, ok := w.Backend.(backend.OperatingModeController)
	if !ok {
		logger.Warn("设备不支持射频控制，无法投影飞行模式", "device", w.ID, "reason", reason)
		return
	}
	if cur, err := ctrl.GetOperatingMode(p.ctx); err == nil && isFlightOperatingMode(cur) {
		return
	}
	if err := ctrl.SetOperatingMode(p.ctx, backend.ModeRFOff); err != nil {
		logger.Warn("投影飞行模式失败", "device", w.ID, "reason", reason, "err", err)
		return
	}
	logger.Info("已按策略进入飞行模式", "device", w.ID, "reason", reason)
}

// exitAirplaneModeIfNeeded 当设备当前处于飞行(RFOff/LowPower)且策略不要求飞行时，切回 Online。
func (p *Pool) exitAirplaneModeIfNeeded(w *Worker, reason string) {
	if w == nil {
		return
	}
	ctrl, ok := w.Backend.(backend.OperatingModeController)
	if !ok {
		return
	}
	cur, err := ctrl.GetOperatingMode(p.ctx)
	if err != nil || !isFlightOperatingMode(cur) {
		return
	}
	if err := ctrl.SetOperatingMode(p.ctx, backend.ModeOnline); err != nil {
		logger.Warn("退出飞行模式失败", "device", w.ID, "reason", reason, "err", err)
		return
	}
	logger.Info("已按策略退出飞行模式", "device", w.ID, "reason", reason)
}

// CurrentICCIDForDevice 返回指定设备当前 worker 的 ICCID（无 worker 或未就绪返回空串）。
func (p *Pool) CurrentICCIDForDevice(deviceID string) string {
	if p == nil {
		return ""
	}
	w := p.GetWorker(deviceID)
	if w == nil {
		return ""
	}
	return w.CurrentICCID()
}

// SetPolicyResolver 注入卡策略解析器。
func (p *Pool) SetPolicyResolver(r cardpolicy.Resolver) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.policyResolver = r
	p.mu.Unlock()
}
