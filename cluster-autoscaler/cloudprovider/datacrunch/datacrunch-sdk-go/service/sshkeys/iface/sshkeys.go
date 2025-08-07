package interfaces

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/sshkeys"
)

// SSHKeyAPI provides the interface for the SSH key service
type SSHKeyAPI interface {
	// ListSSHKeys lists all SSH keys
	ListSSHKeys() ([]*sshkeys.SSHKeyResponse, error)
	// GetSSHKey gets a single SSH key by ID
	GetSSHKey(id string) (*sshkeys.SSHKeyResponse, error)
	// CreateSSHKey creates a new SSH key
	CreateSSHKey(input *sshkeys.CreateSSHKeyInput) (*sshkeys.SSHKeyResponse, error)
	// DeleteSSHKeys deletes multiple SSH keys
	DeleteSSHKeys(input *sshkeys.DeleteSSHKeysInput) error
	// DeleteSSHKey deletes a single SSH key by ID
	DeleteSSHKey(id string) error
}

var _ SSHKeyAPI = (*sshkeys.SSHKey)(nil)
