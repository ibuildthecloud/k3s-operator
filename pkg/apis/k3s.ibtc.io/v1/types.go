package v1

import (
	"encoding/json"

	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type K3s struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              K3sSpec   `json:"spec"`
	Status            K3sStatus `json:"status,omitempty"`
}

type K3sSpec struct {
	ControlPlaneEndpoint *Endpoint `json:"controlPlaneEndpoint,omitempty"`
	Channel              string    `json:"channel,omitempty"`
}

type Endpoint struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

type K3sStatus struct {
	ObservedGeneration   int64                               `json:"observedGeneration"`
	Ready                bool                                `json:"ready,omitempty"`
	CredentialSecretName string                              `json:"credentialSecretName,omitempty"`
	Token                string                              `json:"token,omitempty"`
	Conditions           []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type GenericMap struct {
	Data map[string]interface{} `json:"-"`
}

func (in GenericMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(in.Data)
}

func (in *GenericMap) UnmarshalJSON(data []byte) error {
	in.Data = map[string]interface{}{}
	return json.Unmarshal(data, &in.Data)
}

func (in *GenericMap) DeepCopyInto(out *GenericMap) {
	if err := convert.ToObj(in.Data, &out.Data); err != nil {
		panic(err)
	}
}
