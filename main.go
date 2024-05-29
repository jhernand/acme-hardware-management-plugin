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
	"os"

	pluginapi "github.com/openshift-kni/oran-o2ims/api/hardwaremanagement/v1alpha1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	// Create a logger and configure libraries to use it:
	logger := zap.New()
	ctrl.SetLogger(logger)
	klog.SetLogger(logger)

	// Create the controller manager:
	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		logger.Error(err, "Failed to create controller manager")
		os.Exit(1)
	}

	// Add the types of the plugin API to the scheme:
	err = pluginapi.AddToScheme(manager.GetScheme())
	if err != nil {
		logger.Error(err, "Failed to add plugin API types to the scheme")
		os.Exit(1)
	}

	// Create the allocation request reconciler:
	err = ctrl.NewControllerManagedBy(manager).
		For(&pluginapi.NodeAllocationRequest{}).
		Complete(&AllocationReconciler{
			logger: logger.WithName("allocation"),
			client: manager.GetClient(),
		})
	if err != nil {
		logger.Error(err, "Failed to create allocation reconciler")
		os.Exit(1)
	}

	// Create the release request reconciler:
	err = ctrl.NewControllerManagedBy(manager).
		For(&pluginapi.NodeReleaseRequest{}).
		Complete(&ReleaseReconciler{
			logger: logger.WithName("release"),
			client: manager.GetClient(),
		})
	if err != nil {
		logger.Error(err, "Failed to create release reconciler")
		os.Exit(1)
	}

	// Start the controller manager, then wait for the signal to stop it:
	logger.Info("Starting controller manager")
	err = manager.Start(ctrl.SetupSignalHandler())
	if err != nil {
		logger.Error(err, "Controller manager failed")
		os.Exit(1)
	}
}
