package main

import (
	"flag"
	"os"
	"strings"
	"time"

	"github.com/jboss-openshift/ascaler/sources"
	"github.com/golang/glog"
)

var argPollDuration = flag.Duration("poll_duration", 10*time.Second, "Polling duration")

func main() {
	flag.Parse()
	glog.Infof(strings.Join(os.Args, " "))
	glog.Infof("AScaler version %v", ascalerVersion)

	err := doWork()
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func doWork() error {
	source, err := sources.NewSource(argPollDuration)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(*argPollDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := source.CheckData()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
