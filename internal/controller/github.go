package controller

import (
	"context"

	qaenviov1 "github.com/duckhue01/qaenv/api/v1alpha1"
	"github.com/google/go-github/v53/github"
)

func (r *CoordinatorReconciler) createCommentOnPR(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	prId int,
	message string,
) error {
	_, _, err := r.gh.Issues.CreateComment(ctx,
		coordinator.Spec.GithubRepoOwner,
		coordinator.Spec.GithubRepoName,
		prId,
		&github.IssueComment{
			Body: github.String(message),
		},
	)

	return err
}

func (r *CoordinatorReconciler) removeLabelOnPR(
	ctx context.Context,
	coordinator *qaenviov1.Coordinator,
	prId int,
	labelName PRLabels,
) error {
	_, err := r.gh.Issues.RemoveLabelForIssue(
		ctx,
		coordinator.Spec.GithubRepoOwner,
		coordinator.Spec.GithubRepoName,
		prId,
		string(labelName),
	)
	return err
}
