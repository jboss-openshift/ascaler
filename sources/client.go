package sources

import (
	"crypto/tls"
	"crypto/x509"
	kube_api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kube_client "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"io/ioutil"
	"net/http"
	"os"
)

type KubeClient struct {
	client *kube_client.Client
}

func (self *KubeClient) Pods(namespace string) kube_client.PodInterface {
	return self.client.Pods(namespace)
}

func (self *KubeClient) GetReplicas(name string) (int, error) {
	rc, err := self.client.ReplicationControllers(*argNamespace).Get(name)
	if err != nil {
		return 0, err
	}

	return rc.Spec.Replicas, nil
}

func (self *KubeClient) SetReplicas(name string, replicas int) error {
	rc, err := self.client.ReplicationControllers(*argNamespace).Get(name)
	if err != nil {
		return err
	}

	rc.Spec.Replicas = replicas

	_, err = self.client.ReplicationControllers(*argNamespace).Update(rc)
	if err != nil {
		return err
	}

	return nil
}

func createTransport() (*http.Transport, error) {
	// run as insecure
	if *argMasterInsecure {
		return nil, nil
	}

	// Load client cert
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		return nil, err
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(*caFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	transport := &http.Transport{TLSClientConfig: tlsConfig}

	return transport, nil
}

func createClient(transport *http.Transport) *kube_client.Client {
	var bearerToken string;
	if argBearerTokenFile != nil {
		if _, err := os.Stat(*argBearerTokenFile); err == nil {
			buf, _ := ioutil.ReadFile(*argBearerTokenFile)
			bearerToken = string(buf)
		}
	}
	if transport != nil {
		return kube_client.NewOrDie(&kube_client.Config{
			Host:      os.ExpandEnv(*argMaster),
			Version:   *argMasterVersion,
			Transport: transport,
			BearerToken: bearerToken,
		})
	} else {
		return kube_client.NewOrDie(&kube_client.Config{
			Host:     os.ExpandEnv(*argMaster),
			Version:  *argMasterVersion,
			Insecure: *argMasterInsecure,
			BearerToken: bearerToken,
		})
	}
}

func newKubeClient(transport *http.Transport) *KubeClient {
	return &KubeClient{client: createClient(transport)}
}
