package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"integration/app/dv"
	"integration/app/logging"
	"integration/app/tree"
	"io"
	"io/ioutil"
	"net/http"
)

func GetNodeMap(persistentId, token string) (map[string]tree.Node, error) {
	url := dataverseServer + "/api/datasets/:persistentId/versions/:latest/files?persistentId=" + persistentId
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("X-Dataverse-key", token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	res := dv.ListResponse{}
	err = json.Unmarshal(responseData, &res)
	if err != nil {
		return nil, err
	}
	if res.Status != "OK" {
		return nil, fmt.Errorf("listing files for %s failed: %+v", persistentId, res)
	}
	return mapToNodes(res.Data), nil
}

func mapToNodes(data []tree.Metadata) map[string]tree.Node {
	res := map[string]tree.Node{}
	for _, d := range data {
		dir := ""
		if d.DirectoryLabel != "" {
			dir = d.DirectoryLabel + "/"
		}
		id := dir + d.DataFile.Filename
		res[id] = tree.Node{
			Id:   id,
			Name: d.DataFile.Filename,
			Path: d.DirectoryLabel,
			Attributes: tree.Attributes{
				ParentId:  d.DirectoryLabel,
				Metadata:  d,
				IsFile:    true,
				LocalHash: d.DataFile.Checksum.Value,
			},
		}
	}
	return res
}

func doWork(job Job) (Job, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-Stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	if job.StreamType == "hash-only" {
		return doRehash(ctx, job.DataverseKey, job.PersistentId, job.WritableNodes, job)
	}
	streams, err := deserialize(ctx, job.StreamType, job.Streams, job.StreamParams)
	if err != nil {
		return job, err
	}
	knownHashes := getKnownHashes(job.PersistentId)
	//filter not valid actions (when someone had browser open for a very long time and other job started and finished)
	writableNodes, err := filterRedundant(job, knownHashes)
	if err != nil {
		return job, err
	}
	job.WritableNodes = writableNodes
	return doPersistNodeMap(ctx, streams, job, knownHashes)
}

func filterRedundant(job Job, knownHashes map[string]calculatedHashes) (map[string]tree.Node, error) {
	filteredEqual := map[string]tree.Node{}
	isDelete := false
	for k, v := range job.WritableNodes {
		h, ok := knownHashes[k].RemoteHashes[v.Attributes.RemoteHashType]
		if v.Action == tree.Delete {
			isDelete = true
		} else if ok && h == v.Attributes.RemoteHash {
			continue
		}
		filteredEqual[k] = v
	}
	if !isDelete {
		return filteredEqual, nil
	}
	res := map[string]tree.Node{}
	nm, err := GetNodeMap(job.PersistentId, job.DataverseKey)
	if err != nil {
		return nil, err
	}
	for k, v := range filteredEqual {
		_, ok := nm[k]
		if v.Action == tree.Delete && ok {
			continue
		}
		res[k] = v
	}
	return res, nil
}

func doPersistNodeMap(ctx context.Context, streams map[string]stream, in Job, knownHashes map[string]calculatedHashes) (out Job, err error) {
	dataverseKey, persistentId, writableNodes := in.DataverseKey, in.PersistentId, in.WritableNodes
	err = CheckPermission(dataverseKey, persistentId)
	if err != nil {
		return
	}
	defer func() {
		storeKnownHashes(persistentId, knownHashes)
	}()
	out = in
	i := 0
	total := len(writableNodes)
	for k, v := range writableNodes {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		default:
		}
		i++
		if i%10 == 0 && i < total {
			storeKnownHashes(persistentId, knownHashes) //if we have many files to hash -> polling at the gui is happier to see some progress
		}

		if v.Action == tree.Delete {
			err = deleteFromDV(dataverseKey, v.Attributes.Metadata.DataFile.Id)
			if err != nil {
				return
			}
			delete(knownHashes, v.Id)
			delete(out.WritableNodes, k)
			continue
		}
		fileStream := streams[k]
		fileName := generateFileName()
		storageIdentifier := generateStorageIdentifier(fileName)
		hashType := defaultHash
		remoteHashType := v.Attributes.RemoteHashType
		var h []byte
		var remoteH []byte
		h, remoteH, err = write(ctx, fileStream, storageIdentifier, persistentId, hashType, remoteHashType, v.Attributes.Metadata.DataFile.Filesize)
		if err != nil {
			return
		}
		hashValue := fmt.Sprintf("%x", h)
		//updated or new: always rehash
		knownHashes[v.Id] = calculatedHashes{
			LocalHashType:  hashType,
			LocalHashValue: hashValue,
			RemoteHashes:   map[string]string{remoteHashType: fmt.Sprintf("%x", remoteH)},
		}
		if v.Attributes.Metadata.DataFile.Id != 0 {
			err = deleteFromDV(dataverseKey, v.Attributes.Metadata.DataFile.Id)
			if err != nil {
				return
			}
		}
		directoryLabel := &(v.Attributes.Metadata.DirectoryLabel)
		if *directoryLabel == "" {
			directoryLabel = nil
		}
		data := dv.JsonData{
			StorageIdentifier: storageIdentifier,
			FileName:          v.Attributes.Metadata.DataFile.Filename,
			DirectoryLabel:    directoryLabel,
			MimeType:          "application/octet-stream",
			Checksum: dv.Checksum{
				Type:  hashType,
				Value: hashValue,
			},
		}
		err = writeToDV(dataverseKey, persistentId, data)
		if err != nil {
			return
		}
		delete(out.WritableNodes, k)
	}
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
		err = cleanup(in.DataverseKey, in.PersistentId)
	}
	return
}

func deleteFromDV(dataverseKey string, id int) error {
	url := fmt.Sprintf("%s/dvn/api/data-deposit/v1.1/swordv2/edit-media/file/%d", dataverseServer, id)
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		logging.Logger.Println(err)
		return err
	}
	request.SetBasicAuth(dataverseKey, "")
	r, err := http.DefaultClient.Do(request)
	if r.StatusCode != 200 && r.StatusCode != 202 && r.StatusCode != 204 {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("deleting file %d failed: %d - %s", id, r.StatusCode, string(b))
	}
	return err
}

func writeToDV(dataverseKey, persistentId string, jsonData dv.JsonData) error {
	url := dataverseServer + "/api/datasets/:persistentId/addFiles?persistentId=" + persistentId
	data, err := json.Marshal([]dv.JsonData{jsonData})
	if err != nil {
		return err
	}
	body, formDataContentType, err := dv.RequestBody(data)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", formDataContentType)
	request.Header.Add("X-Dataverse-key", dataverseKey)
	r, err := http.DefaultClient.Do(request)
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("writing file in %s failed: %d - %s", persistentId, r.StatusCode, string(b))
	}
	b, _ := io.ReadAll(r.Body)
	res := dv.AddFilesResponse{}
	err = json.Unmarshal(b, &res)
	if res.Data.Result.Added != 1 && len(res.Data.Files) == 1 {
		return fmt.Errorf(res.Data.Files[0].ErrorMessage)
	}
	return err
}

func CheckPermission(dataverseKey, persistentId string) error {
	url := fmt.Sprintf("%s/api/admin/permissions/:persistentId?persistentId=%s&unblock-key=%s", dataverseServer, persistentId, unblockKey)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	request.Header.Add("X-Dataverse-key", dataverseKey)
	r, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("getting permissions for dataset %s failed: %s", persistentId, string(b))
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	res := dv.Permissions{}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}
	if res.Status != "OK" {
		return fmt.Errorf("permission check status is %s for dataset %s", res.Status, persistentId)
	}
	for _, v := range res.Data.Permissions {
		if v == "EditDataset" {
			return nil
		}
	}
	return fmt.Errorf("user %v has no permission to edit dataset %v", res.Data.User, persistentId)
}

func getUser(dataverseKey string) (dv.User, error) {
	url := fmt.Sprintf("%s/api/users/:me", dataverseServer)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return dv.User{}, err
	}
	request.Header.Add("X-Dataverse-key", dataverseKey)
	r, err := http.DefaultClient.Do(request)
	if err != nil {
		return dv.User{}, err
	}
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		return dv.User{}, fmt.Errorf("getting user failed: %s", string(b))
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return dv.User{}, err
	}
	res := dv.User{}
	err = json.Unmarshal(b, &res)
	return res, err
}

func CreateNewDataset(dataverseKey string) (string, error) {
	user, err := getUser(dataverseKey)
	if err != nil {
		return "", err
	}
	body := dv.CreateDatasetRequestBody(user)
	url := dataverseServer + "/api/dataverses/" + defaultDataverse + "/datasets?doNotValidate=true"
	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Dataverse-key", dataverseKey)
	r, err := http.DefaultClient.Do(request)
	if r.StatusCode != 201 {
		b, _ := io.ReadAll(r.Body)
		return "", fmt.Errorf("creating dataset failed (%v): %s", r.StatusCode, string(b))
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	res := dv.CreateNewDatasetResponse{}
	err = json.Unmarshal(b, &res)
	return res.Data.PersistentId, err
}

func cleanup(token, persistentId string) error {
	url := dataverseServer + "/api/datasets/:persistentId/cleanStorage?persistentId=" + persistentId
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	request.Header.Add("X-Dataverse-key", token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	res := dv.CleanupResponse{}
	err = json.Unmarshal(responseData, &res)
	if err != nil {
		return err
	}
	if res.Status != "OK" {
		return fmt.Errorf("cleaning up files for %s failed: %+v", persistentId, res)
	}
	logging.Logger.Println(res.Data.Message)
	return nil
}
