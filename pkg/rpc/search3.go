package rpc

import (
	"bytes"
	"encoding/json"
	"files/pkg/postgres"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"os"
	"sync"
)

var SearchIdCache map[string]string = make(map[string]string)

var SearchIdCacheLock sync.RWMutex

type Response struct {
	StatusCode string `json:"status_code"`
	FailReason string `json:"fail_reason"`
	Data       *Data  `json:"data"`
}

type Data struct {
	ID              int                    `json:"id"`
	Title           string                 `json:"title"`
	TitleLanguage   string                 `json:"title_language"`
	Content         string                 `json:"content"`
	ContentLanguage string                 `json:"content_language"`
	Author          string                 `json:"author"`
	OwnerUserid     string                 `json:"owner_userid"`
	ResourceURI     string                 `json:"resource_uri"`
	CreatedAt       int64                  `json:"created_at"`
	Service         string                 `json:"service"`
	Meta            map[string]interface{} `json:"meta"`
}

func InitSearch3() {
	if postgres.DBServer != nil {
		recreate := os.Getenv("RECREATE_PATH_LIST")
		if recreate != "" {
			postgres.RecreateTable(postgres.DBServer, &postgres.PathList{})
		}
		postgres.InitDrivePathList()
	} else {
		klog.Info("no postgres server, no need to init path_list for search3")
	}
}

func fetchDocumentByResourceUri(resourceUri, bflName string) (string, string, error) {
	baseURL := "http://search3.os-system:80/document/get_by_resource_uri"
	query := url.Values{}
	query.Set("resource_uri", resourceUri)
	query.Set("service", "files")
	fullURL := baseURL + "?" + query.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bfl-user", bflName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		klog.Errorf("Error parsing JSON: %v", err)
	}

	var searchId string
	var md5 string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		searchId = fmt.Sprintf("%d", response.Data.ID)
		md5 = ""
	} else {
		searchId = ""
		md5 = ""
		klog.Infoln("Failed to retrieve data or data is nil.")
	}

	return searchId, md5, nil
}

func getSerachIdOrCache(resourceUri, bflName string, withMD5 bool) (string, string, error) {
	SearchIdCacheLock.Lock()
	defer SearchIdCacheLock.Unlock()

	var val string
	var ok bool

	if withMD5 == false {
		if val, ok = SearchIdCache[resourceUri]; ok {
			return val, "", nil
		}
	}

	searchId, md5, err := fetchDocumentByResourceUri(resourceUri, bflName)
	if err != nil {
		klog.Errorln("Error fetching document:", err)
		return "", "", nil
	}

	if withMD5 == true {
		val, ok = SearchIdCache[resourceUri]
	}
	if !ok && searchId != "" {
		SearchIdCache[resourceUri] = searchId
	}
	return searchId, md5, nil
}

func postDocumentSearch3(doc map[string]interface{}, bflName string) (string, error) {
	docBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		klog.Errorln("Error marshaling doc to JSON:", err)
		return "", err
	}

	url := "http://search3.os-system:80/document/add"
	jsonStr := bytes.NewBuffer(docBytes)

	req, err := http.NewRequest("POST", url, jsonStr)
	if err != nil {
		klog.Errorln("Error creating request:", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bfl-user", bflName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.Errorln("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorln("Error reading response body:", err)
		return "", err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		klog.Errorf("Error parsing JSON: %v", err)
	}

	var searchId string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		searchId = fmt.Sprintf("%d", response.Data.ID)
	} else {
		searchId = ""
		klog.Infoln("Failed to retrieve data or data is nil.")
	}

	SearchIdCacheLock.Lock()
	defer SearchIdCacheLock.Unlock()
	if searchId != "" {
		klog.Infoln("Search Id to key: ", doc["resource_uri"].(string))
		SearchIdCache[doc["resource_uri"].(string)] = searchId
	}
	return searchId, nil
}

func putDocumentSearch3(searchId string, doc map[string]interface{}, bflName string) (string, error) {
	docBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		klog.Errorln("Error marshaling doc to JSON:", err)
		return "", err
	}

	url := "http://search3.os-system:80/document/update/" + searchId
	jsonStr := bytes.NewBuffer(docBytes)

	req, err := http.NewRequest("PUT", url, jsonStr)
	if err != nil {
		klog.Errorln("Error creating request:", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bfl-user", bflName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.Errorln("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorln("Error reading response body:", err)
		return "", err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		klog.Errorf("Error parsing JSON: %v", err)
	}

	var respSearchId string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		respSearchId = fmt.Sprintf("%d", response.Data.ID)
	} else {
		respSearchId = ""
		klog.Infoln("Failed to retrieve data or data is nil.")
	}
	return respSearchId, nil
}

func deleteDocumentSearch3(searchId string, bflName string) (string, error) {
	url := "http://search3.os-system:80/document/delete/" + searchId

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		klog.Errorln("Error creating request:", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bfl-user", bflName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		klog.Errorln("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorln("Error reading response body:", err)
		return "", err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		klog.Errorf("Error parsing JSON: %v", err)
	}

	var respSearchId string
	var respResourceUri string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		respSearchId = fmt.Sprintf("%d", response.Data.ID)
		respResourceUri = fmt.Sprintf("%d", response.Data.ResourceURI)
	} else {
		respSearchId = ""
		klog.Infoln("Failed to retrieve data or data is nil.")
	}

	SearchIdCacheLock.Lock()
	defer SearchIdCacheLock.Unlock()
	if searchId != "" {
		klog.Infoln("Search Id to key: ", respResourceUri)
		delete(SearchIdCache, respResourceUri)
	}
	return respSearchId, nil
}
