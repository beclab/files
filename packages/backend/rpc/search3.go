package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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
	Meta            map[string]interface{} `json:"meta"` // 使用map来存储未知字段
}

// maybe need in the future
//type Meta struct {
//	MD5         string    `json:"md5"`
//	Size        int64     `json:"size"`
//	Created     int64     `json:"created"`
//	Updated     int64     `json:"updated"`
//	FormatName  string    `json:"format_name"`
//}

func fetchDocumentByResourceUri(resourceUri, bflName string) (string, string, error) {
	// maybe need in the future
	// escapedResourceUri := url.QueryEscape(resourceUri)

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
		log.Fatalf("Error parsing JSON: %v", err)
	}

	var searchId string
	var md5 string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		searchId = fmt.Sprintf("%d", response.Data.ID)
		//md5 = fmt.Sprintf("%d", response.Data.Meta.MD5)
		//for key, value := range response.Data.Meta {
		//	fmt.Printf("Meta[%s] = %v\n", key, value)
		//	if key == "md5" {
		//		md5 = value.(string)
		//	}
		//}
		md5 = ""
	} else {
		searchId = ""
		md5 = ""
		log.Println("Failed to retrieve data or data is nil.")
	}

	//fmt.Println("searchId: ", searchId, "; md5: ", md5)

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
		fmt.Println("Error fetching document:", err)
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
		fmt.Println("Error marshaling doc to JSON:", err)
		return "", err
	}
	//fmt.Println("Doc content:")
	//fmt.Println(string(docBytes))

	url := "http://search3.os-system:80/document/add"
	jsonStr := bytes.NewBuffer(docBytes)

	req, err := http.NewRequest("POST", url, jsonStr)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bfl-user", bflName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "", err
	}

	//fmt.Println("Response from server:")
	//fmt.Println(string(body))

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	var searchId string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		searchId = fmt.Sprintf("%d", response.Data.ID)
		//for key, value := range response.Data.Meta {
		//	fmt.Printf("Meta[%s] = %v\n", key, value)
		//}
	} else {
		searchId = ""
		log.Println("Failed to retrieve data or data is nil.")
	}

	//fmt.Println("searchId:", searchId)

	SearchIdCacheLock.Lock()
	defer SearchIdCacheLock.Unlock()
	if searchId != "" {
		fmt.Println("Search Id to key: ", doc["resource_uri"].(string))
		SearchIdCache[doc["resource_uri"].(string)] = searchId
	}
	return searchId, nil
}

func putDocumentSearch3(searchId string, doc map[string]interface{}, bflName string) (string, error) {
	docBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling doc to JSON:", err)
		return "", err
	}
	//fmt.Println("Doc content:")
	//fmt.Println(string(docBytes))

	url := "http://search3.os-system:80/document/update/" + searchId
	jsonStr := bytes.NewBuffer(docBytes)

	req, err := http.NewRequest("PUT", url, jsonStr)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bfl-user", bflName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "", err
	}

	//fmt.Println("Response from server:")
	//fmt.Println(string(body))

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	var respSearchId string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		respSearchId = fmt.Sprintf("%d", response.Data.ID)
		//for key, value := range response.Data.Meta {
		//	fmt.Printf("Meta[%s] = %v\n", key, value)
		//}
	} else {
		respSearchId = ""
		log.Println("Failed to retrieve data or data is nil.")
	}

	//fmt.Println("searchId:", respSearchId)

	//SearchIdCacheLock.Lock()
	//defer SearchIdCacheLock.Unlock()
	//if searchId != "" {
	//	fmt.Println("Search Id to key: ", doc["resource_uri"].(string))
	//	SearchIdCache[doc["resource_uri"].(string)] = searchId
	//}
	return respSearchId, nil
}

func deleteDocumentSearch3(searchId string, bflName string) (string, error) {
	url := "http://search3.os-system:80/document/delete/" + searchId

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bfl-user", bflName)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "", err
	}

	//fmt.Println("Response from server:")
	//fmt.Println(string(body))

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	var respSearchId string
	var respResourceUri string
	if response.StatusCode == "SUCCESS" && response.Data != nil {
		respSearchId = fmt.Sprintf("%d", response.Data.ID)
		respResourceUri = fmt.Sprintf("%d", response.Data.ResourceURI)
		//for key, value := range response.Data.Meta {
		//	fmt.Printf("Meta[%s] = %v\n", key, value)
		//}
	} else {
		respSearchId = ""
		log.Println("Failed to retrieve data or data is nil.")
	}

	//fmt.Println("searchId:", respSearchId)

	SearchIdCacheLock.Lock()
	defer SearchIdCacheLock.Unlock()
	if searchId != "" {
		fmt.Println("Search Id to key: ", respResourceUri)
		//SearchIdCache[respResourceUri] = searchId
		delete(SearchIdCache, respResourceUri)
	}
	return respSearchId, nil
}
