package sources

import (
	kube_api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kube_labels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
)

type Inspector interface {
	GetSelector() kube_labels.Selector
	CanScaleDown(pod *kube_api.Pod) bool
	SuspendPod(pod *kube_api.Pod) error
	ResumePod(pod *kube_api.Pod) error
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