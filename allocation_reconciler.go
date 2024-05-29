/*
Copyright (c) 2024 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in
compliance with the License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is
distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied. See the License for the specific language governing permissions and limitations under the
License.
*/

package main

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	pluginapi "github.com/openshift-kni/oran-o2ims/api/hardwaremanagement/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	clnt "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AllocationReconciler contains the data and logic needed to reconcile request to allocate nodes.
type AllocationReconciler struct {
	logger logr.Logger
	client clnt.Client
}

// Reconcile is the method that will be called by the controller runtime library when a request
// to allocate a node has been created, updated or deleted.
func (r *AllocationReconciler) Reconcile(ctx context.Context,
	request ctrl.Request) (result ctrl.Result, err error) {
	// Fetch the object object:
	object := &pluginapi.NodeAllocationRequest{}
	err = r.client.Get(ctx, request.NamespacedName, object)
	if apierrors.IsNotFound(err) {
		r.logger.Info(
			"Object no longer exists",
			"namespace", request.Namespace,
			"name", request.Name,
		)
		err = nil
		return
	}
	if err != nil {
		r.logger.Error(err, "Failed to get object")
		return
	}

	// Check if the object is being deleted and if it has our finalizer:
	deleting := !object.DeletionTimestamp.IsZero()
	finalizer := controllerutil.ContainsFinalizer(object, finalizerName)

	// Make a copy of the object so that we can modify it during our processing, and calculate
	// the changes from the original to make a patch when we are done.
	copy := object.DeepCopy()

	// If the object isn't being deleted and doesn't have our finalizeer then we need to add
	// the finalizer and save it inmediately, so that when it is eventually deleted we will
	// have time to do our cleanup actions. This will generate another call to our reconciler
	// where we will do the real work.
	if !deleting && !finalizer {
		controllerutil.AddFinalizer(copy, finalizerName)
		err = r.client.Patch(ctx, copy, clnt.MergeFrom(object))
		if err != nil {
			r.logger.Error(
				err,
				"Failed to add finalizer",
				"namespace", request.Namespace,
				"name", request.Name,
				"finalizer", finalizerName,
			)
		}
		return
	}

	// If the object is being deleted then we need to do our cleaning actions, save the updated
	// status and remove the finalizer.
	if deleting {
		result, err = r.processDelete(ctx, copy)
		if err != nil {
			return
		}
		err = r.client.Status().Patch(ctx, copy, clnt.MergeFrom(object))
		if err != nil {
			r.logger.Error(
				err,
				"Failed to updated status",
				"namespace", request.Namespace,
				"name", request.Name,
			)
			return
		}
		controllerutil.RemoveFinalizer(copy, finalizerName)
		err = r.client.Patch(ctx, copy, clnt.MergeFrom(object))
		if err != nil {
			r.logger.Error(
				err,
				"Failed to remove finalizer",
				"namespace", request.Namespace,
				"name", request.Name,
				"finalizer", finalizerName,
			)
		}
		return
	}

	// If we are here then the object was just created or updated, and it already has our
	// finalizer, so we must do our update processing and save the updated status.
	result, err = r.processUpdate(ctx, copy)
	if err != nil {
		r.logger.Error(
			err,
			"Failed to process update",
			"namespace", request.Namespace,
			"name", request.Name,
		)
		return
	}
	err = r.client.Status().Patch(ctx, copy, clnt.MergeFrom(object))
	if err != nil {
		r.logger.Error(
			err,
			"Failed to updated status",
			"namespace", request.Namespace,
			"name", request.Name,
		)
		return
	}
	r.logger.Info(
		"Saved updated status",
		"namespace", request.Namespace,
		"name", request.Name,
	)

	return
}

func (r *AllocationReconciler) processUpdate(ctx context.Context,
	object *pluginapi.NodeAllocationRequest) (result reconcile.Result, err error) {
	// Inform in the log that we are fulfilling the request:
	r.logger.Info(
		"Fulfilling request",
		"namespace", object.Namespace,
		"name", object.Name,
		"cloud_id", object.Spec.CloudID,
		"location", object.Spec.Location,
		"extensions", object.Spec.Extensions,
	)

	// Do the actual processing ...

	// If the node identifier is not yet assiged we should assign it now. Note that in this
	// example it is just a random UUID, but in reality it should be an identifier that allows
	// the hardware manager to identify the node in later requests to update or release it.
	if object.Status.NodeID == "" {
		object.Status.NodeID = uuid.NewString()
	}

	// Create or update the secret containing the BMC credentials of the node. The secret will
	// be in the same namespace than the allocation request, and the name will be the name of
	// the allocation request followed with a `-bmc` suffix.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: object.Namespace,
			Name:      fmt.Sprintf("%s-bmc", object.Name),
		},
	}
	_, err = controllerutil.CreateOrPatch(ctx, r.client, secret, func() error {
		err := controllerutil.SetOwnerReference(object, secret, r.client.Scheme())
		if err != nil {
			return err
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data["username"] = []byte("myuser")
		secret.Data["password"] = []byte("mypass")
		return nil
	})
	if err != nil {
		return
	}
	r.logger.Info(
		"Created BMC credentials secret",
		"namespace", secret.Namespace,
		"name", secret.Name,
	)

	// Set the reference to the BMC credentials and the rest of the BMC details:
	object.Status.BMC.Address = "https://mybmc.com"
	object.Status.BMC.CredentialsName = secret.Name

	// Update the conditions:
	meta.SetStatusCondition(&object.Status.Conditions, metav1.Condition{
		Type:    pluginapi.FulfilledCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "Fulfilled",
		Message: "The request has been fulfilled",
	})

	// Inform in the log that the request is fulfilled:
	r.logger.Info(
		"Fulfilled request",
		"namespace", object.Namespace,
		"name", object.Name,
		"cloud_id", object.Spec.CloudID,
		"location", object.Spec.Location,
		"extensions", object.Spec.Extensions,
	)

	return
}

func (r *AllocationReconciler) processDelete(ctx context.Context,
	object *pluginapi.NodeAllocationRequest) (result reconcile.Result, err error) {
	r.logger.Info(
		"Performing cleanup",
		"namespace", object.GetNamespace(),
		"name", object.GetName(),
	)
	return
}
