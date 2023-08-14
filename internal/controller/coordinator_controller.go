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
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	qaenviov1 "github.com/duckhue01/qaenv/api/v1alpha1"
)

const (
	InQA = "in-qa"
)

// CoordinatorReconciler reconciles a Coordinator object
type CoordinatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	gh       *github.Client

	tickets map[string][]github.PullRequest
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
	if coordinator.Status.TicketMap == nil {
		coordinator.Status.TicketMap = make(map[string]*qaenviov1.TicketMap)
	}

	if len(coordinator.Status.QaEnvs) == 0 {
		coordinator.Status.QaEnvs = make(map[string]bool)
		for _, v := range coordinator.Spec.QAEnvTemplate.QaEnvs {
			coordinator.Status.QaEnvs[fmt.Sprint(v)] = false
		}

	}

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
	if err != nil {
		log.Error(err, "error reconcile github pr")
		return ctrl.Result{}, err
	}

	// todo: remove
	for k, t := range r.tickets {
		for _, pr := range t {
			fmt.Printf("ticket: %s pr: %s - %s \n", k, pr.Head.Repo.GetName(), pr.Head.GetRef())
		}
	}

	err = r.reconcileQAEnv(ctx, &coordinator)
	if err != nil {
		log.Error(err, "error reconcile qa env")
		return ctrl.Result{}, err
	}

	if err := r.Client.Status().Update(ctx, &coordinator); err != nil {
		log.Error(err, "error reconcile github pr")
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: coordinator.Spec.Interval.Duration,
	}, err
}

func (r *CoordinatorReconciler) reconcileGithubPR(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
) error {
	r.tickets = make(map[string][]github.PullRequest)

	// need to follow operator code style
	for repoName := range coordinator.Spec.Services {
		err := r.makeTicketMapFromPullRequest(ctx, coordinator.Spec.GithubRepoOwner, repoName)
		if err != nil {
			return fmt.Errorf("error get list of opened pr: %w", err)
		}
	}

	return nil
}

// reconcileQAEnv make sure current status match with the state of git github
func (r *CoordinatorReconciler) reconcileQAEnv(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
) error {
	log := log.FromContext(ctx)

	// remove qaenv for ticket doesn't have any in-qa pr and update prs for that ticket
	for ticketId, tval := range coordinator.Status.TicketMap {
		if r.tickets[ticketId] == nil {
			err := r.revokeQAEnv(ctx, coordinator, ticketId)
			// todo: notice pr will be removed
			if err != nil {
				log.Error(err, "error revoke QAEnv")
				return err
			}
			continue
		}

		// remove all non-inqa pr
		for key, pr := range tval.PullRequestsMap {
			inqa := false

			for _, currPr := range r.tickets[ticketId] {
				if fmt.Sprintf("%s-%d", currPr.Head.GetRepo().GetName(), currPr.GetNumber()) == key {
					inqa = true
				}
			}

			if !inqa {
				r.createCommentOnPR(
					ctx,
					coordinator.Spec.GithubRepoOwner,
					pr.Repository,
					pr.Name,
					fmt.Sprintf("this pr has been allocated on qaenv: %s", *coordinator.Status.TicketMap[ticketId].QAEnvIndex),
				)
				delete(coordinator.Status.TicketMap[ticketId].PullRequestsMap, key)

			}
		}
	}

	// create qaenv for ticket has in-qa pr and update prs for that ticket
	for ticketId, prs := range r.tickets {
		if coordinator.Status.TicketMap[ticketId] == nil {

			// check if there are available qaenvs
			var envSlot *string
			for i, env := range coordinator.Status.QaEnvs {
				if !env {
					envSlot = &i
					break
				}
			}

			if envSlot != nil {
				// allocate the qaenv for ticket
				err := r.createQAEnv(ctx, coordinator, ticketId, *envSlot)
				if err != nil {
					log.Error(err, "error allocate the qaenv for ticket")
					return err
				}

				prStatus := make(map[string]qaenviov1.PullRequest, 0)

				for _, v := range prs {

					key := fmt.Sprintf("%s-%d", v.Head.GetRepo().GetName(), v.GetNumber())
					prStatus[key] = qaenviov1.PullRequest{
						Name:       v.GetNumber(),
						Repository: v.Head.GetRepo().GetName(),
					}

					r.createCommentOnPR(
						ctx,
						coordinator.Spec.GithubRepoOwner,
						v.Head.GetRepo().GetName(),
						v.GetNumber(),
						fmt.Sprintf("this pr has been allocated on qaenv: %s", *envSlot),
					)
				}

				coordinator.Status.TicketMap[ticketId] = &qaenviov1.TicketMap{
					Status:          qaenviov1.Allocated,
					QAEnvIndex:      envSlot,
					PullRequestsMap: prStatus,
				}

				coordinator.Status.QaEnvs[*envSlot] = true

			} else {
				// not enough env for ticket
				prStatus := make(map[string]qaenviov1.PullRequest, 0)

				for _, v := range prs {

					key := fmt.Sprintf("%s-%d", v.Head.GetRepo().GetName(), v.GetNumber())
					prStatus[key] = qaenviov1.PullRequest{
						Name:       v.GetNumber(),
						Repository: v.Head.GetRepo().GetName(),
					}

					r.createCommentOnPR(
						ctx,
						coordinator.Spec.GithubRepoOwner,
						v.Head.GetRepo().GetName(),
						v.GetNumber(),
						"this pr has been push on queue",
					)
				}

				coordinator.Status.TicketMap[ticketId] = &qaenviov1.TicketMap{
					Status:          qaenviov1.Pending,
					QAEnvIndex:      nil,
					PullRequestsMap: prStatus,
				}
			}

		} else {
			// insert more in-qa pr
			for _, currPr := range prs {
				prKey := fmt.Sprintf("%s-%d", currPr.Head.GetRepo().GetName(), currPr.GetNumber())
				if _, ok := coordinator.Status.TicketMap[ticketId].PullRequestsMap[prKey]; !ok {
					coordinator.Status.TicketMap[ticketId].PullRequestsMap[prKey] = qaenviov1.PullRequest{
						Name:       currPr.GetNumber(),
						Repository: currPr.Head.GetRepo().GetName(),
					}

					r.createCommentOnPR(
						ctx,
						coordinator.Spec.GithubRepoOwner,
						currPr.Head.GetRepo().GetName(),
						currPr.GetNumber(),
						fmt.Sprintf("this pr has been allocated on qaenv: %s", *coordinator.Status.TicketMap[ticketId].QAEnvIndex),
					)
				}

			}
		}

	}

	//re-create allocated ticket and allocate pending ticket if there is available slot
	qaenvs := &qaenviov1.QAEnvList{}
	if err := r.Client.List(ctx, qaenvs,
		client.InNamespace(coordinator.Namespace),
		// client.MatchingFields{ownerKey: obj.Name}, //todo: index
	); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if len(qaenvs.Items) != len(coordinator.Status.TicketMap) {
		for ticketId, ticket := range coordinator.Status.TicketMap {

			switch ticket.Status {
			case qaenviov1.Pending:
				// check if there are available qaenvs
				var envSlot *string
				for i, env := range coordinator.Status.QaEnvs {
					if !env {
						envSlot = &i
						break
					}
				}

				if envSlot != nil {
					err := r.createQAEnv(ctx, coordinator, ticketId, *envSlot)
					if err != nil {
						log.Error(err, "error create QAEnv")
						return err
					}

					coordinator.Status.TicketMap[ticketId].Status = qaenviov1.Allocated
					coordinator.Status.TicketMap[ticketId].QAEnvIndex = envSlot
					coordinator.Status.QaEnvs[*envSlot] = true

					for _, pr := range ticket.PullRequestsMap {
						r.createCommentOnPR(
							ctx,
							coordinator.Spec.GithubRepoOwner,
							pr.Repository,
							pr.Name,
							fmt.Sprintf("this pr has been allocated on qaenv: %s", *envSlot))
					}
				}

			case qaenviov1.Allocated:
				// todo: need review create condition to avoid too much requests to api server
				// todo: check if qaenv existed or not
				err := r.createQAEnv(ctx, coordinator, ticketId, *ticket.QAEnvIndex)
				if err != nil {
					log.Error(err, "error create QAEnv")
					return err
				}
			}
		}
	}

	return nil
}

func (r *CoordinatorReconciler) makeTicketMapFromPullRequest(
	ctx context.Context,
	ownerName string,
	repoName string,
) error {
	prs, _, err := r.gh.PullRequests.List(
		ctx,
		ownerName,
		repoName,
		nil,
	)
	if err != nil {
		return err
	}

	for _, pr := range prs {
		inqa := false
		for _, l := range pr.Labels {
			if l.GetName() == InQA {
				inqa = true
			}
		}

		if inqa {
			r.tickets[pr.Head.GetRef()] = append(r.tickets[pr.Head.GetRef()], *pr)
		}
	}

	return nil
}

func (r *CoordinatorReconciler) revokeQAEnv(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	ticketId string,
) error {
	log := ctrl.LoggerFrom(ctx)
	if err := r.Client.Delete(ctx, &qaenviov1.QAEnv{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", coordinator.Name, strings.ToLower(ticketId)),
			Namespace: coordinator.Namespace},
	}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		log.Info("qaenv has been deleted", "name", ticketId, "namespace", coordinator.Namespace)
	}

	coordinator.Status.QaEnvs[*coordinator.Status.TicketMap[ticketId].QAEnvIndex] = false
	delete(coordinator.Status.TicketMap, fmt.Sprint(ticketId))

	return nil
}

func (r *CoordinatorReconciler) createQAEnv(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	ticketId string,
	envIndex string,
) error {
	log := log.FromContext(ctx)
	imagePolicies := make([]qaenviov1.ImagePolicySpec, 0)

	for _, repo := range coordinator.Spec.Services {
		for _, service := range repo {
			imagePolicies = append(imagePolicies, qaenviov1.ImagePolicySpec{
				Name: fmt.Sprintf("%s-qa%s", service, envIndex),
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name:      service,
					Namespace: coordinator.Namespace,
				},
			})
		}
	}

	qaEnv := &qaenviov1.QAEnv{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", coordinator.Name, strings.ToLower(ticketId)),
			Namespace: coordinator.Namespace,
		},
		Spec: qaenviov1.QAEnvSpec{
			TicketId:      ticketId,
			QAEnvIndex:    envIndex,
			ImagePolicies: imagePolicies,
			KustomizationSpec: qaenviov1.KustomizationSpec{
				Path:      fmt.Sprintf("./clusters/songoku/qa/%s/%s", envIndex, coordinator.Spec.ProjectName),
				Prune:     true,
				SourceRef: coordinator.Spec.SourceRef,
				Kind:      coordinator.Spec.SourceRef.Kind,
			},
			Interval: coordinator.Spec.Interval,
		},
	}

	if err := ctrl.SetControllerReference(coordinator, qaEnv, r.Scheme); err != nil {
		return fmt.Errorf("error set controller reference: %w", err)
	}

	if err := r.Client.Create(ctx, qaEnv); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error create qaenv: %w", err)
		}

		log.Info("qaenv has been created", "name", ticketId, "namespace", coordinator.Namespace)
	}

	return nil
}

func (r *CoordinatorReconciler) createCommentOnPR(
	ctx context.Context,
	ownerName string,
	repoName string,
	prNumber int,
	content string,
) {
	log := log.FromContext(ctx)
	_, _, err := r.gh.Issues.CreateComment(ctx, ownerName, repoName, prNumber, &github.IssueComment{
		Body: &content,
	})

	if err != nil {
		log.Error(err, "error create comment")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *CoordinatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qaenviov1.Coordinator{}).
		Owns(&qaenviov1.QAEnv{}).
		Complete(r)
}
