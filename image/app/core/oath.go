// Author: Eryk Kulikowski @ KU Leuven (2023). Apache 2.0 License

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"integration/app/config"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type OauthTokenRequest struct {
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectUri  string `json:"redirect_uri"`
	GrantType    string `json:"grant_type"`
	Resource     string `json:"resource,omitempty"`
}

type OauthTokenResponse struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Scope                 string `json:"scope"`
	TokenType             string `json:"token_type"`
	Error                 string `json:"error"`
	Error_description     string `json:"error_description"`
	Error_uri             string `json:"error_uri"`
}

type OauthTokenResponseStrings struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             string `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn string `json:"refresh_token_expires_in"`
	Scope                 string `json:"scope"`
	TokenType             string `json:"token_type"`
	Error                 string `json:"error"`
	Error_description     string `json:"error_description"`
	Error_uri             string `json:"error_uri"`
}

type TokenResponse struct {
	SessionId string `json:"session_id"`
}

var PluginConfig = map[string]config.RepoPlugin{}
var RedirectUri string

func GetOauthToken(ctx context.Context, id, code, nounce string) (TokenResponse, error) {
	res := TokenResponse{fmt.Sprintf("%v-%v-%v", id, code, nounce)}
	clientId := PluginConfig[id].TokenGetter.OauthClientId
	redirectUri := RedirectUri
	clientSecret, resource, postUrl, err := config.ClientSecret(clientId)
	if err != nil {
		return res, err
	}
	req := OauthTokenRequest{clientId, clientSecret, code, redirectUri, "authorization_code", resource}
	//data, _ := json.Marshal(req)
	//body := bytes.NewBuffer(data)
	request, _ := http.NewRequestWithContext(ctx, "POST", postUrl, encode(req))
	//request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Accept", "application/json")
	r, err := http.DefaultClient.Do(request)
	if err != nil {
		return res, fmt.Errorf("getting API token failed: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		return res, fmt.Errorf("getting API token failed: %d - %s", r.StatusCode, string(b))
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return res, fmt.Errorf("getting token response failed: %v", err)
	}
	result := OauthTokenResponse{}
	err = json.Unmarshal(b, &result)
	if err != nil {
		resultStrings := OauthTokenResponseStrings{}
		err = json.Unmarshal(b, &resultStrings)
		exp, _ := strconv.Atoi(resultStrings.ExpiresIn)
		exp2, _ := strconv.Atoi(resultStrings.RefreshTokenExpiresIn)
		result = OauthTokenResponse{
			AccessToken:           resultStrings.AccessToken,
			ExpiresIn:             exp,
			RefreshToken:          resultStrings.RefreshToken,
			RefreshTokenExpiresIn: exp2,
			Scope:                 resultStrings.Scope,
			TokenType:             resultStrings.TokenType,
		}
	}
	if err != nil {
		str := string(b)
		if str == "" {
			return res, fmt.Errorf("getting API token failed: response is empty")
		}
		params, err := url.ParseQuery(str)
		if err != nil {
			return res, fmt.Errorf("getting API token failed: %v", err)
		}
		exp, _ := strconv.Atoi(params.Get("expires_in"))
		exp2, _ := strconv.Atoi(params.Get("refresh_token_expires_in"))
		result = OauthTokenResponse{
			AccessToken:           params.Get("access_token"),
			ExpiresIn:             exp,
			RefreshToken:          params.Get("refresh_token"),
			RefreshTokenExpiresIn: exp2,
			Scope:                 params.Get("scope"),
			TokenType:             params.Get("token_type"),
		}
	}
	//TODO: store in cache, return key, don't use directly: retrieve from cache
	//return res, nil
	return TokenResponse{result.AccessToken}, nil
}

func encode(req OauthTokenRequest) *bytes.Buffer {
	s := fmt.Sprintf("code=%s&client_id=%s&client_secret=%s&redirect_uri=%s&grant_type=%s",
		url.QueryEscape(req.Code),
		url.QueryEscape(req.ClientId),
		url.QueryEscape(req.ClientSecret),
		url.QueryEscape(req.RedirectUri),
		url.QueryEscape(req.GrantType),
	)
	if req.Resource != "" {
		s = s + "&resource=" + url.QueryEscape(req.Resource)
	}
	return bytes.NewBuffer([]byte(s))
}
