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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

const (
	mockName      = "mockName"
	mockNamespace = "mockNamespace"
	mockVPCID     = "mockVPCID"
)

var (
	errBoom = errors.New("boom")
)

type mockCanReference struct {
	resource.CanReference
	ns string
}

func (c *mockCanReference) GetNamespace() string {
	return c.ns
}

type mockReader struct {
	client.Reader
	readFn func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
}

func (m *mockReader) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return m.readFn(ctx, key, obj)
}

func TestVPCIDReferencerGetStatus(t *testing.T) {

	errResourceNotFound := &kerrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}

	readyResource := VPC{
		Status: VPCStatus{
			VPCExternalStatus: VPCExternalStatus{
				VPCID: mockVPCID,
			},
		},
	}

	readyResource.Status.SetConditions(runtimev1alpha1.Available())

	type input struct {
		readerFn func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
	}
	type expected struct {
		statuses []resource.ReferenceStatus
		err      error
	}
	for name, tc := range map[string]struct {
		input    input
		expected expected
	}{
		"ReaderError_ReturnsError": {
			input: input{
				readerFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errBoom
				},
			},
			expected: expected{
				err: errBoom,
			},
		},
		"ReaderNotFoundError_ReturnsExpected": {
			input: input{
				readerFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errResourceNotFound
				},
			},
			expected: expected{
				statuses: []resource.ReferenceStatus{{Name: mockName, Status: resource.ReferenceNotFound}},
			},
		},
		"ReferenceNotReady_ReturnsExpected": {
			input: input{
				readerFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return nil
				},
			},
			expected: expected{
				statuses: []resource.ReferenceStatus{{Name: mockName, Status: resource.ReferenceNotReady}},
			},
		},
		"ReferenceReady_ReturnsExpected": {
			input: input{
				readerFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					p := obj.(*VPC)
					p.Status = readyResource.Status
					return nil
				},
			},
			expected: expected{
				statuses: []resource.ReferenceStatus{{Name: mockName, Status: resource.ReferenceReady}},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := VPCIDReferencer{LocalObjectReference: corev1.LocalObjectReference{Name: mockName}}

			canReference := &mockCanReference{ns: mockNamespace}
			reader := &mockReader{readFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				if diff := cmp.Diff(key, client.ObjectKey{Name: mockName, Namespace: mockNamespace}); diff != "" {
					t.Errorf("reader.Get(...): -expected key, +got key:\n%s", diff)
				}
				return tc.input.readerFn(ctx, key, obj)
			}}

			statuses, err := r.GetStatus(context.Background(), canReference, reader)
			if diff := cmp.Diff(tc.expected.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("GetStatus(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.expected.statuses, statuses); diff != "" {
				t.Errorf("GetStatus(...): -want statuses, +got statuses:\n%s", diff)
			}
		})
	}
}

func TestVPCIDReferencerBuild(t *testing.T) {
	type input struct {
		readerFn func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
	}
	type expected struct {
		value string
		err   error
	}
	for name, tc := range map[string]struct {
		input    input
		expected expected
	}{
		"ReaderError_ReturnsError": {
			input: input{
				readerFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errBoom
				},
			},
			expected: expected{
				err: errBoom,
			},
		},
		"ReferenceRetrieved_ReturnsExpected": {
			input: input{
				readerFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					p := obj.(*VPC)
					p.Status.VPCID = mockVPCID
					return nil
				},
			},
			expected: expected{
				value: mockVPCID,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := VPCIDReferencer{LocalObjectReference: corev1.LocalObjectReference{Name: mockName}}

			canReference := &mockCanReference{ns: mockNamespace}
			reader := &mockReader{readFn: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				if diff := cmp.Diff(key, client.ObjectKey{Name: mockName, Namespace: mockNamespace}); diff != "" {
					t.Errorf("reader.Get(...): -expected key, +got key:\n%s", diff)
				}
				return tc.input.readerFn(ctx, key, obj)
			}}

			value, err := r.Build(context.Background(), canReference, reader)
			if diff := cmp.Diff(tc.expected.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Build(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.expected.value, value); diff != "" {
				t.Errorf("Build(...): -want value, +got value:\n%s", diff)
			}
		})
	}
}
