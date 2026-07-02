package device

// SMSOutcome DTO。
type SMSOutcome struct {
	MessageID     string
	PartsTotal    int
	DeliveryState string
}

// DeliveryStatus DTO。
type DeliveryStatus struct {
	MessageID string
	Status    string
}

// waitWorkerReady waits for the worker to become ready.
func (p *Pool) waitWorkerReady(deviceID string, timeout any) error { return nil }

// WaitQMICoreReady stub (maintained for compilation, always succeeds).
func (p *Pool) WaitQMICoreReady(deviceID string, timeout any) error { return nil }

// WaitQMIControlReady stub (maintained for compilation, always succeeds).
func (p *Pool) WaitQMIControlReady(deviceID string, timeout any) error { return nil }

// broadcastVoWiFiStateChange stub (no-op after VoWiFi removal).
func (p *Pool) broadcastVoWiFiStateChange(deviceID string) {}

// voWiFiHost stub - returns nil.
func (p *Pool) voWiFiHost() *vowifihostManagerStub { return nil }

// IsVoWiFiActive stub - always returns false.
func (p *Pool) IsVoWiFiActive(deviceID string) bool { return false }

// vowifihostManagerStub is a placeholder for the removed VoWiFi host manager.
type vowifihostManagerStub struct{}

func (s *vowifihostManagerStub) Instance(deviceID string) *vowifiAppServiceStub    { return nil }
func (s *vowifihostManagerStub) SwitchBegin(ctx any, deviceID string) error        { return nil }
func (s *vowifihostManagerStub) SwitchEnd(ctx any, deviceID string, ok bool) error { return nil }

// vowifiAppServiceStub is a placeholder for VoWiFi app service.
type vowifiAppServiceStub struct{}

func (s *vowifiAppServiceStub) TriggerMOBIKE(oldIP, newIP string) error { return nil }

// teardownVoWiFiForReconnect stub - no-op.
func (p *Pool) teardownVoWiFiForReconnect(deviceID string) bool { return false }

// enableVoWiFiWhenReady stub - no-op.
func (p *Pool) enableVoWiFiWhenReady(deviceID string, timeout any, reason string) error { return nil }

// RestoreRadioAfterVoWiFi stub - no-op.
func (p *Pool) RestoreRadioAfterVoWiFi(deviceID string) error { return nil }

// EnableVoWiFi stub - no-op.
func (p *Pool) EnableVoWiFi(deviceID string) error { return nil }

// RestartVoWiFi stub - no-op.
func (p *Pool) RestartVoWiFi(deviceID string) error { return nil }

// CancelUSSD stub.
func (p *Pool) CancelUSSD(ctx any, deviceID string, extra ...any) error { return nil }
