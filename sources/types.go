package sources

import (
	"flag"

	kube_api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types"
	"encoding/json"
	"strconv"
	"time"
	"fmt")

var (
	argMaster         = flag.String("kubernetes_master", "https://localhost:8443", "Kubernetes master address")
	argMasterVersion  = flag.String("kubernetes_version", "v1beta2", "Kubernetes api version")
	argMasterInsecure = flag.Bool("kubernetes_insecure", false, "Trust Kubernetes master certificate (if using https)")

	certFile = flag.String("cert", "/opt/openshift/origin/openshift.local.certificates/admin/cert.crt", "A PEM eoncoded certificate file.")
	keyFile  = flag.String("key", "/opt/openshift/origin/openshift.local.certificates/admin/key.key", "A PEM encoded private key file.")
	caFile   = flag.String("CA", "/opt/openshift/origin/openshift.local.certificates/master/root.crt", "A PEM encoded CA's certificate file.")

	sourceType = flag.String("source", "k8s", "Source of metrics - direct EAP containers or influxdb")

	eapSelector = flag.String("eap_selector", "name=eapPod", "EAP pod selector")
	eapReplicationController = flag.String("eap_replication_controller", "eaprc", "EAP replication controller")
	eapPodRate = flag.Int("eap_pod_rate", 1000, "EAP pod rate") // allowed requests per second
	maxEapPods = flag.Int("max_eap_pods", 20, "Max EAP pod instances") // max EAP pod instances // TODO: set the right number
)

// PodState is the state of a pod, used as either input (desired state) or output (current state)
type Pod struct {
	Namespace  string            `json:"namespace,omitempty"`
	Name       string            `json:"name,omitempty"`
	ID         types.UID         `json:"id,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`
	Containers []Container       `json:"containers"`
	Status     string            `json:"status,omitempty"`
	PodIP      string            `json:"podIP,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type Container interface {
	GetName() string
	CheckStats(kube *KubeSource) (error)
}

type QueryEntry interface {
	Calculate(client *KubeClient) error
}

func newDmrContainer() *DmrContainer {
	return &DmrContainer{}
}

type Source interface {
	CheckData() (error)
}

func NewSource(d *time.Duration) (Source, error) {
	if *sourceType == "k8s" {
		return NewKubeSource(d)
	} else if (*sourceType == "influxdb") {
		return NewInfluxdbSource(d)
	} else {
		return nil, fmt.Errorf("No such source type: %s", *sourceType)
	}
}

type Environment interface {
	GetHost(pod *kube_api.Pod, port kube_api.Port) string
	GetPort(pod *kube_api.Pod, port kube_api.Port) int
}

type StringInt struct {
	Value int
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (strint *StringInt) UnmarshalJSON(value []byte) error {
	if value[0] == '"' {
		arr := value[1:len(value)-1]
		return json.Unmarshal(arr, &strint.Value)
	}
	return json.Unmarshal(value, &strint.Value)
}

// String returns the string value, or Itoa's the int value.
func (strint *StringInt) String() string {
	return strconv.Itoa(strint.Value)
}

// MarshalJSON implements the json.Marshaller interface.
func (strint StringInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(strint.Value)
}
