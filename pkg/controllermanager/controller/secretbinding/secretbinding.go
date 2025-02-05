// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secretbinding

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller controls SecretBindings.
type Controller struct {
	reconciler                      reconcile.Reconciler
	secretBindingProviderReconciler reconcile.Reconciler

	hasSyncedFuncs []cache.InformerSynced

	secretBindingQueue workqueue.RateLimitingInterface
	shootQueue         workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSecretBindingController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <secretBindingInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSecretBindingController(
	ctx context.Context,
	clientMap clientmap.ClientMap,
	recorder record.EventRecorder,
) (
	*Controller,
	error,
) {
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	secretBindingInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.SecretBinding{})
	if err != nil {
		return nil, fmt.Errorf("failed to get SecretBinding Informer: %w", err)
	}

	shootInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Shoot Informer: %w", err)
	}

	secretBindingController := &Controller{
		reconciler:                      NewSecretBindingReconciler(logger.Logger, gardenClient.Client(), recorder),
		secretBindingProviderReconciler: NewSecretBindingProviderReconciler(logger.Logger, gardenClient.Client()),
		secretBindingQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SecretBinding"),
		shootQueue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Shoot"),
		workerCh:                        make(chan int),
	}

	secretBindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    secretBindingController.secretBindingAdd,
		UpdateFunc: secretBindingController.secretBindingUpdate,
		DeleteFunc: secretBindingController.secretBindingDelete,
	})

	shootInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: secretBindingController.shootAdd,
	})

	secretBindingController.hasSyncedFuncs = append(secretBindingController.hasSyncedFuncs, secretBindingInformer.HasSynced, shootInformer.HasSynced)

	return secretBindingController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, secretBindingWorkers, secretBindingProviderWorkers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running SecretBinding workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("SecretBinding controller initialized.")

	for i := 0; i < secretBindingWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.secretBindingQueue, "SecretBinding", c.reconciler, &waitGroup, c.workerCh)
	}
	for i := 0; i < secretBindingProviderWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.shootQueue, "SecretBinding Provider", c.secretBindingProviderReconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.secretBindingQueue.ShutDown()
	c.shootQueue.ShutDown()

	queueLengths := c.secretBindingQueue.Len() + c.shootQueue.Len()

	for {
		if queueLengths == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running SecretBinding worker and no items left in the queues. Terminated SecretBinding controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d SecretBinding worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, queueLengths)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}
