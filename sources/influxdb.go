package sources

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	influxdb "github.com/influxdb/influxdb/client"
	"math"
	"os"
	"time"
)

var (
	argDbUsername = flag.String("influxdb_username", "root", "InfluxDB username")
	argDbPassword = flag.String("influxdb_password", "root", "InfluxDB password")
	argDbHost     = flag.String("influxdb_host", "localhost:8086", "InfluxDB host:port")
	argDbName     = flag.String("influxdb_name", "k8s", "Influxdb database name")
)

type InfluxdbSource struct {
	Poll_time  *time.Duration
	client     *influxdb.Client
	dbName     string
	lastWrite  time.Time
	kubeClient *KubeClient
	metrics    []Metric
}

type Metric interface {
	Execute(source *InfluxdbSource) error
}

func toInt64(n interface{}) (int64, error) {
	if f, ok := n.(float64); ok {
		return int64(f), nil
	}
	return 0, fmt.Errorf("Cannot convert %s to int64, expecting float64", n)
}

func toValue(points [][]interface{}) interface{} {
	return points[0][2]
}

func max64(x, y int64) int64 {
	return int64(math.Max(float64(x), float64(y)))
}

func (self *InfluxdbSource) CheckData() error {
	for _, metric := range self.metrics {
		err := metric.Execute(self)
		if err != nil {
			glog.Errorf("Error checking data for metric %s -> %s", metric, err)
		}
	}
	return nil
}

// TODO make this more generic
func getMetrics() []Metric {
	ms := make([]Metric, 0)
	return append(ms, &SimpleEapMetric{})
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
		Poll_time:  duration,
		client:     client,
		dbName:     *argDbName,
		lastWrite:  time.Now(),
		kubeClient: newKubeClient(transport),
		metrics:    getMetrics(),
	}, nil
}
