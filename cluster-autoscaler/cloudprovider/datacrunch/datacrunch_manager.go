package datacrunch

import (
	"encoding/json"
	"errors"
	"io"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instance"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/datacrunch/datacrunch-sdk-go/service/instancetypes"
)

const (
	datacrunchProviderIDPrefix = "datacrunch://"
	GPULabel                   = "datacrunch.io/deployment-name"
)

type DatacrunchManager struct {
	cfg         *cloudConfig
	sdkProvider *datacrunchSDKProvider
	dcService   datacrunchWrapper
	asgs        map[string]*Asg
}

func createDatacrunchManager(cloudReader io.Reader) (*DatacrunchManager, error) {
	cfg := &cloudConfig{}
	if cloudReader != nil {
		decoder := json.NewDecoder(cloudReader)
		if err := decoder.Decode(cfg); err != nil {
			return nil, err
		}
	}

	if cfg.isValid() == false {
		return nil, errors.New("please check whether you have provided correct AccessKeyId,AccessKeySecret,RegionId or STS Token")
	}

	// create the sdk provider
	sdkProvider, err := createDatacrunchSDKProvider()
	if err != nil {
		return nil, err
	}

	// create the datacrunch wrapper
	dcService := datacrunchWrapper{
		instance.New(sdkProvider.session),
		instancetypes.New(sdkProvider.session),
	}
}
