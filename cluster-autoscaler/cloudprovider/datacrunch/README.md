# Cluster Autoscaler for DataCrunch

The cluster autoscaler for DataCrunch scales worker nodes.

## Configuration

`DATACRUNCH_CLIENT_ID` Required DataCrunch OAuth2 client ID.

`DATACRUNCH_CLIENT_SECRET` Required DataCrunch OAuth2 client secret.

`DATACRUNCH_BASE_URL` Optional DataCrunch API base URL. Defaults to `https://api.datacrunch.io/v1`.

`DATACRUNCH_CLUSTER_CONFIG` Base64 encoded JSON according to the following structure:

```json
{
  "image": {
    "gpu": "ubuntu-24.04-cuda-12.8-open-docker",
    "cpu": "ubuntu-24.04"
  },
  "sshKeyIDs": ["cace0fae-a43d-40d4-94bd-07178102923e"],
  "billingConfig": {
    "price": "FIXED_PRICE",
    "contract": "PAY_AS_YOU_GO"
  },
  "labels": [],
  "startupScript": "base64 encoded startup script",
  "startupScriptEnv": {
    "K8S_VERSION": "1.32.7",
    "MASTER_IP": "",
    "MASTER_PORT": "",
    "JOIN_TOKEN": "",
    "JOIN_HASH_FULL": ""
  },
  "additionalVolumes": [],
  "taints": []
}
```

`DATACRUNCH_CLUSTER_CONFIG_FILE` Can be used as alternative to `DATACRUNCH_CLUSTER_CONFIG`. This is the path to a file containing the JSON structure described above. The file will be read and the contents will be used as the configuration.

**NOTE**: In contrast to `DATACRUNCH_CLUSTER_CONFIG`, this file is not base64 encoded.

Node groups must be defined with the `--nodes=<min-servers>:<max-servers>:<instance-type>:<region>:<name>` flag.

Multiple flags will create multiple node pools. For example:
```
--nodes=1:5:1A6000.10V:FIN-01:as-test-a6000
--nodes=0:10:CPU.4V.16G:FIN-01:cpu-workers
--nodes=1:3:1H100.20V:FIN-01:gpu-h100-pool
```

You can find a complete deployment sample under [examples/cluster-autoscaler-run-on-master.yaml](examples/cluster-autoscaler-run-on-master.yaml). This single file contains all required Kubernetes resources including namespace, RBAC, secrets, configmap, and deployment. Please be aware that you should change the values within this deployment to reflect your cluster:

- Replace `your-client-id` and `your-client-secret` in the Secret
- Update `your-ssh-key-id` in the ConfigMap
- Configure your cluster join parameters (`MASTER_IP`, `JOIN_TOKEN`, `JOIN_HASH_FULL`)
- Modify the `--nodes` flags to match your desired instance types and scaling limits
- Update the startup script with your actual base64-encoded cluster join script

## Development

Make sure you're inside the `cluster-autoscaler` root folder.

1.) Build the `cluster-autoscaler` binary:

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cluster-autoscaler-datacrunch --tags datacrunch .
```

2.) Build the docker image:

```
docker build -t datacrunch/cluster-autoscaler:dev .
```

3.) Push the docker image to Docker hub:

```
docker push datacrunch/cluster-autoscaler:dev
```

## Debugging

To enable debug logging, set the log level of the autoscaler to at least level 4 via cli flag: `--v=4`  
The logs will include all requests and responses made towards the DataCrunch API including headers and body.