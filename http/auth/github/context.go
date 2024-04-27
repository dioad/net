package github

import (
	"context"

	"github.com/google/go-github/v33/github"
	"golang.org/x/oauth2"

	"github.com/dioad/generics"
)

type UserInfo struct {
	Login                string
	Name                 string
	PrimaryEmail         string
	PrimaryEmailVerified bool
	Company              string
	WebSite              string
	Location             string
	PlanName             string
}

type githubUserInfoContext struct{}

func NewContextWithGitHubUserInfo(ctx context.Context, userInfo *UserInfo) context.Context {
	return context.WithValue(ctx, githubUserInfoContext{}, userInfo)
}

func GitHubUserInfoFromContext(ctx context.Context) *UserInfo {
	val := ctx.Value(githubUserInfoContext{})
	if val != nil {
		return val.(*UserInfo)
	}
	return nil
}

func FetchUserInfo(accessToken string) (*UserInfo, error) {
	t := &TokenSource{AccessToken: accessToken}
	oauthClient := oauth2.NewClient(context.Background(), t)
	client := github.NewClient(oauthClient)

	u, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, err
	}

	emails, _, err := client.Users.ListEmails(context.Background(), &github.ListOptions{PerPage: 10})
	if err != nil {
		return nil, err
	}

	userInfo := &UserInfo{
		Login:    u.GetLogin(),
		Name:     u.GetName(),
		WebSite:  u.GetBlog(),
		Company:  u.GetCompany(),
		Location: u.GetLocation(),
		PlanName: u.GetPlan().GetName(),
	}

	primaryEmail, err := generics.SelectOne(emails, func(e *github.UserEmail) bool {
		return e.GetPrimary()
	})
	if err != nil {
		return nil, err
	}

	userInfo.PrimaryEmail = primaryEmail.GetEmail()
	userInfo.PrimaryEmailVerified = primaryEmail.GetVerified()

	return userInfo, nil
}
