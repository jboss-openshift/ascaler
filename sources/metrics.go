package sources
import (
	influxdb "github.com/influxdb/influxdb/client"
	"flag"
	"github.com/golang/glog"
	"fmt")

var (
	argDbTable = flag.String("eap_influxdb_table", "/^default\\.eap-controller-.*\\.eap-container\\.dmr/i", "Influxdb table name")
)

type SimpleEapMetric struct {
	currentReplicas int
}

func query(source *InfluxdbSource, k int) ([]*influxdb.Series, error) {
	pt := int(source.Poll_time.Seconds())
	query := fmt.Sprintf("SELECT %s FROM %s WHERE time > now() - %ds AND time < now() - %ds", "request_count", *argDbTable, pt * (k + 1), pt * k)
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

func (self *SimpleEapMetric) Execute(source *InfluxdbSource) (error) {
	glog.Infof("Querying InfluxDB data for EAP requests ...")

	// current data

	newS, err := query(source, 0)
	if err != nil {
		return err
	}

	n := int64(len(newS))

	if n == 0 {
		return nil
	}

	// previous data

	oldS, err := query(source, 1)
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

		diff := value - previous // new requests
		sum += (diff / int64(source.Poll_time.Seconds())) // average req / sec
	}

	replicas := int((sum / n) / int64(*eapPodRate)) + 1
	// limit replicas
	if replicas > *maxEapPods {
		replicas = *maxEapPods
	}

	if replicas > 0 && self.currentReplicas != replicas {
		glog.Infof("Applying replicas: %v", replicas)

		err := source.kubeClient.SetReplicas(*eapReplicationController, replicas)
		if err != nil {
			return err
		}

		self.currentReplicas = replicas
	}

	return nil
}
