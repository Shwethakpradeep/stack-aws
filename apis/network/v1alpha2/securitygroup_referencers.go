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
	"context"

	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecurityGroupIDReferencer is used to get the ID from another SecurityGroup
type SecurityGroupIDReferencer struct {
	corev1.LocalObjectReference `json:",inline"`
}

// GetStatus implements GetStatus method of AttributeReferencer interface
func (v *SecurityGroupIDReferencer) GetStatus(ctx context.Context, res resource.CanReference, reader client.Reader) ([]resource.ReferenceStatus, error) {
	sg := SecurityGroup{}

	nn := types.NamespacedName{Name: v.Name, Namespace: res.GetNamespace()}
	if err := reader.Get(ctx, nn, &sg); err != nil {
		if kerrors.IsNotFound(err) {
			return []resource.ReferenceStatus{{Name: v.Name, Status: resource.ReferenceNotFound}}, nil
		}

		return nil, err
	}

	if !resource.IsConditionTrue(sg.GetCondition(runtimev1alpha1.TypeReady)) {
		return []resource.ReferenceStatus{{Name: v.Name, Status: resource.ReferenceNotReady}}, nil
	}

	return []resource.ReferenceStatus{{Name: v.Name, Status: resource.ReferenceReady}}, nil
}

// Build retrieves and builds the SubnetID
func (v *SecurityGroupIDReferencer) Build(ctx context.Context, res resource.CanReference, reader client.Reader) (string, error) {
	sg := SecurityGroup{}
	nn := types.NamespacedName{Name: v.Name, Namespace: res.GetNamespace()}
	if err := reader.Get(ctx, nn, &sg); err != nil {
		return "", err
	}

	return sg.Status.SecurityGroupID, nil
}
