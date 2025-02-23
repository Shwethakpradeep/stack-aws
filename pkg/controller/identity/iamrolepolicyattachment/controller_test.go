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

package iamrolepolicyattachment

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"

	v1alpha2 "github.com/crossplaneio/stack-aws/apis/identity/v1alpha2"
	"github.com/crossplaneio/stack-aws/pkg/clients/iam"
	"github.com/crossplaneio/stack-aws/pkg/clients/iam/fake"
)

var (
	mockExternalClient external
	mockClient         fake.MockRolePolicyAttachmentClient

	// an arbitrary managed resource
	unexpecedItem resource.Managed
)

func TestMain(m *testing.M) {

	mockClient = fake.MockRolePolicyAttachmentClient{}
	mockExternalClient = external{&mockClient}

	os.Exit(m.Run())
}

func Test_Connect(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := &v1alpha2.IAMRolePolicyAttachment{}
	var clientErr error
	var configErr error

	conn := connector{
		client: nil,
		newClientFn: func(conf *aws.Config) (iam.RolePolicyAttachmentClient, error) {
			return &mockClient, clientErr
		},
		awsConfigFn: func(context.Context, client.Reader, *corev1.ObjectReference) (*aws.Config, error) {
			return &aws.Config{}, configErr
		},
	}

	for _, tc := range []struct {
		description       string
		managedObj        resource.Managed
		configErr         error
		clientErr         error
		expectedClientNil bool
		expectedErrNil    bool
	}{
		{
			"valid input should return expected",
			mockManaged,
			nil,
			nil,
			false,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			true,
			false,
		},
		{
			"if aws config provider fails, should return error",
			mockManaged,
			errors.New("some error"),
			nil,
			true,
			false,
		},
		{
			"if aws client provider fails, should return error",
			mockManaged, // an arbitrary managed resource which is not expected
			nil,
			errors.New("some error"),
			true,
			false,
		},
	} {
		clientErr = tc.clientErr
		configErr = tc.configErr

		res, err := conn.Connect(context.Background(), tc.managedObj)
		g.Expect(res == nil).To(gomega.Equal(tc.expectedClientNil), tc.description)
		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
	}
}

func Test_Observe(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha2.IAMRolePolicyAttachment{
		Spec: v1alpha2.IAMRolePolicyAttachmentSpec{
			IAMRolePolicyAttachmentParameters: v1alpha2.IAMRolePolicyAttachmentParameters{
				PolicyARN: "some arbitrary arn",
			},
		},
	}
	mockExternal := awsiam.AttachedPolicy{
		PolicyArn: aws.String("some arbitrary arn"),
	}
	var mockClientErr error
	var itemsList []awsiam.AttachedPolicy
	mockClient.MockListAttachedRolePoliciesRequest = func(input *awsiam.ListAttachedRolePoliciesInput) awsiam.ListAttachedRolePoliciesRequest {
		return awsiam.ListAttachedRolePoliciesRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data: &awsiam.ListAttachedRolePoliciesOutput{
					AttachedPolicies: itemsList,
				},
				Error: mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description           string
		managedObj            resource.Managed
		returnedList          []awsiam.AttachedPolicy
		clientErr             error
		expectedErrNil        bool
		expectedResourceExist bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			[]awsiam.AttachedPolicy{mockExternal},
			nil,
			true,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			false,
			false,
		},
		{
			"if external resource doesn't exist, it should return expected",
			mockManaged.DeepCopy(),
			[]awsiam.AttachedPolicy{},
			nil,
			true,
			false,
		},
		{
			"if external resource fails, it should return error",
			mockManaged.DeepCopy(),
			[]awsiam.AttachedPolicy{mockExternal},
			errors.New("some error"),
			false,
			false,
		},
	} {
		mockClientErr = tc.clientErr
		itemsList = tc.returnedList

		result, err := mockExternalClient.Observe(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		g.Expect(result.ResourceExists).To(gomega.Equal(tc.expectedResourceExist), tc.description)
		if tc.expectedResourceExist {
			mgd := tc.managedObj.(*v1alpha2.IAMRolePolicyAttachment)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionTrue), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonAvailable), tc.description)
			g.Expect(mgd.Status.AttachedPolicyARN).To(gomega.Equal(aws.StringValue(mockExternal.PolicyArn)), tc.description)
		}
	}
}

func Test_Create(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha2.IAMRolePolicyAttachment{
		Spec: v1alpha2.IAMRolePolicyAttachmentSpec{
			IAMRolePolicyAttachmentParameters: v1alpha2.IAMRolePolicyAttachmentParameters{
				PolicyARN: "some arbitrary arn",
			},
		},
	}

	var mockClientErr error
	mockClient.MockAttachRolePolicyRequest = func(input *awsiam.AttachRolePolicyInput) awsiam.AttachRolePolicyRequest {
		g.Expect(aws.StringValue(input.RoleName)).To(gomega.Equal(mockManaged.Spec.RoleName), "the passed parameters are not valid")
		g.Expect(aws.StringValue(input.PolicyArn)).To(gomega.Equal(mockManaged.Spec.PolicyARN), "the passed parameters are not valid")
		return awsiam.AttachRolePolicyRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsiam.AttachRolePolicyOutput{},
				Error:       mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description    string
		managedObj     resource.Managed
		clientErr      error
		expectedErrNil bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			false,
		},
		{
			"if attaching resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			false,
		},
	} {
		mockClientErr = tc.clientErr

		_, err := mockExternalClient.Create(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha2.IAMRolePolicyAttachment)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonCreating), tc.description)
		}
	}
}

func Test_Update(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha2.IAMRolePolicyAttachment{
		Spec: v1alpha2.IAMRolePolicyAttachmentSpec{
			IAMRolePolicyAttachmentParameters: v1alpha2.IAMRolePolicyAttachmentParameters{
				PolicyARN: "some arbitrary arn",
			},
		},
		Status: v1alpha2.IAMRolePolicyAttachmentStatus{
			IAMRolePolicyAttachmentExternalStatus: v1alpha2.IAMRolePolicyAttachmentExternalStatus{
				AttachedPolicyARN: "another arbitrary arn",
			},
		},
	}

	var mockClientAttachErr error
	var attachIsCalled bool
	mockClient.MockAttachRolePolicyRequest = func(input *awsiam.AttachRolePolicyInput) awsiam.AttachRolePolicyRequest {
		attachIsCalled = true

		g.Expect(aws.StringValue(input.RoleName)).To(gomega.Equal(mockManaged.Spec.RoleName), "the passed parameters are not valid")
		g.Expect(aws.StringValue(input.PolicyArn)).To(gomega.Equal(mockManaged.Spec.PolicyARN), "the passed parameters are not valid")
		return awsiam.AttachRolePolicyRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsiam.AttachRolePolicyOutput{},
				Error:       mockClientAttachErr,
			},
		}
	}

	var mockClientDetachErr error
	var detachIsCalled bool
	mockClient.MockDetachRolePolicyRequest = func(input *awsiam.DetachRolePolicyInput) awsiam.DetachRolePolicyRequest {
		detachIsCalled = true

		g.Expect(aws.StringValue(input.RoleName)).To(gomega.Equal(mockManaged.Spec.RoleName), "the passed parameters are not valid")
		g.Expect(aws.StringValue(input.PolicyArn)).To(gomega.Equal(mockManaged.Status.AttachedPolicyARN), "the passed parameters are not valid")
		return awsiam.DetachRolePolicyRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsiam.DetachRolePolicyOutput{},
				Error:       mockClientDetachErr,
			},
		}
	}

	for _, tc := range []struct {
		description         string
		managedObj          resource.Managed
		clientAttachErr     error
		clientDetachErr     error
		expectedAttachCcall bool
		expectedDetachCcall bool
		expectedErrNil      bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			nil,
			true,
			true,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			false,
			false,
			false,
		},
		{
			"if status has no policy attached, return expected",
			&v1alpha2.IAMRolePolicyAttachment{
				Spec: v1alpha2.IAMRolePolicyAttachmentSpec{
					IAMRolePolicyAttachmentParameters: v1alpha2.IAMRolePolicyAttachmentParameters{
						PolicyARN: "some arbitrary arn",
					},
				},
			},
			nil,
			nil,
			false,
			false,
			true,
		},
		{
			"if status policy matches spec policy, return expected",
			&v1alpha2.IAMRolePolicyAttachment{
				Spec: v1alpha2.IAMRolePolicyAttachmentSpec{
					IAMRolePolicyAttachmentParameters: v1alpha2.IAMRolePolicyAttachmentParameters{
						PolicyARN: "some arbitrary arn",
					},
				},
				Status: v1alpha2.IAMRolePolicyAttachmentStatus{
					IAMRolePolicyAttachmentExternalStatus: v1alpha2.IAMRolePolicyAttachmentExternalStatus{
						AttachedPolicyARN: "some arbitrary arn",
					},
				},
			},
			nil,
			nil,
			false,
			false,
			true,
		}, {
			"if attaching resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			nil,
			true,
			false,
			false,
		},
		{
			"if detaching resource fails, it should return error",
			mockManaged.DeepCopy(),
			nil,
			errors.New("some error"),
			true,
			true,
			false,
		},
	} {
		attachIsCalled = false
		detachIsCalled = false

		mockClientAttachErr = tc.clientAttachErr
		mockClientDetachErr = tc.clientDetachErr

		_, err := mockExternalClient.Update(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		g.Expect(attachIsCalled).To(gomega.Equal(tc.expectedAttachCcall), tc.description)
		g.Expect(detachIsCalled).To(gomega.Equal(tc.expectedDetachCcall), tc.description)
	}
}

func Test_Delete(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha2.IAMRolePolicyAttachment{
		Spec: v1alpha2.IAMRolePolicyAttachmentSpec{
			IAMRolePolicyAttachmentParameters: v1alpha2.IAMRolePolicyAttachmentParameters{
				PolicyARN: "some arbitrary arn",
			},
		},
	}
	var mockClientErr error
	mockClient.MockDetachRolePolicyRequest = func(input *awsiam.DetachRolePolicyInput) awsiam.DetachRolePolicyRequest {
		g.Expect(aws.StringValue(input.RoleName)).To(gomega.Equal(mockManaged.Spec.RoleName), "the passed parameters are not valid")
		g.Expect(aws.StringValue(input.PolicyArn)).To(gomega.Equal(mockManaged.Spec.PolicyARN), "the passed parameters are not valid")
		return awsiam.DetachRolePolicyRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsiam.DetachRolePolicyOutput{},
				Error:       mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description    string
		managedObj     resource.Managed
		clientErr      error
		expectedErrNil bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			false,
		},
		{
			"if the resource doesn't exist deleting resource should not return an error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			false,
		},
		{
			"if the resource doesn't exist deleting resource should not return an error",
			mockManaged.DeepCopy(),
			awserr.New(awsiam.ErrCodeNoSuchEntityException, "", nil),
			true,
		},
		{
			"if deleting resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			false,
		},
	} {
		mockClientErr = tc.clientErr

		err := mockExternalClient.Delete(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha2.IAMRolePolicyAttachment)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonDeleting), tc.description)
		}
	}
}
