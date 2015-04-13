package sources

import (
	"net/http"
	"fmt"
	"encoding/json"
	"bytes"
	"time"
	"github.com/golang/glog"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types")

type DmrContainer struct {
	Pod		Pod
	Name    string
	Host    string
	DmrPort int
}

type DmrAttributeRequest struct {
	Operation 	string	`json:"operation"`
	Name      	string	`json:"name"`
	Pretty		int		`json:"json.pretty"`
}

type DmrResourceRequest struct {
	Operation 			string		`json:"operation"`
	IncludeRuntime      bool		`json:"include-runtime"`
	Address				[]string	`json:"address"`
	Pretty				int			`json:"json.pretty"`
}

type DmrResponse struct {
	Outcome 				string		`json:"outcome"`
	Result      			interface{}	`json:"result"`
	FailureDescription      string		`json:"failure-description"`
	RolledBacked      		bool		`json:"rolled-back"`
}

type WebResult struct {
	BytesReceived		StringInt			`json:"bytesReceived"`
	BytesSent			StringInt			`json:"bytesSent"`
	EnableLookups		bool				`json:"enable-lookups"`
	Enabled				bool				`json:"enabled"`
	ErrorCount			StringInt			`json:"errorCount"`
	Executor			string				`json:"executor"`
	MaxConnections		int					`json:"max-connections"`
	MaxPostSize			int64				`json:"max-post-size"`
	MaxSavePostSize		int64				`json:"max-save-post-size"`
	MaxTime				StringInt			`json:"maxTime"`
	Name				string				`json:"name"`
	ProcessingTime		StringInt			`json:"processingTime"`
	Protocol			string				`json:"protocol"`
	ProxyName			string				`json:"proxy-name"`
	ProxyPort			string				`json:"proxy-port"`
	RedirectPort		int					`json:"redirect-port"`
	RequestCount		StringInt			`json:"requestCount"`
	Scheme				string				`json:"scheme"`
	Secure				bool				`json:"secure"`
	SocketBinding		string				`json:"socket-binding"`
	SSL					string				`json:"ssl"`
	VirtualServer		string				`json:"virtual-server"`
}

type InstanceData struct {
	Timestamp int64 // previous timestamp
	Previous int64 // previous request count

	Current int64 // current request count
}

type RequestCountData struct {
	pods map[types.UID]*InstanceData // 1pod --> 1eap container, no locking/synch atm
	currentPods []types.UID
	currentReplicas int // how many replicas we currently have
}

func (self *RequestCountData) Calculate(client *KubeClient) error {
	size := len(self.pods)

	if size == 0 {
		// should probably not happen
		glog.Warning("EAP size is zero!")
		return nil
	}

	currentTime := time.Now().Unix()

	// new map
	currentPods := make(map[types.UID]*InstanceData)

	sum := int64(0)
	for _, uid := range self.currentPods {
		data := self.pods[uid]
		timeDiff := (currentTime - data.Timestamp)
		if timeDiff > 0 {
			podAvg := (data.Current - data.Previous) / timeDiff
			sum += podAvg
		}
		data.Timestamp = currentTime
		data.Previous = data.Current
		data.Current = int64(0)
		currentPods[uid] = data
	}
	replicas := int(sum / int64(*eapPodRate)) + 1

	// cleanup pods info
	self.pods = currentPods // forget old pods/containers
	self.currentPods = nil

	glog.Infof("Current EAP replicas: %v ... [%v / %v]", replicas, sum, *eapPodRate)

	// only poke k8s if we have to change replicas size
	if (replicas != self.currentReplicas) {
		err := client.SetReplicas(*eapReplicationController, replicas)
		if err != nil {
			return err
		}
	}

	self.currentReplicas = replicas

	return nil
}

func (self *DmrContainer) GetName() string {
	return self.Name
}

func (self *DmrContainer) CheckStats(kube *KubeSource) (error) {
	dmrRequest := DmrResourceRequest{
		Operation: "read-resource",
		IncludeRuntime: true,
		Address: []string{"subsystem", "web", "connector", "http"},
		Pretty: 1,
	}

	wr := WebResult{}
	dmrResponse := DmrResponse{
		Result: &wr,
	}

	err := self.getStats(&dmrRequest, &dmrResponse)
	if err != nil {
		return err
	}

	rcValue := int64(wr.RequestCount.Value)

	queryEntry := kube.GetData(*eapSelector)
	if queryEntry != nil {
		requestCountData := queryEntry.(*RequestCountData)

		requestCountData.currentPods = append(requestCountData.currentPods, self.Pod.ID)

		data := requestCountData.pods[self.Pod.ID]
		if data != nil {
			data.Current += rcValue
		} else {
			// best guess, just set timestamp to "previous" poll
			data = &InstanceData{Timestamp: time.Now().Unix() - int64(*kube.Poll_time), Current: rcValue}
			requestCountData.pods[self.Pod.ID] = data
		}
	} else {
		data := &InstanceData{Timestamp: time.Now().Unix() - int64(*kube.Poll_time), Current: rcValue}
		requestCountDataPtr := &RequestCountData{
			pods: map[types.UID]*InstanceData{self.Pod.ID:data},
			currentPods: []types.UID{self.Pod.ID},
		}
		kube.PutData(*eapSelector, requestCountDataPtr)
	}

	return nil
}

func (self *DmrContainer) getStats(request interface{}, result interface{}) error {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s:%d/management", self.Host, self.DmrPort)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	err = PostRequestAndGetValue(&http.Client{}, req, result)
	if err != nil {
		return err
	}

	return nil
}
