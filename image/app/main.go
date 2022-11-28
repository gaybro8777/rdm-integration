package main

import (
	"crypto/tls"
	"fmt"
	"integration/app/common"
	"integration/app/gh"
	"integration/app/gl"
	"integration/app/logging"
	"integration/app/utils"
	"integration/app/workers/spinner"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func main() {
	// allow bad certificates
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// serve api
	// when adding compare to a new repository type, do not forget to add corresponding stream in utils/streams.go
	// github
	http.HandleFunc("/api/github/compare", gh.GithubCompare)
	// gitlab
	http.HandleFunc("/api/gitlab/compare", gl.GitlabCompare)

	//common
	http.HandleFunc("/api/common/newdataset", common.NewDataset)
	http.HandleFunc("/api/common/compare", common.Compare)
	http.HandleFunc("/api/common/cached", common.GetCachedResponse)
	http.HandleFunc("/api/common/store", common.Store)

	// serve html
	fs := http.FileServer(http.Dir(utils.FileServerPath))
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/connect") {
			r.URL.Path = "/"
		}
		fs.ServeHTTP(w, r)
	}))

	numberWorkers := 0
	var err error
	if len(os.Args) > 1 {
		numberWorkers, err = strconv.Atoi(os.Args[1])
		if err != nil {
			panic(fmt.Errorf("failed to parse number of workers from %v: %v", numberWorkers, err))
		}
	}
	if numberWorkers > 0 {
		logging.Logger.Println("nuber workers:", numberWorkers)
		go http.ListenAndServe(":7788", nil)
		spinner.SpinWorkers(numberWorkers)
	} else {
		logging.Logger.Println("http server only")
		http.ListenAndServe(":7788", nil)
	}
}
