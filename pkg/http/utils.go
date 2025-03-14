package http

import (
	"encoding/json"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"strings"
)

// This is an addaptation if http.StripPrefix in which we don't
// return 404 if the page doesn't have the needed prefix.
func stripPrefix(prefix string, h http.Handler) http.Handler {
	if prefix == "" || prefix == "/" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		rp := strings.TrimPrefix(r.URL.RawPath, prefix)
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p
		r2.URL.RawPath = rp
		h.ServeHTTP(w, r2)
	})
}

func getOwner(r *http.Request) (ownerID, ownerName string) {
	bflName := r.Header.Get("X-Bfl-User")
	url := "http://bfl.user-space-" + bflName + "/bfl/info/v1/terminus-info"

	resp, err := http.Get(url)
	if err != nil {
		klog.Errorln("Error making GET request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Errorln("Error reading response body:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		klog.Infof("Received non-200 response: %d\n", resp.StatusCode)
		return
	}

	type BflResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TerminusName    string `json:"terminusName"`
			WizardStatus    string `json:"wizardStatus"`
			Selfhosted      bool   `json:"selfhosted"`
			TailScaleEnable bool   `json:"tailScaleEnable"`
			OsVersion       string `json:"osVersion"`
			LoginBackground string `json:"loginBackground"`
			Avatar          string `json:"avatar"`
			TerminusId      string `json:"terminusId"`
			Did             string `json:"did"`
			ReverseProxy    string `json:"reverseProxy"`
			Terminusd       string `json:"terminusd"`
		} `json:"data"`
	}

	var responseObj BflResponse
	err = json.Unmarshal(body, &responseObj)
	if err != nil {
		klog.Errorln("Error unmarshaling JSON:", err)
		return
	}

	ownerID = responseObj.Data.TerminusId
	ownerName = responseObj.Data.TerminusName
	return
}
