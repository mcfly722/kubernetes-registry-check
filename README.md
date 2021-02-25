# kubernetes-registry-check

DaemonSet checks ability to connect from each Kubernetes cluster node to specified registries.
Results printed in json format and further could be handled by other monitoring system (f.e. FluentD to ElasticSearch).


Example of output json (preformatted):
```
{
	"Timestamp": "2021-02-25T09:21:56Z",
	"Source": {
		"PodName": "kubernetes-registry-check-f9w9g",
		"PodIP": "10.42.128.2",
		"HostName": "kub-test-master1",
		"HostIP": "192.168.1.101"
	},
	"Destination": "nexus.registry.contoso.com:45105",
	"Message": "{\"repositories\":[]}",
	"Success": true
}
```
or failed example (dns problem):
```
{
	"Timestamp": "2021-02-25T09:24:37Z",
	"Source": {
		"PodName": "kubernetes-registry-check-f9w9g",
		"PodIP": "10.42.128.2",
		"HostName": "kub-test-master1",
		"HostIP": "192.168.1.101"
	},
	"Destination": "nexus.registry.contoso.com:45105",
	"Message": "Get https://nexus.registry.contoso.com:45105/v2/_catalog: dial tcp: lookup nexus.registry.contoso.com on 10.43.0.10:53: read udp 10.42.128.2:38515-\u003e10.43.0.10:53: i/o timeout",
	"Success": false
}
```

### Deployment
Current configuration deploys DaemonSet to **monitoring** namespace. To deploy in another namespace (f.e. kube-system) you have to modify yaml and use appropriate name in kubectl commands.

```
./kubectl --server "<your k8s cluster address>" --token "<your token here>" --insecure-skip-tls-verify apply -f "kubernetes-registry-check.yaml"
```
### Adding new registry for monitoring
To add new registry for monitoring use next command:
```
./kubectl --server "<your k8s cluster address>" --token "<your token here>" --insecure-skip-tls-verify create secret docker-registry registry --namespace monitoring --docker-server="<registry address>" --docker-username="<registry user name>" --docker-password="<registry password>"
```
Several registries are supported.
