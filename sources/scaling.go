package sources

import (
	kube_api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kube_labels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"
)

type Inspector interface {
	GetReplicationController() string
	GetSelector() kube_labels.Selector
	CanScaleDown(pod *kube_api.Pod) bool
	SuspendPod(pod *kube_api.Pod) error
	ResumePod(pod *kube_api.Pod) error
}

func Scale(client *KubeClient, inspector Inspector, currentReplicas, newReplicas int) error {
	if newReplicas <= 0 {
		return nil
	}

	if newReplicas > currentReplicas {
		glog.Infof("Scaling up: %v [%v]", newReplicas, currentReplicas)

		err := client.SetReplicas(inspector.GetReplicationController(), newReplicas)
		if err != nil {
			return err
		}
	}

	if newReplicas < currentReplicas {
		glog.Infof("Scaling down: %v [%v]", newReplicas, currentReplicas)

		err := inspect(client, inspector, currentReplicas, newReplicas)
		if err != nil {
			return err
		}
	}

	return nil
}

func inspect(client *KubeClient, inspector Inspector, currentReplicas, newReplicas int) error {
	diff := currentReplicas - newReplicas

	for diff > 0 {
		pods, err := client.Pods(kube_api.NamespaceAll).List(inspector.GetSelector())
		if err != nil {
			return err
		}

		for _, pod := range pods.Items {
			if diff <= 0 {
				break
			}

			if pod.Status.Phase == kube_api.PodRunning {
				podPtr := &pod

				err := client.SuspendPod(inspector, podPtr)
				if err != nil {
					return err
				}

				if inspector.CanScaleDown(podPtr) {
					err := client.ScaleDown(podPtr)
					if err != nil {
						return err
					}
					diff--
				} else {
					err := client.ResumePod(inspector, podPtr)
					if err != nil {
						return err
					}
				}
			}
		}

	}

	return nil
}
