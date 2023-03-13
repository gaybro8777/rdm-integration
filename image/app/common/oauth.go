// Author: Eryk Kulikowski @ KU Leuven (2023). Apache 2.0 License

package common

import (
	"encoding/json"
	"fmt"
	"integration/app/core"
	"io"
	"net/http"
)

type OauthTokenRequest struct {
	PluginId string `json:"pluginId"`
	Code     string `json:"code"`
	Nounce   string `json:"nounce"`
}

func GetOauthToken(w http.ResponseWriter, r *http.Request) {
	req := OauthTokenRequest{}
	b, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - %v", err)))
		return
	}
	err = json.Unmarshal(b, &req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - %v", err)))
		return
	}

	sessionId := core.GetShibSessionFromHeader(r.Header)
	res, err := core.GetOauthToken(r.Context(), req.PluginId, req.Code, req.Nounce, sessionId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - %v", err)))
		return
	}

	b, err = json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - %v", err)))
		return
	}
	w.Write(b)
}
