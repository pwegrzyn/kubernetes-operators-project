package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// THIS IS OUR API SCAFFOLDING!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// H2DatabaseSpec defines the desired state of H2Database
// +k8s:openapi-gen=true
type H2DatabaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Size is the size of the h2 deployment
	// Imporant: having more that 2 pods in the deplyoment is probably not necessary,
	// as currently H2 only supports running on a single node or in HA mode with
	// a cluster of size 2
	Size int32 `json:"size"`

	// URL to which the operator should POST DB backups, leave as 'skip' string if you don't want backups
	Backup string `json:"backup"`

	// Indicate whether to try to run the DBs as a connected cluster; will only be considered when there
	// are exactly two DB instances running (since H2 demands it); 'yes' or 'no'
	Clustering string `json:"clustering"`
}

// H2DatabaseStatus defines the observed state of H2Database
// +k8s:openapi-gen=true
type H2DatabaseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Nodes are the names of the h2 pods
	Nodes []string `json:"nodes"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// H2Database is the Schema for the h2databases API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=h2databases,scope=Namespaced
type H2Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   H2DatabaseSpec   `json:"spec,omitempty"`
	Status H2DatabaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// H2DatabaseList contains a list of H2Database
type H2DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []H2Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&H2Database{}, &H2DatabaseList{})
}
