package sources

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	influxdb "github.com/influxdb/influxdb/client"
	kube_api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kube_labels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"strings"
)

var (
	// simple eap
	argEapDbTable      = flag.String("eap_influxdb_table", "/^default\\.eap-controller-.*\\.eap-container\\.dmr/i", "Influxdb table name")
	simple_eap_columns = []string{"request_count"}
)

func query(source *InfluxdbSource, columns []string, table string, k int) ([]*influxdb.Series, error) {
	pt := int(source.Poll_time.Seconds())
	select_columns := strings.Join(columns, ",")
	query := fmt.Sprintf("SELECT %s FROM %s WHERE time > now() - %ds AND time < now() - %ds", select_columns, table, pt*(k+1), pt*k)
	series, err := source.client.Query(query, influxdb.Second)
	if err != nil {
		return nil, err
	}

	return series, nil
}

func toMap(series []*influxdb.Series) (map[string]int64, error) {
	sm := make(map[string]int64)

	for _, s := range series {
		value, err := toInt64(toValue(s.Points))
		if err != nil {
			return nil, err
		}
		if previous, found := sm[s.GetName()]; found {
			value = max64(previous, value)
		}
		sm[s.GetName()] = value
	}

	return sm, nil
}

type SimpleEapMetric struct {
	currentReplicas int
	selector kube_labels.Selector
}

func NewSimpleEapMetric() *SimpleEapMetric {
	return &SimpleEapMetric{selector: ParseSelector(*eapSelector)}
}

func (self *SimpleEapMetric) Execute(source *InfluxdbSource) error {
	glog.Infof("Querying InfluxDB data for EAP requests ...")

	// current data

	newS, err := query(source, simple_eap_columns, *argEapDbTable, 0)
	if err != nil {
		return err
	}

	n := int64(len(newS))

	if n == 0 {
		return nil
	}

	// previous data

	oldS, err := query(source, simple_eap_columns, *argEapDbTable, 1)
	if err != nil {
		return err
	}

	oldMap, err := toMap(oldS)
	if err != nil {
		return err
	}

	sum := int64(0)
	for _, s := range newS {
		previous := oldMap[s.GetName()]
		value, err := toInt64(toValue(s.Points))
		if err != nil {
			return err
		}

		diff := value - previous                          // new requests
		sum += (diff / int64(source.Poll_time.Seconds())) // average req / sec
	}

	replicas := int((sum/n)/int64(*eapPodRate)) + 1
	// limit replicas
	if replicas > *maxEapPods {
		replicas = *maxEapPods
	}

	if (replicas <= 0) {
		return nil
	}

	if replicas > self.currentReplicas {
		glog.Infof("Scaling up: %v [%v]", replicas, self.currentReplicas)

		err := source.kubeClient.SetReplicas(*eapReplicationController, replicas)
		if err != nil {
			return err
		}
	}

	if replicas < self.currentReplicas {
		glog.Infof("Scaling down: %v [%v]", replicas, self.currentReplicas)

		err := inspect(source.kubeClient, self, self.currentReplicas, replicas)
		if err != nil {
			return err
		}
	}

	self.currentReplicas = replicas

	return nil
}

func (self *SimpleEapMetric) GetSelector() kube_labels.Selector {
	return self.selector
}

func (self *SimpleEapMetric) CanScaleDown(pod *kube_api.Pod) bool {
	// TODO
	return true
}

func (self *SimpleEapMetric) SuspendPod(pod *kube_api.Pod) error {
	pod.Labels["state"] = "Suspended"
	return nil
}

func (self *SimpleEapMetric) ResumePod(pod *kube_api.Pod) error {
	pod.Labels["state"] = "Running"
	return nil
}
