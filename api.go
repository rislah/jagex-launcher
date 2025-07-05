package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type Account struct {
	Username     string      `json:"username"`
	RefreshToken string      `json:"refresh_token"`
	AccessToken  string      `json:"access_token"`
	IDToken      string      `json:"id_token"`
	ExpiresAt    int64       `json:"expires_at"`
	SessionID    string      `json:"session_id"`
	characters   []Character 
}

type Character struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

type SessionResponse struct {
	SessionID string `json:"sessionId"`
}

type AccountsResponse []struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
}

type UserInfoResponse struct {
	Nickname string `json:"nickname"`
}

type JagexAPI struct {
	httpClient      *http.Client
	oauthConfig     *oauth2.Config
	gameOAuthConfig *oauth2.Config
}

const (
	AUTH_URL  = "https://account.jagex.com/oauth2/auth"
	TOKEN_URL = "https://account.jagex.com/oauth2/token"
)

func NewJagexAPI() *JagexAPI {
	return &JagexAPI{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		oauthConfig: &oauth2.Config{
			ClientID:    "com_jagex_auth_desktop_launcher",
			RedirectURL: "https://secure.runescape.com/m=weblogin/launcher-redirect",
			Scopes:      []string{"openid", "offline", "gamesso.token.create", "user.profile.read"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  AUTH_URL,
				TokenURL: TOKEN_URL,
			},
		},
		gameOAuthConfig: &oauth2.Config{
			ClientID:    "1fddee4e-b100-4f4e-b2b0-097f9088f9d2",
			RedirectURL: "http://localhost",
			Scopes:      []string{"openid", "offline"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  AUTH_URL,
				TokenURL: TOKEN_URL,
			},
		},
	}
}

func (api *JagexAPI) getAuthURL(codeChallenge, state string) string {
	return api.oauthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("auth_method", ""),
		oauth2.SetAuthURLParam("login_type", ""),
		oauth2.SetAuthURLParam("flow", "launcher"),
		oauth2.SetAuthURLParam("prompt", "login"),
	)
}

func (api *JagexAPI) exchangeCodeForToken(code, codeVerifier string) (*TokenResponse, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, api.httpClient)
	token, err := api.oauthConfig.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  token.AccessToken,
		ExpiresIn:    int(token.Expiry.Sub(time.Now()).Seconds()),
		IDToken:      token.Extra("id_token").(string),
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
	}, nil
}

func (api *JagexAPI) refreshToken(refreshToken string) (*TokenResponse, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, api.httpClient)
	token := &oauth2.Token{
		RefreshToken: refreshToken,
		Expiry:       time.Now().Add(-1 * time.Hour),
	}

	newToken, err := api.oauthConfig.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  newToken.AccessToken,
		ExpiresIn:    int(newToken.Expiry.Sub(time.Now()).Seconds()),
		IDToken:      newToken.Extra("id_token").(string),
		RefreshToken: newToken.RefreshToken,
		TokenType:    newToken.TokenType,
	}, nil
}

func (api *JagexAPI) getUserInfo(accessToken string) (*UserInfoResponse, error) {
	req, err := http.NewRequest("GET", "https://account.jagex.com/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userInfo UserInfoResponse
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	return &userInfo, err
}

func (api *JagexAPI) getGameAuthURL(idToken, state string) string {
	return api.gameOAuthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("id_token_hint", idToken),
		oauth2.SetAuthURLParam("nonce", generateRandomString(48)),
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("response_type", "id_token code"),
	)
}

func (api *JagexAPI) createGameSession(gameIDToken string) (*SessionResponse, error) {
	payload := map[string]string{"idToken": gameIDToken}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://auth.jagex.com/game-session/v1/sessions", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sessionResp SessionResponse
	err = json.NewDecoder(resp.Body).Decode(&sessionResp)
	return &sessionResp, err
}

func (api *JagexAPI) getAccounts(sessionID string) (*AccountsResponse, error) {
	req, err := http.NewRequest("GET", "https://auth.jagex.com/game-session/v1/accounts", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sessionID)

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var accounts AccountsResponse
	err = json.NewDecoder(resp.Body).Decode(&accounts)
	return &accounts, err
}
