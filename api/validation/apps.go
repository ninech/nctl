package validation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"github.com/theckman/yacspin"
)

// RepositoryValidator validates a git repository
type RepositoryValidator struct {
	GitInformationServiceURL string
	Token                    string
	Debug                    bool
}

// Validate validates the repository access and shows a visual spinner while doing so
func (v *RepositoryValidator) Validate(ctx context.Context, git *apps.ApplicationGitConfig, auth util.GitAuth) error {
	gitInfoClient, err := NewGitInformationClient(v.GitInformationServiceURL, v.Token)
	if err != nil {
		return err
	}
	msg := " testing repository access 🔐"
	spinner, err := format.NewSpinner(msg, msg)
	if err != nil {
		return err
	}
	gitInfoClient.SetLogRetryFunc(retryRepoAccess(spinner, v.Debug))
	if err := spinner.Start(); err != nil {
		return err
	}
	if err := testRepositoryAccess(ctx, gitInfoClient, git, auth); err != nil {
		if err := spinner.StopFail(); err != nil {
			return err
		}
		return err
	}
	return spinner.Stop()
}

// testRepositoryAccess tests if the given git repository can be accessed.
func testRepositoryAccess(ctx context.Context, client *GitInformationClient, git *apps.ApplicationGitConfig, auth util.GitAuth) error {
	repoInfo, err := client.RepositoryInformation(ctx, git.URL, auth)
	if err != nil {
		// we are not returning a detailed error here as it might be
		// too technical. The full error can still be seen by using
		// a different RetryLog function in the client.
		return errors.New(
			"communication issue with git information service " +
				"(use --skip-repo-access-check to skip this check)",
		)
	}
	if len(repoInfo.Warnings) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %s\n", strings.Join(repoInfo.Warnings, "."))
	}
	if repoInfo.Error != "" {
		return errors.New(repoInfo.Error)
	}
	if !(containsBranch(git.Revision, repoInfo) ||
		containsTag(git.Revision, repoInfo)) {
		return fmt.Errorf(
			"can not find specified git revision (%q) in repository",
			git.Revision,
		)
	}
	// it is possible to set a git URL without a proper scheme. In that
	// case, HTTPS is used as a default. If the access check succeeds we
	// need to overwrite the URL in the application as it will otherwise be
	// denied by the webhook.
	git.URL = repoInfo.RepositoryInfo.URL
	return nil
}

func retryRepoAccess(spinner *yacspin.Spinner, debug bool) func(err error) {
	return func(err error) {
		// in non debug mode we just change the color of the spinner to
		// indicate that something went wrong, but we are still on it
		if err := spinner.Colors("fgYellow"); err != nil {
			fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
		}
		if debug {
			fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
		}
	}
}
