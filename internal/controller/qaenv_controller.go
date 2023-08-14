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
	"fmt"
	"time"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

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

	qaEnv := &qaenviov1.QAEnv{}
	if err := r.Get(context.Background(), req.NamespacedName, qaEnv); err != nil {
		log.Error(err, "qaEnv not found") // need to review
		return ctrl.Result{}, err
	}

	coordinator := &qaenviov1.Coordinator{}
	if err := r.Get(context.Background(),
		types.NamespacedName{
			Name:      metav1.GetControllerOf(qaEnv).Name,
			Namespace: qaEnv.GetNamespace(),
		}, coordinator); err != nil {
		log.Error(err, "coordinator not found")
		return ctrl.Result{}, err
	}

	// initiate state
	if qaEnv.Status.ImagePolicies == nil {
		qaEnv.Status.ImagePolicies = make(map[string]*meta.NamespacedObjectReference)
	}

	err := r.reconcileGithubPolicy(ctx, nil, qaEnv)
	if err != nil {
		log.Error(err, "can not reconcile github policy")
		return ctrl.Result{}, err
	}

	err = r.reconcileKustomization(ctx, coordinator, qaEnv)
	if err != nil {
		log.Error(err, "can not reconcile kustomization")
		return ctrl.Result{}, err
	}

	if err := r.Client.Status().Update(ctx, qaEnv); err != nil {
		log.Error(err, "error updating status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: qaEnv.Spec.Interval.Duration * time.Second,
	}, nil
}

func (r *QAEnvReconciler) reconcileGithubPolicy(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	qaenv *qaenviov1.QAEnv,
) error {
	log := log.FromContext(ctx)
	for _, ip := range qaenv.Spec.ImagePolicies {

		if qaenv.Status.ImagePolicies[ip.Name] == nil {
			imagePolicy := &imagev1.ImagePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ip.Name,
					Namespace: qaenv.Namespace,
				},
				Spec: imagev1.ImagePolicySpec{
					ImageRepositoryRef: ip.ImageRepositoryRef,
					Policy: imagev1.ImagePolicyChoice{
						Alphabetical: &imagev1.AlphabeticalPolicy{
							Order: "asc",
						},
					},
					FilterTags: &imagev1.TagFilter{
						Pattern: fmt.Sprintf(`qa.%s\.(?P<ts>[1-9][0-9].*)$`, qaenv.Spec.TicketId),
						Extract: `$ts`,
					},
				},
			}

			if err := ctrl.SetControllerReference(qaenv, imagePolicy, r.Scheme); err != nil {
				return fmt.Errorf("error set controller ref: %w", err)
			}

			if err := r.Client.Create(
				ctx,
				imagePolicy,
			); err != nil {
				if !errors.IsAlreadyExists(err) {
					return fmt.Errorf("error create ImagePolicy: %w", err)
				}

				log.Info("ImagePolicy has been create")
			}

			qaenv.Status.ImagePolicies[ip.Name] = &meta.NamespacedObjectReference{
				Name:      ip.Name,
				Namespace: qaenv.Namespace,
			}
		}
	}

	return nil
}

func (r *QAEnvReconciler) reconcileKustomization(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	qaenv *qaenviov1.QAEnv,
) error {
	log := ctrl.LoggerFrom(ctx)

	if qaenv.Status.Kustomization == nil {
		kustomization := &kustomizev1.Kustomization{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: qaenv.Namespace,
				Name:      fmt.Sprintf("qa%s-vibes", qaenv.Spec.QAEnvIndex),
			},
			Spec: kustomizev1.KustomizationSpec{
				SourceRef: qaenv.Spec.KustomizationSpec.SourceRef,
				Path:      qaenv.Spec.KustomizationSpec.Path,
				Prune:     qaenv.Spec.KustomizationSpec.Prune,
				Interval:  qaenv.Spec.KustomizationSpec.Interval,
			},
		}

		if err := ctrl.SetControllerReference(qaenv, kustomization, r.Scheme); err != nil {
			return fmt.Errorf("error set controller ref: %w", err)
		}

		if err := r.Client.Create(ctx, kustomization); err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("error creating kustomization: %w", err)
			}

			log.Info("Kustomization has been created")
		}

		qaenv.Status.Kustomization = &meta.NamespacedObjectReference{
			Namespace: kustomization.Namespace,
			Name:      kustomization.Name,
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *QAEnvReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qaenviov1.QAEnv{}).
		Owns(&imagev1.ImagePolicy{}).
		Owns(&kustomizev1.Kustomization{}).
		Complete(r)
}
