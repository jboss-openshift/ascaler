package sources
import (
	"time"
	influxdb "github.com/influxdb/influxdb/client"
	"flag"
	"github.com/golang/glog"
	"os"
	"fmt"
	"math")

var (
	argDbUsername = flag.String("influxdb_username", "root", "InfluxDB username")
	argDbPassword = flag.String("influxdb_password", "root", "InfluxDB password")
	argDbHost = flag.String("influxdb_host", "localhost:8086", "InfluxDB host:port")
	argDbName = flag.String("influxdb_name", "k8s", "Influxdb database name")
	argDbTable = flag.String("influxdb_table", "/^default\\.eap-controller-.*\\.eap-container\\.dmr/i", "Influxdb table name")
)

type InfluxdbSource struct {
	Poll_time      *time.Duration
	client         *influxdb.Client
	dbName         string
	lastWrite      time.Time
	kubeClient     *KubeClient
}

func toInt64(n interface{}) (int64, error) {
	if f, ok := n.(float64); ok {
		return int64(f), nil
	}
	return 0, fmt.Errorf("Cannot convert %s to int64, expecting float64", n)
}

func (self *InfluxdbSource) query(k int) ([]*influxdb.Series, error) {
	pt := int(self.Poll_time.Seconds())
	query := fmt.Sprintf("SELECT %s FROM %s WHERE time > now() - %ds AND time < now() - %ds", "request_count", *argDbTable, pt * (k + 1), pt * k)
	series, err := self.client.Query(query, influxdb.Second)
	if err != nil {
		return nil, err
	}

	return series, nil
}

func toValue(points [][]interface{}) interface{} {
	return points[0][2]
}

func max64(x,y int64) int64 {
	return int64(math.Max(float64(x), float64(y)))
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

func (self *InfluxdbSource) CheckData() (error) {
	err := self.doCheckData()
	if err != nil {
		glog.Errorf("Error checking data: %s", err)
	}
	return nil
}

func (self *InfluxdbSource) doCheckData() (error) {
	glog.Infof("Querying InfluxDB data ...")

	// current data

	newS, err := self.query(0)
	if err != nil {
		return err
	}

	n := int64(len(newS))

	if n == 0 {
		return nil
	}

	// previous data

	oldS, err := self.query(1)
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
		sum += (diff / int64(self.Poll_time.Seconds())) // average req / sec
	}

	replicas := int((sum / n) / int64(*eapPodRate)) + 1
	// limit replicas
	if replicas > *maxEapPods {
		replicas = *maxEapPods
	}

	if replicas > 0 {
		err := self.kubeClient.SetReplicas(*eapReplicationController, replicas)
		if err != nil {
			return err
		}
	}

	return nil
}

func NewInfluxdbSource(duration *time.Duration) (Source, error) {
	config := &influxdb.ClientConfig{
		Host:     os.ExpandEnv(*argDbHost),
		Username: *argDbUsername,
		Password: *argDbPassword,
		Database: *argDbName,
		IsSecure: false,
	}
	client, err := influxdb.NewClient(config)
	if err != nil {
		return nil, err
	}

	client.DisableCompression()

	if err := client.CreateDatabase(*argDbName); err != nil {
		glog.Infof("Database creation failed - %s", err)
	}

	transport, err := createTransport()
	if err != nil {
		return nil, err
	}

	// Create the database if it does not already exist. Ignore errors.
	return &InfluxdbSource{
		Poll_time:        duration,
		client:         client,
		dbName:         *argDbName,
		lastWrite:      time.Now(),
		kubeClient:        newKubeClient(transport),
	}, nil
}
