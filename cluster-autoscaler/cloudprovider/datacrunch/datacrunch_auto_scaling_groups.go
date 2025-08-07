package datacrunch

import (
	"fmt"

	"k8s.io/klog"
)

type Asg struct {
	manager       *DatacrunchManager
	minSize       int
	maxSize       int
	loccationCode string
	id            string
}

// MaxSize returns maximum size of the node group.
func (asg *Asg) MaxSize() int {
	return asg.maxSize
}

// MinSize returns minimum size of the node group.
func (asg *Asg) MinSize() int {
	return asg.minSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kubernetes.
func (asg *Asg) TargetSize() (int, error) {
	size, err := asg.manager.GetAsgSize(asg)
	return int(size), err
}

// IncreaseSize increases Asg size
func (asg *Asg) IncreaseSize(delta int) error {
	klog.Infof("increase ASG:%s with %d nodes", asg.Id(), delta)
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
}
