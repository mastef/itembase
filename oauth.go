package itembase

import (
	"crypto/rand"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"log"
	"net/http"
)

func (c *client) newConf() *oauth2.Config {
	var endpointURL string

	if c.production {
		endpointURL = "https://accounts.itembase.com/oauth/v2"
		c.me = "https://users.itembase.com/v1/me"
		c.root = "https://api.itembase.io/v1"
		c.activation = "https://solutionservice.itembase.com"
	} else {
		endpointURL = "http://sandbox.accounts.itembase.io/oauth/v2"
		c.me = "http://sandbox.users.itembase.io/v1/me"
		c.root = "http://sandbox.api.itembase.io/v1"
		c.activation = "http://sandbox.solutionservice.itembase.io"
	}

	return &oauth2.Config{
		ClientID:     c.options.ClientID,
		ClientSecret: c.options.ClientSecret,
		Scopes:       c.options.Scopes,
		RedirectURL:  c.options.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  endpointURL + "/auth",
			TokenURL: endpointURL + "/token",
		},
	}
}

func (c *client) SaveToken(userID string, token *oauth2.Token) (err error) {
	if c.options.TokenHandler.TokenSaver != nil {
		err = c.options.TokenHandler.TokenSaver(userID, token)
	} else {
		err = errors.New("No Token Store!")
	}
	return
}

func (c *client) GetCachedToken(userID string) (token *oauth2.Token, err error) {
	if c.options.TokenHandler.TokenLoader != nil {
		token, err = c.options.TokenHandler.TokenLoader(userID)
	} else {
		err = errors.New("No Token Cache!")
	}
	return
}

func (c *client) GiveTokenPermissions(authURL string) (authcode string, err error) {
	// add logic for handing retrieving code for oauth exchange and matching state
	// For example throw an error, and send email to user instead with this link

	if c.options.TokenHandler.TokenPermissions != nil {
		if authcode, err = c.options.TokenHandler.TokenPermissions(authURL); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Println("Please go to the following URL : ", authURL)
		if _, err := fmt.Scan(&authcode); err != nil {
			log.Fatal(err)
		}
	}

	// Use the authorization code that is pushed to the redirect URL.
	// NewTransportWithCode will do the handshake to retrieve
	// an access token and initiate a Transport that is
	// authorized and authenticated by the retrieved token.
	return
}

// UserOAuthClient returns an oauth2 client for a specific user
func (c *client) UserOAuthClient(ctx context.Context, config *oauth2.Config, userID string) (client *http.Client, err error) {
	var userToken *oauth2.Token

	if userToken, err = c.GetCachedToken(userID); err != nil {
		// if token for user is not cached then go through oauth2 flow
		if userToken, err = c.newUserToken(ctx, config, userID); err != nil {
			return
		}
	}

	if !userToken.Valid() { // if user token is expired
		userToken = &oauth2.Token{RefreshToken: userToken.RefreshToken}
	}

	return config.Client(ctx, userToken), err
}

func (c *client) newUserToken(ctx context.Context, config *oauth2.Config, userID string) (*oauth2.Token, error) {
	stateBytes := make([]byte, 32)
	_, err := rand.Read(stateBytes)
	if err != nil {
		log.Fatalf("Unable to read random bytes: %v", err)
		return nil, err
	}
	state := fmt.Sprintf("%x", stateBytes)
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	authcode, err := c.GiveTokenPermissions(authURL)

	token, err := config.Exchange(oauth2.NoContext, authcode)
	if err != nil {
		log.Fatalf("Exchange error: %v", err)
		return nil, err
	}
	c.SaveToken(userID, token) // save token to datastore

	return token, nil
}

func (c *client) getUserToken(userID string) (Token *oauth2.Token) {
	config := c.newConf()
	client, err := c.UserOAuthClient(oauth2.NoContext, config, userID)

	_, err = client.Get(c.me)
	if err == nil {
		fmt.Errorf("Fetch should return an error if no refresh token is set")
	}

	Token, err = client.Transport.(*oauth2.Transport).Source.Token()

	if err != nil {
		log.Fatal("Exchange error: %v", err)
	}

	c.SaveToken(userID, Token)

	return
}
