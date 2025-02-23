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

package compute

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"

	awscomputev1alpha2 "github.com/crossplaneio/stack-aws/apis/compute/v1alpha2"
	awsv1alpha2 "github.com/crossplaneio/stack-aws/apis/v1alpha2"
	aws "github.com/crossplaneio/stack-aws/pkg/clients"
	cloudformationclient "github.com/crossplaneio/stack-aws/pkg/clients/cloudformation"
	"github.com/crossplaneio/stack-aws/pkg/clients/eks"
)

const (
	controllerName    = "eks.compute.aws.crossplane.io"
	finalizer         = "finalizer." + controllerName
	clusterNamePrefix = "eks-"

	eksAuthConfigMapName = "aws-auth"
	eksAuthMapRolesKey   = "mapRoles"
	eksAuthMapUsersKey   = "mapUsers"
)

var (
	log = logging.Logger.WithName("controller." + controllerName)
	ctx = context.Background()
)

// Amounts of time we wait before requeuing a reconcile.
const (
	aShortWait = 30 * time.Second
	aLongWait  = 60 * time.Second
)

// Error strings
const (
	errUpdateManagedStatus = "cannot update managed resource status"
)

// CloudFormation States that are non-transitory
var (
	completedCFState = map[cf.StackStatus]bool{
		cf.StackStatusCreateComplete: true,
		cf.StackStatusUpdateComplete: true,
	}

	failedCFState = map[cf.StackStatus]bool{
		cf.StackStatusCreateFailed:     true,
		cf.StackStatusRollbackComplete: true,
		cf.StackStatusRollbackFailed:   true,
		cf.StackStatusDeleteComplete:   true,
		cf.StackStatusDeleteFailed:     true,
	}
)

// Reconciler reconciles a Provider object
type Reconciler struct {
	client.Client
	publisher resource.ManagedConnectionPublisher
	resource.ManagedReferenceResolver

	connect func(*awscomputev1alpha2.EKSCluster) (eks.Client, error)
	create  func(*awscomputev1alpha2.EKSCluster, eks.Client) (reconcile.Result, error)
	sync    func(*awscomputev1alpha2.EKSCluster, eks.Client) (reconcile.Result, error)
	delete  func(*awscomputev1alpha2.EKSCluster, eks.Client) (reconcile.Result, error)
	secret  func(*eks.Cluster, *awscomputev1alpha2.EKSCluster, eks.Client) error
	awsauth func(*eks.Cluster, *awscomputev1alpha2.EKSCluster, eks.Client, string) error
}

// EKSClusterController is responsible for adding the EKSCluster
// controller and its corresponding reconciler to the manager with any runtime configuration.
type EKSClusterController struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *EKSClusterController) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		Client:                   mgr.GetClient(),
		publisher:                resource.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme()),
		ManagedReferenceResolver: resource.NewAPIManagedReferenceResolver(mgr.GetClient()),
	}
	r.connect = r._connect
	r.create = r._create
	r.sync = r._sync
	r.delete = r._delete
	r.secret = r._secret
	r.awsauth = r._awsauth

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&awscomputev1alpha2.EKSCluster{}).
		Complete(r)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *awscomputev1alpha2.EKSCluster, err error) (reconcile.Result, error) {
	instance.Status.SetConditions(runtimev1alpha1.ReconcileError(err))

	// If this is the first time we've encountered this error we'll be requeued
	// implicitly due to the status update. Otherwise we requeue after a short
	// wait in case the error condition was resolved.
	return reconcile.Result{RequeueAfter: aShortWait}, r.Update(ctx, instance)
}

func (r *Reconciler) _connect(instance *awscomputev1alpha2.EKSCluster) (eks.Client, error) {
	p := &awsv1alpha2.Provider{}
	if err := r.Get(ctx, meta.NamespacedNameOf(instance.Spec.ProviderReference), p); err != nil {
		return nil, err
	}

	s := &v1.Secret{}
	n := types.NamespacedName{Namespace: p.GetNamespace(), Name: p.Spec.Secret.Name}
	if err := r.Get(ctx, n, s); err != nil {
		return nil, err
	}

	// NOTE(negz): EKS clusters must specify a region for creation. They never
	// use the provider's region. This should be addressed per the below issue.
	// https://github.com/crossplaneio/stack-aws/issues/38
	config, err := aws.LoadConfig(s.Data[p.Spec.Secret.Key], aws.DefaultSection, string(instance.Spec.Region))
	if err != nil {
		return nil, err
	}

	// Create new EKS Client
	return eks.NewClient(config), nil
}

func (r *Reconciler) _create(instance *awscomputev1alpha2.EKSCluster, client eks.Client) (reconcile.Result, error) {
	instance.Status.SetConditions(runtimev1alpha1.Creating())
	clusterName := fmt.Sprintf("%s%s", clusterNamePrefix, instance.UID)

	// Create Master
	createdCluster, err := client.Create(clusterName, instance.Spec)
	if err != nil && !eks.IsErrorAlreadyExists(err) {
		if eks.IsErrorBadRequest(err) {
			// If this was the first time we encountered this error we'll be
			// requeued implicitly. Otherwise there's no point requeuing, since
			// the error indicates our spec is bad and needs updating before
			// anything will work.
			instance.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
			return reconcile.Result{}, r.Update(ctx, instance)
		}
		return r.fail(instance, err)
	}

	// we will need to set State.ClusterVersion it. this is needed to retrieve
	// the right ami image for the worker nodes
	instance.Status.ClusterVersion = createdCluster.Version

	// Update status
	instance.Status.State = awscomputev1alpha2.ClusterStatusCreating
	instance.Status.ClusterName = clusterName

	instance.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

	// We'll be requeued immediately the first time we update our status
	// condition. Otherwise we want to requeue after a short wait in order to
	// determine whether the cluster is ready.
	return reconcile.Result{RequeueAfter: aShortWait}, r.Update(ctx, instance)
}

// generateAWSAuthConfigMap generates the configmap for configure auth
func generateAWSAuthConfigMap(instance *awscomputev1alpha2.EKSCluster, workerARN string) (*v1.ConfigMap, error) {
	data := map[string]string{}
	defaultRole := awscomputev1alpha2.MapRole{
		RoleARN:  workerARN,
		Username: "system:node:{{EC2PrivateDNSName}}",
		Groups:   []string{"system:bootstrappers", "system:nodes"},
	}

	// Serialize mapRoles
	roles := make([]awscomputev1alpha2.MapRole, len(instance.Spec.MapRoles))
	copy(roles, instance.Spec.MapRoles)
	roles = append(roles, defaultRole)

	rolesMarshalled, err := yaml.Marshal(roles)
	if err != nil {
		return nil, err
	}

	data[eksAuthMapRolesKey] = string(rolesMarshalled)

	// Serialize mapUsers
	if len(instance.Spec.MapUsers) > 0 {
		usersMarshalled, err := yaml.Marshal(instance.Spec.MapUsers)
		if err != nil {
			return nil, err
		}
		data[eksAuthMapUsersKey] = string(usersMarshalled)
	}

	name := eksAuthConfigMapName
	namespace := "kube-system"
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	return &cm, nil
}

// _awsauth generates an aws-auth configmap and pushes it to the remote eks cluster to configure auth
func (r *Reconciler) _awsauth(cluster *eks.Cluster, instance *awscomputev1alpha2.EKSCluster, client eks.Client, workerARN string) error {
	cm, err := generateAWSAuthConfigMap(instance, workerARN)
	if err != nil {
		return err
	}

	// Sync aws-auth to remote eks cluster to configure it's auth.
	token, err := client.ConnectionToken(instance.Status.ClusterName)
	if err != nil {
		return err
	}

	// Client to eks cluster
	caData, err := base64.StdEncoding.DecodeString(cluster.CA)
	if err != nil {
		return err
	}

	c := rest.Config{
		Host: cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caData,
		},
		BearerToken: token,
	}

	clientset, err := kubernetes.NewForConfig(&c)
	if err != nil {
		return err
	}

	// Create or update aws-auth configmap on eks cluster
	_, err = clientset.CoreV1().ConfigMaps(cm.Namespace).Create(cm)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err = clientset.CoreV1().ConfigMaps(cm.Namespace).Update(cm)
		}
	}

	return err
}

func (r *Reconciler) _sync(instance *awscomputev1alpha2.EKSCluster, client eks.Client) (reconcile.Result, error) {
	cluster, err := client.Get(instance.Status.ClusterName)
	if err != nil {
		return r.fail(instance, err)
	}

	if cluster.Status != awscomputev1alpha2.ClusterStatusActive {
		instance.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

		// Requeue after a short wait to see if the cluster has become ready.
		return reconcile.Result{RequeueAfter: aShortWait}, nil
	}

	// Create workers
	if instance.Status.CloudFormationStackID == "" {
		clusterWorkers, err := client.CreateWorkerNodes(instance.Status.ClusterName, instance.Status.ClusterVersion, instance.Spec)
		if err != nil {
			return r.fail(instance, err)
		}
		instance.Status.CloudFormationStackID = clusterWorkers.WorkerStackID
		instance.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

		// We'll likely be requeued implicitly due to the status update, but
		// otherwise we want to requeue a reconcile after a short wait to check
		// on the worker node creation.
		return reconcile.Result{RequeueAfter: aShortWait}, r.Update(ctx, instance)
	}

	clusterWorker, err := client.GetWorkerNodes(instance.Status.CloudFormationStackID)
	if err != nil {
		return r.fail(instance, err)
	}

	if failedCFState[clusterWorker.WorkersStatus] {
		return r.fail(instance, fmt.Errorf("clusterworker stack failed with status %q and reason %q", clusterWorker.WorkersStatus, clusterWorker.WorkerReason))
	}

	if !completedCFState[clusterWorker.WorkersStatus] {
		instance.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

		return reconcile.Result{RequeueAfter: aShortWait}, r.Update(ctx, instance)
	}

	if err := r.awsauth(cluster, instance, client, clusterWorker.WorkerARN); err != nil {
		return r.fail(instance, errors.Wrap(err, "failed to set auth map on eks"))
	}

	if err := r.secret(cluster, instance, client); err != nil {
		return r.fail(instance, err)
	}

	// update resource status
	instance.Status.Endpoint = cluster.Endpoint
	instance.Status.State = awscomputev1alpha2.ClusterStatusActive
	instance.Status.SetConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess())
	resource.SetBindable(instance)

	// Our cluster is available. Requeue speculative yafter a long wait in case
	// the cluster has changed.
	return reconcile.Result{RequeueAfter: aLongWait}, r.Update(ctx, instance)
}

func (r *Reconciler) _secret(cluster *eks.Cluster, instance *awscomputev1alpha2.EKSCluster, client eks.Client) error {
	token, err := client.ConnectionToken(instance.Status.ClusterName)
	if err != nil {
		return err
	}

	// Avoid double base64 encoding on secret
	caData, err := base64.StdEncoding.DecodeString(cluster.CA)
	if err != nil {
		return err
	}

	return r.publisher.PublishConnection(ctx, instance, resource.ConnectionDetails{
		runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(cluster.Endpoint),
		runtimev1alpha1.ResourceCredentialsSecretCAKey:       caData,
		runtimev1alpha1.ResourceCredentialsTokenKey:          []byte(token),
	})
}

// _delete check reclaim policy and if needed delete the eks cluster resource
func (r *Reconciler) _delete(instance *awscomputev1alpha2.EKSCluster, client eks.Client) (reconcile.Result, error) {
	instance.Status.SetConditions(runtimev1alpha1.Deleting())
	if instance.Spec.ReclaimPolicy == runtimev1alpha1.ReclaimDelete {
		var deleteErrors []string
		if err := client.Delete(instance.Status.ClusterName); err != nil && !eks.IsErrorNotFound(err) {
			deleteErrors = append(deleteErrors, fmt.Sprintf("Master Delete Error: %s", err.Error()))
		}

		if instance.Status.CloudFormationStackID != "" {
			if err := client.DeleteWorkerNodes(instance.Status.CloudFormationStackID); err != nil && !cloudformationclient.IsErrorNotFound(err) {
				deleteErrors = append(deleteErrors, fmt.Sprintf("Worker Delete Error: %s", err.Error()))
			}
		}

		if len(deleteErrors) > 0 {
			return r.fail(instance, errors.New(strings.Join(deleteErrors, ", ")))
		}
	}

	meta.RemoveFinalizer(instance, finalizer)
	instance.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

	// No need to requeue a reconcile if we've successfully asked for the
	// cluster to be deleted.
	return reconcile.Result{Requeue: false}, r.Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", awscomputev1alpha2.EKSClusterKindAPIVersion, "request", request)
	// Fetch the Provider instance
	instance := &awscomputev1alpha2.EKSCluster{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		// No need to requeue if the resource no longer exists, otherwise we'll
		// be requeued because we return an error.
		return reconcile.Result{}, resource.IgnoreNotFound(err)
	}

	// Create EKS Client
	eksClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, err)
	}

	if !resource.IsConditionTrue(instance.GetCondition(runtimev1alpha1.TypeReferencesResolved)) {
		if err := r.ResolveReferences(ctx, instance); err != nil {
			condition := runtimev1alpha1.ReconcileError(err)
			if resource.IsReferencesAccessError(err) {
				condition = runtimev1alpha1.ReferenceResolutionBlocked(err)
			}

			instance.Status.SetConditions(condition)
			return reconcile.Result{RequeueAfter: aLongWait}, errors.Wrap(r.Update(ctx, instance), errUpdateManagedStatus)
		}

		// Add ReferenceResolutionSuccess to the conditions
		instance.Status.SetConditions(runtimev1alpha1.ReferenceResolutionSuccess())
	}

	// Add finalizer
	meta.AddFinalizer(instance, finalizer)

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		return r.delete(instance, eksClient)
	}

	// Create cluster instance
	if instance.Status.ClusterName == "" {
		return r.create(instance, eksClient)
	}

	// Sync cluster instance status with cluster status
	return r.sync(instance, eksClient)
}
