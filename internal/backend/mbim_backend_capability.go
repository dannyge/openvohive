package backend

import "github.com/openvohive/openvohive/pkg/mbim"

func (b *MBIMBackend) Capability() *mbim.Capabilities {
	return b.source.Capability()
}
