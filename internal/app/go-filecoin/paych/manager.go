package paych

import (
	"github.com/filecoin-project/go-address"
)

// Manager manages payment channel actor and the data store operations.
type Manager struct {
	store *Store
}

func (pm *Manager) AllocateLane(paychAddr address.Address) (uint64, error) {
	return 0, nil
}

func (pm *Manager) GetPaymentChannelInfo(paychAddr address.Address) (ChannelInfo, error) {
	return ChannelInfo{}, nil
}

func (pm *Manager) CreatePaymentChannel(payer, payee address.Address) (ChannelInfo, error) {
	return ChannelInfo{}, nil
}

func (pm *Manager) UpdatePaymentChannel(paychAddr address.Address) error {
	return nil
}
