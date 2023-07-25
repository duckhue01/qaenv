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

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	qaenviov1 "github.com/duckhue01/qaenv/api/v1alpha1"
)

// QAEnvReconciler reconciles a QAEnv object
type QAEnvReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=qaenv.io,resources=qaenvs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=qaenv.io,resources=qaenvs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=qaenv.io,resources=qaenvs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the QAEnv object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *QAEnvReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	qaEnv := qaenviov1.QAEnv{}

	if err := r.Get(context.Background(), req.NamespacedName, &qaEnv); err != nil {
		log.Error(err, "coordinator not found")
		return ctrl.Result{}, err
	}
	log.Info(qaEnv.Name)

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: 5 * time.Second,
	}, nil
}

func (r *QAEnvReconciler) createNamespace() {

}

func (r *QAEnvReconciler) createImagePolicy() {

}

func (r *QAEnvReconciler) createKustomization() {

}

func (r *QAEnvReconciler) createGitRepository() {

}

// SetupWithManager sets up the controller with the Manager.
func (r *QAEnvReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qaenviov1.QAEnv{}).
		Owns(&imagev1.ImagePolicy{}).
		Owns(&sourcev1.GitRepository{}).
		Owns(&kustomizev1.Kustomization{}).
		Complete(r)
}
