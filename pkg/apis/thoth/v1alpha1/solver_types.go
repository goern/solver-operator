package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SolverPhase is the Solver Phase type
type SolverPhase string

// These are the Solver Phases
const (
	SolverPhaseInitial   SolverPhase = "Initializing"
	SolverPhaseRunning               = "Running"
	SolverPhaseCompleted             = "Completed"
)

// SolverSpec defines the desired state of Solver
type SolverSpec struct {
	// Pod defines the policy for pods owned by the operator.
	// This field cannot be updated once the CR is created.
	Pod               *PodPolicy `json:"pod,omitempty"`
	Packages          string     `json:"packages"`          // the software stack specification
	IncludeTransitive bool       `json:"includeTransitive"` // also solve transitive dependencies
	Output            string     `json:"output"`            // output ... FIXME
	KeepJob           bool       `json:"keepJob"`           // do not delete the Job even if it succeeded
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
}

// SolverStatus defines the observed state of Solver
type SolverStatus struct {
	// Phase indicates the state this Solver is in.
	// Phase goes as one way as below:
	//   Initial -> Running -> Completed
	Phase          SolverPhase `json:"phase"`
	StartTime      metav1.Time `json:"startTime"`
	CompletionTime metav1.Time `json:"completionTime"`
	Active         int         `json:"active,omitempty"`
	Failed         int         `json:"failed,omitempty"`
	Succeeded      int         `json:"succeeded,omitempty"`
	// TODO we should have conditions...
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Solver is the Schema for the solvers API
// +k8s:openapi-gen=true
type Solver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SolverSpec   `json:"spec,omitempty"`
	Status SolverStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SolverList contains a list of Solver
type SolverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Solver `json:"items"`
}

// PodPolicy defines the policy for pods owned by vault operator.
type PodPolicy struct {
	// Resources is the resource requirements for the containers.
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Solver{}, &SolverList{})
}
