# AScaler

AScaler is here to help you auto-scale your EAP instance.

## Running

```
Usage of ascaler
  -kubernetes_master=https://localhost:8443: Kubernetes master address
  -kubernetes_insecure=false: Trust Kubernetes master certificate (if using https)
  -token=/var/run/secrets/kubernetes.io/serviceaccount/token: file path to token
  -namespace=mynamespace: targeted namespace
  -cert=/var/certs/cert.crt: Client certificate
  -key=/var/certs/key.key: Certificate key
  -CA=/var/certs/root.crt CA certificate
  -eap_pod_rate=100: Set allowed request rate per second
  -jube=true: to force Jube usage
```

## Contributing

Normal fork, branch, PR process please.
