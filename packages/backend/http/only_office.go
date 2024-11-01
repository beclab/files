package http

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func getOnlyOfficeJwt(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	bflName := r.Header.Get("X-Bfl-User")
	if bflName == "" {
		return nil, os.ErrPermission
	}

	dstUrl := "http://jwtgeter.onlyoffice-wangrongxiang2:3030/onlyoffice/jwt"

	fmt.Println("dstUrl:", dstUrl)

	var req *http.Request
	var err error
	req, err = http.NewRequest("GET", dstUrl, nil)

	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}
	
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	//for name, values := range req.Header {
	//	for _, value := range values {
	//		fmt.Printf("%s: %s\n", name, value)
	//	}
	//}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	//fmt.Printf("GoogleDriveListResponse Hedears:\n")
	//for name, values := range resp.Header {
	//	for _, value := range values {
	//		fmt.Printf("%s: %s\n", name, value)
	//	}
	//}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		fmt.Println("GoogleDrive Call Response is not JSON format:", contentType)
	}

	var body []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("Error creating gzip reader:", err)
			return nil, err
		}
		defer reader.Close()

		body, err = ioutil.ReadAll(reader)
		if err != nil {
			fmt.Println("Error reading gzipped response body:", err)
			return nil, err
		}
	} else {
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return nil, err
		}
	}

	var datas map[string]interface{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		fmt.Println("Error unmarshaling JSON response:", err)
		return nil, err
	}

	fmt.Println("Parsed JSON response:", datas)
	responseText, err := json.MarshalIndent(datas, "", "  ")
	if err != nil {
		http.Error(w, "Error marshaling JSON response to text: "+err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	return responseText, nil
	//w.Header().Set("Content-Type", "application/json; charset=utf-8")
	//w.Write([]byte(responseText))
	//return nil, nil
}

var resourceOnlyOfficeJwtHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	respJson, err := getOnlyOfficeJwt(w, r)
	if err != nil {
		return errToStatus(err), err
	}

	return renderJSON(w, r, respJson)
	//return http.StatusOK, nil
})
