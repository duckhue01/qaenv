/*
Copyright 2023.

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

package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	qaenviov1 "github.com/duckhue01/qaenv/api/v1alpha1"
	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
)

// CoordinatorReconciler reconciles a Coordinator object
type CoordinatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	envM map[string]string
}

//+kubebuilder:rbac:groups=qaenv.io,resources=coordinators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=qaenv.io,resources=coordinators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=qaenv.io,resources=coordinators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Coordinator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *CoordinatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	coordinator := qaenviov1.Coordinator{}

	if err := r.Get(context.Background(), req.NamespacedName, &coordinator); err != nil {
		log.Info("preview environment not found")
	}

	imagePolicy := imagev1.ImagePolicy{}

	// Create initial state for controller
	if r.envM == nil {
		envM := make(map[string]string)

		for _, v := range coordinator.Spec.ImageRepositories {
			if err := r.Get(ctx, types.NamespacedName{
				Namespace: req.NamespacedName.Namespace,
				Name:      v,
			}, &imagePolicy); err != nil {
				log.Info("can not get ImagePolicy")
			}

			envM[v] = imagePolicy.Status.LatestImage
		}

		r.envM = envM

		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, nil
	}

	// latestEnvM := make(map[string]string)
	for _, v := range coordinator.Spec.ImageRepositories {
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: req.NamespacedName.Namespace,
			Name:      v,
		}, &imagePolicy); err != nil {
			log.Info("can not get ImagePolicy")
		}

		r.reconcileForEachEnv(v, imagePolicy.Status.LatestImage)
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: 5 * time.Second,
	}, nil
}

func (r *CoordinatorReconciler) reconcileForEachEnv(env, latestTag string) {

}

// SetupWithManager sets up the controller with the Manager.
func (r *CoordinatorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&qaenviov1.Coordinator{}).
		Complete(r)
}
