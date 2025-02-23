/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"

	identity "github.com/crossplaneio/stack-aws/apis/identity/v1alpha2"
	network "github.com/crossplaneio/stack-aws/apis/network/v1alpha2"
)

// Cluster statuses.
const (
	// The resource is inaccessible while it is being created.
	ClusterStatusCreating = "CREATING"

	ClusterStatusActive = "ACTIVE"
)

// Error strings
const (
	errResourceIsNotEKSCluster = "The managed resource is not an EKSCluster"
)

// EKSRegion represents an EKS enabled AWS region.
type EKSRegion string

// EKS regions.
const (
	// EKSRegionUSWest2 - us-west-2 (Oregon) region for eks cluster
	EKSRegionUSWest2 EKSRegion = "us-west-2"
	// EKSRegionUSEast1 - us-east-1 (N. Virginia) region for eks cluster
	EKSRegionUSEast1 EKSRegion = "us-east-1"
	// EKSRegionUSEast2 - us-east-2 (Ohio) region for eks worker only
	EKSRegionUSEast2 EKSRegion = "us-east-2"
	// EKSRegionEUWest1 - eu-west-1 (Ireland) region for eks cluster
	EKSRegionEUWest1 EKSRegion = "eu-west-1"
)

// VPCIDReferencerForEKSCluster is an attribute referencer that resolves VPCID from a referenced VPC
type VPCIDReferencerForEKSCluster struct {
	network.VPCIDReferencer `json:",inline"`
}

// Assign assigns the retrieved vpcId to the managed resource
func (v *VPCIDReferencerForEKSCluster) Assign(res resource.CanReference, value string) error {
	eks, ok := res.(*EKSCluster)
	if !ok {
		return errors.New(errResourceIsNotEKSCluster)
	}

	eks.Spec.VPCID = value
	return nil
}

// IAMRoleARNReferencerForEKSCluster is an attribute referencer that retrieves IAMRoleARN from a referenced IAMRole
type IAMRoleARNReferencerForEKSCluster struct {
	identity.IAMRoleARNReferencer `json:",inline"`
}

// Assign assigns the retrieved value to the managed resource
func (v *IAMRoleARNReferencerForEKSCluster) Assign(res resource.CanReference, value string) error {
	eks, ok := res.(*EKSCluster)
	if !ok {
		return errors.New(errResourceIsNotEKSCluster)
	}

	eks.Spec.RoleARN = value
	return nil
}

// SubnetIDReferencerForEKSCluster is an attribute referencer that resolves SubnetID from a referenced Subnet
type SubnetIDReferencerForEKSCluster struct {
	network.SubnetIDReferencer `json:",inline"`
}

// Assign assigns the retrieved subnetId to the managed resource
func (v *SubnetIDReferencerForEKSCluster) Assign(res resource.CanReference, value string) error {
	eks, ok := res.(*EKSCluster)
	if !ok {
		return errors.New(errResourceIsNotEKSCluster)
	}

	eks.Spec.SubnetIDs = append(eks.Spec.SubnetIDs, value)
	return nil
}

// SecurityGroupIDReferencerForEKSCluster is an attribute referencer that resolves ID from a referenced SecurityGroup
type SecurityGroupIDReferencerForEKSCluster struct {
	network.SecurityGroupIDReferencer `json:",inline"`
}

// Assign assigns the retrieved securityGroupId to the managed resource
func (v *SecurityGroupIDReferencerForEKSCluster) Assign(res resource.CanReference, value string) error {
	eks, ok := res.(*EKSCluster)
	if !ok {
		return errors.New(errResourceIsNotEKSCluster)
	}

	eks.Spec.SecurityGroupIDs = append(eks.Spec.SecurityGroupIDs, value)
	return nil
}

// SecurityGroupIDReferencerForEKSWorkerNodes is an attribute referencer that resolves ID from a referenced SecurityGroup
type SecurityGroupIDReferencerForEKSWorkerNodes struct {
	network.SecurityGroupIDReferencer `json:",inline"`
}

// Assign assigns the retrieved securityGroupId to worker nodes in the managed resource
func (v *SecurityGroupIDReferencerForEKSWorkerNodes) Assign(res resource.CanReference, value string) error {
	eks, ok := res.(*EKSCluster)
	if !ok {
		return errors.New(errResourceIsNotEKSCluster)
	}

	eks.Spec.WorkerNodes.ClusterControlPlaneSecurityGroup = value
	return nil
}

// EKSClusterParameters define the desired state of an AWS Elastic Kubernetes
// Service cluster.
type EKSClusterParameters struct {
	// Configuration of this Spec is dependent on the readme as described here
	// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html

	// Region for this EKS Cluster.
	// +kubebuilder:validation:Enum=us-west-2;us-east-1;eu-west-1
	Region EKSRegion `json:"region"`

	// RoleARN: The Amazon Resource Name (ARN) of the IAM role that provides
	// permis sions for Amazon EKS to make calls to other AWS  API  operations
	// on your behalf. For more information, see 'Amazon EKS Service IAM Role'
	// in the Amazon EKS User Guide.
	RoleARN string `json:"roleARN,omitempty"`

	// RoleARNRef references to an IAMRole to retrieve its ARN
	RoleARNRef *IAMRoleARNReferencerForEKSCluster `json:"roleARNRef,omitempty" resource:"attributereferencer"`

	// The VPC subnets and security groups  used  by  the  cluster  control
	// plane.  Amazon  EKS VPC resources have specific requirements to work
	// properly with Kubernetes. For more information, see Cluster VPC Con-
	// siderations  and Cluster Security Group Considerations in the Amazon
	// EKS User Guide . You must specify at  least  two  subnets.  You  may
	// specify  up  to  5  security groups, but we recommend that you use a
	// dedicated security group for your cluster control plane.

	// VPCID is the ID of the VPC.
	VPCID string `json:"vpcId,omitempty"`

	// VPCIDRef references to a VPC to and retrieves its vpcId
	VPCIDRef *VPCIDReferencerForEKSCluster `json:"vpcIdRef,omitempty" resource:"attributereferencer"`

	// SubnetIDs of this EKS cluster.
	SubnetIDs []string `json:"subnetIds,omitempty"`

	// SubnetIDRefs is a set of referencers that each retrieve the subnetID from the referenced Subnet
	SubnetIDRefs []*SubnetIDReferencerForEKSCluster `json:"subnetIdRefs,omitempty" resource:"attributereferencer"`

	// SecurityGroupIDs of this EKS cluster.
	SecurityGroupIDs []string `json:"securityGroupIds,omitempty"`

	// SecurityGroupIDRefs is a set of referencers that each retrieve the ID from the referenced SecurityGroup
	SecurityGroupIDRefs []*SecurityGroupIDReferencerForEKSCluster `json:"securityGroupIdRefs,omitempty" resource:"attributereferencer"`

	// ClusterVersion: The desired Kubernetes version of this EKS Cluster. If
	// you do not specify a value here, the latest version available is used.
	// +optional
	ClusterVersion string `json:"clusterVersion,omitempty"`

	// WorkerNodes configuration for cloudformation
	WorkerNodes WorkerNodesSpec `json:"workerNodes"`

	// MapRoles map AWS roles to one or more Kubernetes groups. A Default role
	// that allows nodes access to communicate with master is autogenerated when
	// a node pool comes online.
	// +optional
	MapRoles []MapRole `json:"mapRoles,omitempty"`

	// MapUsers map AWS users to one or more Kubernetes groups.
	// +optional
	MapUsers []MapUser `json:"mapUsers,omitempty"`
}

// An EKSClusterSpec defines the desired state of an EKSCluster.
type EKSClusterSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	EKSClusterParameters         `json:",inline"`
}

// MapRole maps an AWS IAM role to one or more Kubernetes groups. See
// https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html and
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md
type MapRole struct {
	// RoleARN to match, e.g. 'arn:aws:iam::000000000000:role/KubernetesNode'.
	RoleARN string `json:"rolearn"`

	// Username (in Kubernetes) the RoleARN should map to.
	Username string `json:"username"`

	// Groups (in Kubernetes) the RoleARN should map to.
	Groups []string `json:"groups"`
}

// MapUser maps an AWS IAM user to one or more Kubernetes groups. See
// https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html and
// https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/master/README.md
type MapUser struct {
	// UserARN to match, e.g. 'arn:aws:iam::000000000000:user/Alice'
	UserARN string `json:"userarn"`

	// Username (in Kubernetes) the UserARN should map to.
	Username string `json:"username"`

	// Groups (in Kubernetes) the UserARN should map to.
	Groups []string `json:"groups"`
}

//WorkerNodesSpec - Worker node spec used to define cloudformation template that provisions workers for cluster
type WorkerNodesSpec struct {
	// KeyName of the EC2 Key Pair to allow SSH access to the EC2 instances.
	// +optional
	KeyName string `json:"keyName,omitempty"`

	// NodeImageId that the EC2 instances should run. Defaults to the region's
	// standard AMI.
	// +optional
	NodeImageID string `json:"nodeImageId,omitempty"`

	// NodeInstanceType of the EC2 instances.
	// +kubebuilder:validation:Enum=t2.small;t2.medium;t2.large;t2.xlarge;t2.2xlarge;t3.nano;t3.micro;t3.small;t3.medium;t3.large;t3.xlarge;t3.2xlarge;m3.medium;m3.large;m3.xlarge;m3.2xlarge;m4.large;m4.xlarge;m4.2xlarge;m4.4xlarge;m4.10xlarge;m5.large;m5.xlarge;m5.2xlarge;m5.4xlarge;m5.12xlarge;m5.24xlarge;c4.large;c4.xlarge;c4.2xlarge;c4.4xlarge;c4.8xlarge;c5.large;c5.xlarge;c5.2xlarge;c5.4xlarge;c5.9xlarge;c5.18xlarge;i3.large;i3.xlarge;i3.2xlarge;i3.4xlarge;i3.8xlarge;i3.16xlarge;r3.xlarge;r3.2xlarge;r3.4xlarge;r3.8xlarge;r4.large;r4.xlarge;r4.2xlarge;r4.4xlarge;r4.8xlarge;r4.16xlarge;x1.16xlarge;x1.32xlarge;p2.xlarge;p2.8xlarge;p2.16xlarge;p3.2xlarge;p3.8xlarge;p3.16xlarge;r5.large;r5.xlarge;r5.2xlarge;r5.4xlarge;r5.12xlarge;r5.24xlarge;r5d.large;r5d.xlarge;r5d.2xlarge;r5d.4xlarge;r5d.12xlarge;r5d.24xlarge;z1d.large;z1d.xlarge;z1d.2xlarge;z1d.3xlarge;z1d.6xlarge;z1d.12xlarge
	NodeInstanceType string `json:"nodeInstanceType"`

	// NodeAutoScalingGroupMinSize configures the minimum size of this node
	// group's Autoscaling Group. Defaults to 1.
	// +optional
	NodeAutoScalingGroupMinSize *int `json:"nodeAutoScalingGroupMinSize,omitempty"`

	// NodeAutoScalingGroupMaxSize configures the maximum size of this node
	// group's Autoscaling Group. Defaults to 3.
	// +optional
	NodeAutoScalingGroupMaxSize *int `json:"nodeAutoScalingGroupMaxSize,omitempty"`

	// NodeVolumeSize configures the volume size in GB. Defaults to 20.
	// +optional
	NodeVolumeSize *int `json:"nodeVolumeSize,omitempty"`

	// BootstrapArguments to pass to the bootstrap script. See
	// files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami
	// +optional
	BootstrapArguments string `json:"bootstrapArguments,omitempty"`

	// NodeGroupName is a unique identifier for the Node Group.
	// +optional
	NodeGroupName string `json:"nodeGroupName,omitempty"`

	// ClusterControlPlaneSecurityGroup configures the security group of the
	// cluster control plane in order to allow communication to this node group.
	// +optional
	ClusterControlPlaneSecurityGroup string `json:"clusterControlPlaneSecurityGroup,omitempty"`

	// ClusterControlPlaneSecurityGroupRef references to a SecurityGroup to retrieve its ID
	ClusterControlPlaneSecurityGroupRef *SecurityGroupIDReferencerForEKSWorkerNodes `json:"clusterControlPlaneSecurityGroupRef,omitempty" resource:"attributereferencer"`
}

// An EKSClusterStatus represents the observed state of an EKSCluster.
type EKSClusterStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`

	// State of the cluster.
	State string `json:"state,omitempty"`

	// ClusterName of the cluster.
	ClusterName string `json:"resourceName,omitempty"`

	// ClusterVersion of the cluster.
	ClusterVersion string `json:"resourceVersion,omitempty"`

	// Endpoint for connecting to the cluster.
	Endpoint string `json:"endpoint,omitempty"`

	// CloudFormationStackID of the Stack used to create node groups.
	CloudFormationStackID string `json:"cloudformationStackId,omitempty"`
}

// +kubebuilder:object:root=true

// An EKSCluster is a managed resource that represents an AWS Elastic Kubernetes
// Service cluster.
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLUSTER-NAME",type="string",JSONPath=".status.clusterName"
// +kubebuilder:printcolumn:name="ENDPOINT",type="string",JSONPath=".status.endpoint"
// +kubebuilder:printcolumn:name="CLUSTER-CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="LOCATION",type="string",JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type EKSCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EKSClusterSpec   `json:"spec,omitempty"`
	Status EKSClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EKSClusterList contains a list of EKSCluster items
type EKSClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EKSCluster `json:"items"`
}

// An EKSClusterClassSpecTemplate is a template for the spec of a dynamically
// provisioned EKSCluster.
type EKSClusterClassSpecTemplate struct {
	runtimev1alpha1.NonPortableClassSpecTemplate `json:",inline"`
	EKSClusterParameters                         `json:",inline"`
}

// +kubebuilder:object:root=true

// An EKSClusterClass is a non-portable resource class. It defines the desired
// spec of resource claims that use it to dynamically provision a managed
// resource.
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".specTemplate.providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".specTemplate.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type EKSClusterClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SpecTemplate is a template for the spec of a dynamically provisioned
	// EKSCluster.
	SpecTemplate EKSClusterClassSpecTemplate `json:"specTemplate"`
}

// +kubebuilder:object:root=true

// EKSClusterClassList contains a list of cloud memorystore resource classes.
type EKSClusterClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EKSClusterClass `json:"items"`
}
