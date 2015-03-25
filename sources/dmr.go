package sources

import (
	"net/http"
	"fmt"
	"encoding/json"
	"bytes"
	"time"
	"github.com/golang/glog")

type DmrContainer struct {
	Name    string      `json:"name,omitempty"`
	Host    string      `json:"host"`
	DmrPort int         `json:"dmrPort"`
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

type RequestCountData struct {
	Timestamp time.Time
	Previous int

	Current int
}

func (self *RequestCountData) Calculate(kube *KubeSource) error {
	previousRequestCount := self.Previous
	currentRequestCount := self.Current

	previousTime := self.Timestamp
	currentTime := time.Now()

	self.Previous = currentRequestCount // update count
	self.Timestamp = currentTime // update ts
	self.Current = 0 // reset current count

	timeDiff := (currentTime.Second() - previousTime.Second())

	if (timeDiff > 0) {
		replicas := (((currentRequestCount - previousRequestCount) / timeDiff) / *eapPodRate) + 1

		glog.Infof("Current EAP replicas: %v [(%v - %v) / %v / %v]", replicas, currentRequestCount, previousRequestCount, timeDiff, *eapPodRate)

		// skip if container was restarted
		if replicas > 0 {
			err := kube.SetReplicas(*eapReplicationController, replicas)
			if err != nil {
				return err
			}
		}
	}

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

	queryEntry := kube.GetData(*eapSelector)
	if queryEntry != nil {
		requestCountData, _ := queryEntry.(*RequestCountData)
		requestCountData.Current += wr.RequestCount.Value
	} else {
		requestCountDataPtr := &RequestCountData{Timestamp: time.Now(), Previous: 0, Current: wr.RequestCount.Value}
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
