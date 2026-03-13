package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

// GoogleOAuth wraps the OAuth2 config and handles the code exchange
type GoogleOAuth struct {
	config *oauth2.Config
}

func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			// Scopes: only request what we need — email + basic profile
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// AuthURL returns the URL to redirect the user to Google's login page.
// state is a CSRF token we generate per-request and verify on callback.
func (g *GoogleOAuth) AuthURL(state string) string {
	return g.config.AuthCodeURL(state,
		oauth2.AccessTypeOnline, // no offline refresh — we use our own refresh tokens
		oauth2.ApprovalForce,    // always show consent screen (good for multi-account)
	)
}

// Exchange trades the authorization code for an access token,
// then fetches the user's profile from Google.
func (g *GoogleOAuth) Exchange(ctx context.Context, code string) (*GoogleUserInfo, error) {
	// Step 1: Exchange code → Google OAuth token
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}

	// Step 2: Use token to fetch user profile
	client := g.config.Client(ctx, token)
	resp, err := client.Get(googleUserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("fetching google user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo returned %d", resp.StatusCode)
	}

	var user GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding google user: %w", err)
	}

	if !user.Verified {
		return nil, fmt.Errorf("google email not verified")
	}

	return &user, nil
}
