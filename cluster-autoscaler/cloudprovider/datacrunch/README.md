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
    "gpu": "24.04.kubernetes1.31.1.cuda12.9.qcow2",
    "cpu": "24.04.kubernetes1.31.1.cuda12.9.qcow2"
  },
  "sshKeyIDs": ["your-sshkey-id"],
  "billingConfig": {
    "price": "FIXED_PRICE",
    "contract": "PAY_AS_YOU_GO"
  },
  "debug": false,
  "availableLocations": ["FIN-02", "FIN-03"],
  "labels": [],
  "startupScript": "base64 encoded startup script",
  "startupScriptEnv": {
    "MASTER_IP": "",
    "MASTER_PORT": "",
    "JOIN_TOKEN": "",
    "JOIN_HASH_FULL": ""
  },
  "additionalVolumes": [],
  "taints": []
}
```
## Configuration reference

The JSON above is the authoritative format. This table summarizes top‑level keys for quick reference:

| Key | Type | Required | Default | Notes |
|-----|------|----------|---------|-------|
| image.gpu | string | optional | — | Image for GPU nodes |
| image.cpu | string | optional | — | Image for CPU nodes |
| sshKeyIDs | array<string> | required | — | SSH key IDs to inject |
| billingConfig.price | string | optional | — | One of DYNAMIC_PRICE or FIXED_PRICE |
| billingConfig.contract | string | optional | — | One of LONG_TERM, PAY_AS_YOU_GO, or SPOT |
| debug | bool | optional | false | Enables additional provider‑side diagnostics |
| availableLocations | array<string> | required | — | Location codes eligible for provisioning |
| startupScript | string (base64) | required | — | Base64‑encoded startup script executed on boot |
| startupScriptEnv | map<string,string> | optional | — | Extra environment variables for the startup script |
| additionalVolumes | array<object> | optional | — | Each item: { name, size (GB), type (HDD|NVMe) } |
| taints | array<object> | optional | — | Standard k8s taint objects applied to nodes |



`DATACRUNCH_CLUSTER_CONFIG_FILE` Can be used as alternative to `DATACRUNCH_CLUSTER_CONFIG`. This is the path to a file containing the JSON structure described above. The file will be read and the contents will be used as the configuration.

**NOTE**: In contrast to `DATACRUNCH_CLUSTER_CONFIG`, this file is not base64 encoded.

Node groups must be defined with the `--nodes=<min-servers>:<max-servers>:<instance-type>:<name>` flag.

Multiple flags will create multiple node pools. For example:
```
--nodes=1:5:1A6000.10V:as-test-a6000
--nodes=0:10:CPU.4V.16G:cpu-workers
--nodes=1:3:1H100.20V:gpu-h100-pool
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

## Support and caveats

- Hostname format: Instances created by this provider include an internal magic separator in their hostname that encodes the ASG name (format: <asg-name>-<magic>-<location>-<timestamp>). The autoscaler relies on this to identify group membership.
- No legacy fallback: If instances are created outside this provider with different hostname conventions, they may not be associated with the expected ASG by the autoscaler.
- ProviderID format: datacrunch://<location>/<hostname>.

## Debugging

To enable debug logging, run the autoscaler with `--v=4` or higher.
At `--v=7` the autoscaler logs CreateInstance request bodies for troubleshooting; response bodies and headers are not logged.