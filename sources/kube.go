package sources

import (
	"fmt"
	"os"
	"strings"

	kube_api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kube_client "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kube_labels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"
	"net/http"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil")

type KubeSource struct {
	client      *kube_client.Client
	environment *Environment
	selectors	[]string
	data		map[string]QueryEntry
}

func (self *KubeSource) parsePod(pod *kube_api.Pod) *Pod {
	localPod := Pod{
		Namespace:  pod.Namespace,
		Name:       pod.Name,
		ID:         pod.UID,
		PodIP:      pod.Status.PodIP,
		Hostname:   pod.Status.Host,
		Status:     string(pod.Status.Phase),
		Labels:     make(map[string]string, 0),
		Containers: make([]Container, 0),
	}
	for key, value := range pod.Labels {
		localPod.Labels[key] = value
	}

	env := *(self.environment)

	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "mgmt" || port.ContainerPort == 9990 {
				localContainer := newDmrContainer()
				localContainer.Name = container.Name
				localContainer.Host = env.GetHost(pod, port)
				localContainer.DmrPort = env.GetPort(pod, port)
				localPod.Containers = append(localPod.Containers, localContainer)
				break
			}
		}
	}
	glog.V(2).Infof("found pod: %+v", localPod)

	return &localPod
}

func (self *KubeSource) getPods(selector string) ([]Pod, error) {
	sc, err := kube_labels.ParseSelector(selector)
	if err != nil {
		return nil, err
	}

	pods, err := self.client.Pods(kube_api.NamespaceAll).List(sc)
	if err != nil {
		return nil, err
	}
	glog.V(1).Infof("got pods from api server %+v", pods)
	out := make([]Pod, 0)
	for _, pod := range pods.Items {
		if pod.Status.Phase == kube_api.PodRunning {
			pod := self.parsePod(&pod)
			out = append(out, *pod)
		}
	}

	return out, nil
}

func (self *KubeSource) CheckData() error {
	for _, selector := range self.selectors {

		pods, err := self.getPods(selector)
		if err != nil {
			return err
		}

		for _, pod := range pods {
			for _, container := range pod.Containers {
				glog.Infof("Container --> %s", container.GetName())

				err := container.CheckStats(self)

				if err != nil {
					glog.Errorf("Error checking container [%s] stats: %s", container.GetName(), err)
				}
			}
		}

		entry := self.GetData(selector)
		if entry != nil {
			err = entry.Calculate(self)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (self *KubeSource) GetReplicas(name string) (int, error) {
	rc, err := self.client.ReplicationControllers(kube_api.NamespaceAll).Get(name)
	if err != nil {
		return 0, err
	}

	return rc.Spec.Replicas, nil;
}

func (self *KubeSource) SetReplicas(name string, replicas int) error {
	rc, err := self.client.ReplicationControllers(kube_api.NamespaceAll).Get(name)
	if err != nil {
		return err
	}

	rc.Spec.Replicas = replicas

	_, err = self.client.ReplicationControllers(kube_api.NamespaceAll).Update(rc)
	if err != nil {
		return err
	}

	return nil
}

func (self *KubeSource) GetData(selector string) QueryEntry {
	return self.data[selector]
}

func (self *KubeSource) PutData(selector string, value QueryEntry) {
	self.data[selector] = value
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

func newKubeClient(transport *http.Transport) *kube_client.Client {
	if transport != nil {
		return kube_client.NewOrDie(&kube_client.Config{
			Host:     os.ExpandEnv(*argMaster),
			Version:  *argMasterVersion,
			Transport: transport,
		})
	} else {
		return kube_client.NewOrDie(&kube_client.Config{
			Host:     os.ExpandEnv(*argMaster),
			Version:  *argMasterVersion,
			Insecure: *argMasterInsecure,
		})
	}
}

func newKubeSource() (*KubeSource, error) {
	if !(strings.HasPrefix(*argMaster, "http://") || strings.HasPrefix(*argMaster, "https://")) {
		*argMaster = "http://" + *argMaster
	}
	if len(*argMaster) == 0 {
		return nil, fmt.Errorf("kubernetes_master flag not specified")
	}

	transport, err := createTransport()
	if err != nil {
		return nil, err
	}

	kubeClient := newKubeClient(transport)

	return &KubeSource{
		client:      	kubeClient,
		environment: 	newEnvironment(),
		selectors:		[]string{*eapSelector},
		data:		 	make(map[string]QueryEntry),
	}, nil
}
