package datacrunch

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/datacrunch/session"
)

type datacrunchSDKProvider struct {
	session *session.Session
}

func createDatacrunchSDKProvider() (*datacrunchSDKProvider, error) {
	sess := session.NewFromEnv()
	provider := &datacrunchSDKProvider{
		session: sess,
	}

	return provider, nil
}
