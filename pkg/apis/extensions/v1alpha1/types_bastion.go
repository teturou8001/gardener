// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*Bastion)(nil)

// BastionResource is a constant for the name of the Bastion resource.
const BastionResource = "Bastion"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,path=bastions,singular=bastion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=IP,JSONPath=".status.ingress.ip",type=string,description="The public IP address of the temporary bastion host"
// +kubebuilder:printcolumn:name=Hostname,JSONPath=".status.ingress.hostname",type=string,description="The public hostname of the temporary bastion host"
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="The bastion's age."

// Bastion is a bastion or jump host that is dynamically created
// to provide SSH access to shoot nodes.
type Bastion struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the specification of this Bastion.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec BastionSpec `json:"spec"`
	// Status is the bastion's status.
	// +optional
	Status BastionStatus `json:"status,omitempty"`
}

// GetExtensionSpec implements Object.
func (b *Bastion) GetExtensionSpec() Spec {
	return &b.Spec
}

// GetExtensionStatus implements Object.
func (b *Bastion) GetExtensionStatus() Status {
	return &b.Status
}

// BastionSpec contains the specification for an SSH bastion host.
type BastionSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	// UserData is the base64-encoded user data for the bastion instance. This should
	// contain code to provision the SSH key on the bastion instance.
	// This field is immutable.
	UserData []byte `json:"userData"`
	// Ingress controls from where the created bastion host should be reachable.
	Ingress []BastionIngressPolicy `json:"ingress"`
}

// BastionIngressPolicy represents an ingress policy for SSH bastion hosts.
type BastionIngressPolicy struct {
	// IPBlock defines an IP block that is allowed to access the bastion.
	IPBlock networkingv1.IPBlock `json:"ipBlock"`
}

// BastionStatus holds the most recently observed status of the Bastion.
type BastionStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
	// Ingress is the external IP and/or hostname of the bastion host.
	Ingress corev1.LoadBalancerIngress `json:"ingress"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BastionList is a collection of Bastions.
type BastionList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Bastions.
	Items []Bastion
}
