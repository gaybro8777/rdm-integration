package utils

import "os"

var pathToFilesDir = "../../rdm-deployment/data/dv/files/"
var dataverseServer = "https://localhost:7000"
var defaultDriver = "file"
var awsRegion = "libis-cloud"
var awsEndpoint = "http://s3.libis.cloud"
var awsPathstyle = true
var awsBucket = "dataverse"
var defaultHash = Md5

func init() {
	files := os.Getenv("FILES_PATH")
	server := os.Getenv("DATAVERSE_SERVER")
	driver := os.Getenv("STORAGE_DRIVER")
	region := os.Getenv("AWS_REGION")
	endpoint := os.Getenv("AWS_ENDPOINT")
	style := os.Getenv("AWS_PATH_STYLE_ACCESS")
	bucket := os.Getenv("AWS_BUCKET")
	hash := os.Getenv("HASH_TYPE")
	// Environment variables used for credentials:
	// * Access Key ID:     AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY
	// * Secret Access Key: AWS_SECRET_ACCESS_KEY or AWS_SECRET_KEY
	if files != "" {
		pathToFilesDir = files
	}
	if server != "" {
		dataverseServer = server
	}
	if driver != "" {
		defaultDriver = driver
	}
	if region != "" {
		awsRegion = region
	}
	if endpoint != "" {
		awsEndpoint = endpoint
	}
	if style != "" {
		awsPathstyle = style == "true" || style == "TRUE" || style == "\"TRUE\"" || style == "\"true\""
	}
	if bucket != "" {
		awsBucket = bucket
	}
	if hash != "" {
		defaultHash = hash
	}
}
