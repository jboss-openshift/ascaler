package sources

import (
	"fmt"
	"strings"

	kube_api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kube_fields "github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	kube_labels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"
	"time"
)

type KubeSource struct {
	Poll_time   *time.Duration
	client      *KubeClient
	environment *Environment
	selectors   []string
	data        map[string]QueryEntry
}

func (self *KubeSource) parsePod(pod *kube_api.Pod) *Pod {
	localPod := Pod{
		Namespace:  pod.Namespace,
		Name:       pod.Name,
		ID:         pod.UID,
		PodIP:      pod.Status.PodIP,
		Hostname:   pod.Status.HostIP,
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
				localContainer.Pod = localPod
				localContainer.Name = container.Name
				localContainer.Host = env.GetHost(pod, port)
				localContainer.DmrPort = env.GetPort(pod, port)
				localPod.Containers = append(localPod.Containers, localContainer)
				break
			}
		}
	}

	if len(localPod.Containers) == 0 {
		glog.Warningf("No matching containers found ('mgmt' port name or 9990 container port) for pod %s", pod.Name)
	}

	glog.V(2).Infof("found pod: %+v", localPod)

	return &localPod
}

func (self *KubeSource) getPods(selector string) ([]Pod, error) {
	sc, err := kube_labels.Parse(selector)
	if err != nil {
		return nil, err
	}

	pods, err := self.client.Pods(*argNamespace).List(sc, kube_fields.Everything())
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

		if len(pods) == 0 {
			glog.Warningf("No pods found for selector %s", selector)
			continue
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
			err = entry.Calculate(self.client)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (self *KubeSource) GetData(selector string) QueryEntry {
	return self.data[selector]
}

func (self *KubeSource) PutData(selector string, value QueryEntry) {
	self.data[selector] = value
}

func NewKubeSource(d *time.Duration) (*KubeSource, error) {
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
		Poll_time:   d,
		client:      kubeClient,
		environment: newEnvironment(),
		selectors:   []string{*eapSelector},
		data:        make(map[string]QueryEntry),
	}, nil
}
