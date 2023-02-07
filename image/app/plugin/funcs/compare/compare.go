// Author: Eryk Kulikowski @ KU Leuven (2023). Apache 2.0 License

package compare

import (
	"context"
	"encoding/json"
	"fmt"
	"integration/app/common"
	"integration/app/plugin"
	"integration/app/plugin/types"
	"integration/app/tree"
	"integration/app/utils"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func Compare(w http.ResponseWriter, r *http.Request) {
	if !utils.RedisReady(r.Context()) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - cache not ready"))
		return
	}
	//process request
	req := types.CompareRequest{}
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
	key := uuid.New().String()
	go doCompare(req, key)
	res := common.Key{Key: key}
	b, err = json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - %v", err)))
		return
	}
	w.Write(b)
}

func doCompare(req types.CompareRequest, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cachedRes := common.CachedResponse{
		Key: key,
	}
	//check permission
	err := utils.CheckPermission(ctx, req.DataverseKey, req.PersistentId)
	if err != nil {
		cachedRes.ErrorMessage = err.Error()
		common.CacheResponse(cachedRes)
		return
	}

	//query dataverse
	nm, err := utils.GetNodeMap(ctx, req.PersistentId, req.DataverseKey)
	if err != nil {
		cachedRes.ErrorMessage = err.Error()
		common.CacheResponse(cachedRes)
		return
	}

	//query repository
	nmCopy := map[string]tree.Node{}
	for k, v := range nm {
		nmCopy[k] = v
	}
	repoNm, err := plugin.GetPlugin(req.Plugin).Query(ctx, req, nmCopy)
	if err != nil {
		cachedRes.ErrorMessage = err.Error()
		common.CacheResponse(cachedRes)
		return
	}
	tooLarge := []string{}
	maxFileSize := utils.GetMaxFileSize()
	for k, v := range repoNm {
		if maxFileSize > 0 && v.Attributes.Metadata.DataFile.Filesize > maxFileSize {
			delete(repoNm, k)
			tooLarge = append(tooLarge, v.Id)
		}
	}
	nm = utils.MergeNodeMaps(nm, repoNm)

	//compare and write response
	res := utils.Compare(ctx, nm, req.PersistentId, req.DataverseKey, true)
	if err != nil {
		cachedRes.ErrorMessage = err.Error()
		common.CacheResponse(cachedRes)
		return
	}

	cachedRes.Response = res
	cachedRes.Response.MaxFileSize = maxFileSize
	cachedRes.Response.TooLarge = tooLarge
	common.CacheResponse(cachedRes)
}
