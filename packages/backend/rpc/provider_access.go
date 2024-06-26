package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AccessTokenPostRequest struct {
	AppKey    string `json:"app_key"`
	Timestamp int64  `json:"timestamp"`
	Token     string `json:"token"`
	Perm      Perm   `json:"perm"`
}

type DataResponse struct {
	AccessToken string `json:"access_token"`
}

type SystemServerResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message,omitempty"`
	Data    DataResponse `json:"data"`
}

type Perm struct {
	Group    string   `json:"group"`
	DataType string   `json:"dataType"`
	Version  string   `json:"version"`
	Ops      []string `json:"ops"`
}

var OsSystemServer = os.Getenv("OS_SYSTEM_SERVER")
var OsAppSecret = os.Getenv("OS_APP_SECRET")
var OsAppKey = os.Getenv("OS_APP_KEY")

func getAccessToken(dataType, group string, ops []string) (string, error) {
	timestamp := time.Now().Unix()
	text := OsAppKey + strconv.FormatInt(timestamp, 10) + OsAppSecret
	fmt.Println(text)
	token, err := bcrypt.GenerateFromPassword([]byte(text), 10)
	if err != nil {
		return "", err
	}

	fmt.Println(string(token))

	accesTokenPostRequest := AccessTokenPostRequest{
		AppKey:    OsAppKey,
		Timestamp: timestamp,
		Token:     string(token),
		Perm: Perm{
			Group:    group,
			DataType: dataType,
			Version:  "v1",
			Ops:      ops,
		},
	}

	requestBytes, err := json.Marshal(accesTokenPostRequest)
	if err != nil {
		fmt.Println("marlshal error ", err)
		return "", err
	}

	fmt.Println(string(requestBytes))

	bodyReader := bytes.NewReader(requestBytes)
	requestUrl := "http://" + OsSystemServer + "/permission/v1alpha1/access"
	fmt.Println(requestUrl)
	req, err := http.NewRequest(http.MethodPost, requestUrl, bodyReader)
	if err != nil {
		fmt.Println("client: could not create request: ", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("Access-Control-Allow-Headers", "X-Requested-With,Content-Type")
	req.Header.Set("Access-Control-Allow-Methods", "PUT,POST,GET,DELETE,OPTIONS")

	fmt.Println(req)

	client := http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("client: error making http request: ", err)
		return "", err
	}
	fmt.Println("resp: ", resp)

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}
	defer resp.Body.Close()

	fmt.Println("resp body: ", resp.Body)

	var r SystemServerResponse
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		fmt.Println("decode error: ", err)
		return "", err
	}

	if r.Code != 0 {
		fmt.Println(r.Message)
		return "", errors.New(strconv.Itoa(r.Code) + " " + r.Message)
	}

	return r.Data.AccessToken, nil
}

func CallDifyGatewayBaseProvider(opType int, opData map[string]interface{}) ([]byte, error) {
	if KnowledgeBase != "True" {
		fmt.Println("KnowledgeBase is not functional")
		return nil, nil
	}

	accessToken, err := getAccessToken("gateway", "service.agent", []string{"DifyGatewayBaseProvider"})
	if err != nil {
		fmt.Println("get access token failed: ", err)
		return nil, err
	}

	requestData := map[string]interface{}{
		"op_type": opType,
		"op_data": opData,
	}

	requestBytes, err := json.Marshal(requestData)
	if err != nil {
		fmt.Println("Failed to marshal request data:", err.Error())
		return nil, err
	}

	requestJSON := string(requestBytes)
	fmt.Println(requestJSON)

	bodyReader := bytes.NewReader(requestBytes)
	requestUrl := "http://" + OsSystemServer + "/system-server/v1alpha1/gateway/service.agent/v1/DifyGatewayBaseProvider"

	fmt.Println(requestUrl)
	req, err := http.NewRequest(http.MethodPost, requestUrl, bodyReader)

	if err != nil {
		fmt.Println("client: could not create request: ", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("Access-Control-Allow-Headers", "X-Requested-With,Content-Type")
	req.Header.Set("Access-Control-Allow-Methods", "PUT,POST,GET,DELETE,OPTIONS")
	req.Header.Set("X-Access-Token", accessToken)

	client := http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("client: error making http request: ", err)
		return nil, err
	}
	fmt.Println(resp)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Failed to read response body")
		return nil, err
	}

	fmt.Println(string(body))

	if resp.StatusCode != http.StatusOK {
		fmt.Println("status code error: ", errors.New(resp.Status))
		return nil, errors.New(resp.Status)
	}
	defer resp.Body.Close()
	return body, nil
}
