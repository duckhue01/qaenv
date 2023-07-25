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
	"strconv"
	"time"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	qaenviov1 "github.com/duckhue01/qaenv/api/v1alpha1"
)

type PRLabels string

const (
	ReadyToDeployQA PRLabels = "ready-to-deploy-qa"
	ReadyToReVokeQA PRLabels = "ready-to-revoke-qa"
)

var (
	EmptyString        = ""
	EmptyStringPointer = &EmptyString
)

// CoordinatorReconciler reconciles a Coordinator object
type CoordinatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	gh       *github.Client
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
		log.Error(err, "coordinator not found")
		return ctrl.Result{}, err
	}

	// initiate state
	if coordinator.Status.PullRequestMap == nil {
		coordinator.Status.PullRequestMap = make(map[string]qaenviov1.PullRequest)
	}

	if len(coordinator.Status.QaEnvs) == 0 {
		coordinator.Status.QaEnvs = make([]bool, coordinator.Spec.QAEnvLimit)
	}

	fmt.Println(coordinator.Status)

	secret := corev1.Secret{}

	if err := r.Get(
		context.Background(),
		types.NamespacedName{
			Namespace: coordinator.Spec.SecretRef.Namespace,
			Name:      coordinator.Spec.SecretRef.Name,
		},
		&secret,
	); err != nil {
		log.Error(err, "secret not found")
		return ctrl.Result{}, err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(secret.Data["githubPAT"])},
	)

	r.gh = github.NewClient(oauth2.NewClient(ctx, ts))

	err := r.reconcileGithubPR(ctx, &coordinator)

	// reconcileQAEnv
	if err != nil {
		log.Error(err, "error reconcile github pr")
	}

	// reflect to spec status
	if err := r.Client.Status().Update(ctx, &coordinator); err != nil {
		log.Error(err, "error reconcile github pr")
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: 5 * time.Second,
	}, err
}

func (r *CoordinatorReconciler) reconcileGithubPR(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
) error {
	log := log.FromContext(ctx)
	latestPRs := make(map[int]*github.PullRequest)

	// get all of repo's prs that is opened
	prs, _, err := r.gh.PullRequests.List(
		ctx,
		coordinator.Spec.GithubRepoOwner,
		coordinator.Spec.GithubRepoName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("error get list of opened pr: %w", err)
	}

	for _, pr := range prs {
		latestPRs[*pr.Number] = pr
	}

	// closed/merged PR => revoke QAEnv
	// PRs have ready-to-revoke-qa label and QAEnv already existed => revoke QAEnv
	for k, v := range coordinator.Status.PullRequestMap {
		kInt, err := strconv.ParseInt(k, 10, 0)

		if err != nil {
			return fmt.Errorf("error parse pr id string to int: %w", err)
		}

		switch v.Status {
		case qaenviov1.Pending:
			delete(coordinator.Status.PullRequestMap, k)
		default:
			// remove pr has been closed/merged
			if latestPRs[int(kInt)] == nil {
				err = r.revokeQAEnv(ctx, coordinator, int(kInt))
				if err != nil {
					log.Error(err, "error revoke QAEnv")
					return err
				}
				continue
			}

			// remove pr have ready-to-revoke-qa-env label
			for _, label := range latestPRs[int(kInt)].Labels {
				if *label.Name == string(ReadyToReVokeQA) {
					err = r.revokeQAEnv(ctx, coordinator, int(kInt))
					if err != nil {
						log.Error(err, "error revoke QAEnv")
						return err
					}

					log.Info("QA env is revoked for PR")
				}
			}

		}

	}

	// there are new PRs with ready-to-deploy-qa label => create QAEnv
	for k, pr := range latestPRs {
		if coordinator.Status.PullRequestMap[fmt.Sprint(k)].QAEnv == nil {
			for _, label := range pr.Labels {
				if *label.Name == string(ReadyToDeployQA) {
					err := r.createQAEnv(ctx, coordinator, k)
					if err != nil {
						log.Error(err, "error create QAEnv")
						return err
					}

				}
			}
		}
	}

	return nil
}

func (r *CoordinatorReconciler) revokeQAEnv(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	prId int,
) error {
	if err := r.Client.Delete(ctx, &qaenviov1.QAEnv{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprint(prId),
			Namespace: coordinator.Namespace,
		},
	}); err != nil {
		return err
	}

	switch coordinator.Status.PullRequestMap[fmt.Sprint(prId)].Status {
	case qaenviov1.Pending:
		delete(coordinator.Status.PullRequestMap, fmt.Sprint(prId))
	default:
		coordinator.Status.QaEnvs[*coordinator.Status.PullRequestMap[fmt.Sprint(prId)].QAEnv] = false
		delete(coordinator.Status.PullRequestMap, fmt.Sprint(prId))

	}

	err := r.removeLabelOnPR(ctx, coordinator, prId, ReadyToReVokeQA)
	if err != nil {
		return fmt.Errorf("error revoke github label: %w", err)
	}

	err = r.createCommentOnPR(ctx, coordinator, prId, "PR is revoked from ? env")
	if err != nil {
		return fmt.Errorf("error create github comment: %w", err)
	}

	return nil
}

func (r *CoordinatorReconciler) createQAEnv(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	prId int,
) error {

	// check if there are available qaenvs
	var aenv *int
	for i, env := range coordinator.Status.QaEnvs {
		if env {
			aenv = &i
			break
		}
	}

	if aenv != nil {
		qaEnv := &qaenviov1.QAEnv{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprint(prId),
				Namespace: coordinator.Namespace,
			},
			Spec: qaenviov1.QAEnvSpec{},
		}

		if err := ctrl.SetControllerReference(coordinator, qaEnv, r.Scheme); err != nil {
			return err
		}

		if err := r.Client.Create(ctx, qaEnv); err != nil {
			return err
		}

		err := r.removeLabelOnPR(ctx, coordinator, prId, ReadyToDeployQA)
		if err != nil {
			return fmt.Errorf("error remove github label: %w", err)
		}

		err = r.createCommentOnPR(ctx, coordinator, prId, "PR is deployed to ? env")
		if err != nil {
			return fmt.Errorf("error create github comment: %w", err)
		}

		coordinator.Status.PullRequestMap[fmt.Sprint(prId)] = qaenviov1.PullRequest{
			Status: qaenviov1.Allocated,
			QAEnv:  aenv,
		}

		coordinator.Status.QaEnvs[*aenv] = true
	} else {

		coordinator.Status.PullRequestMap[fmt.Sprint(prId)] = qaenviov1.PullRequest{
			Status: qaenviov1.Pending,
			QAEnv:  nil,
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CoordinatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qaenviov1.Coordinator{}).
		Owns(&qaenviov1.QAEnv{}).
		Complete(r)
}
