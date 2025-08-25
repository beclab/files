package handle_func

import (
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/global"
	"files/pkg/models"
	"net/http"

	"k8s.io/klog/v2"
)

type commonFunc func(contextQueryArgs *models.QueryParam) ([]byte, error)

func CommonHandle(fn commonFunc) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var path = r.URL.Path
		var owner = r.Header.Get(common.REQUEST_HEADER_OWNER)
		if owner == "" {
			http.Error(w, "user not found", http.StatusBadRequest)
			return
		}

		var contextQueryArgs = models.CreateQueryParam(owner, r, false, false)

		klog.Infof("Incoming Path: %s, user: %s, method: %s", path, owner, r.Method)

		res, err := fn(contextQueryArgs)
		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		w.Write(res)
		return
	})

	return handler
}

// health
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

/**
 * get repos
 */
func ReposGetHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	if contextQueryArgs.ShareType == "shared" || contextQueryArgs.ShareType == "share_to_me" {
		repos := map[string][]string{
			"repos": {},
		}
		return common.ToBytes(repos), nil
	}

	var owner = contextQueryArgs.Owner

	repos, err := seahub.HandleReposGet(owner, []string{"mine"})
	if err != nil {
		klog.Errorf("get repos error: %v", err)
		return nil, err
	}
	klog.Infof("get repos: %s", string(repos))
	return repos, nil
}

/**
 * create new repo
 */

func CreateRepoHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var owner = contextQueryArgs.Owner
	var repoName = contextQueryArgs.RepoName

	if repoName == "" {
		return nil, errors.New("repo name is empty")
	}

	klog.Infof("Repo create repo, user: %s, name: %s", owner, repoName)

	var res, err = seahub.HandleRepoPost(owner, repoName, "")
	if err != nil {
		klog.Errorf("create repo error: %v, name: %s", err, repoName)
		return nil, err
	}

	klog.Infof("Repo create success, user: %s, name: %s, result: %s", owner, repoName, string(res))

	return nil, nil
}

/**
 * delete repo
 */
func DeleteRepoHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var owner = contextQueryArgs.Owner
	var repoId = contextQueryArgs.RepoId
	if repoId == "" {
		return nil, errors.New("repo id is empty")
	}

	klog.Infof("Repo delete repo, user: %s, id: %s", owner, repoId)

	var res, err = seahub.HandleRepoDelete(owner, repoId)
	if err != nil {
		klog.Errorf("delete repo error: %v, name: %s", err, repoId)
		return nil, err
	}

	klog.Infof("Repo delete success, user: %s, repo id: %s, result: %s", owner, repoId, string(res))

	return nil, nil
}

/**
 * rename repo
 */
func RenameRepoHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var user = contextQueryArgs.Owner
	var repoId = contextQueryArgs.RepoId
	var repoName = contextQueryArgs.Destination

	if repoId == "" {
		return nil, errors.New("repo id is empty")
	}

	if repoName == "" {
		return nil, errors.New("repo name is empty")
	}

	repoName, err := common.UnescapeURLIfEscaped(repoName)
	if err != nil {
		return nil, err
	}

	klog.Infof("Repo rename repo, user: %s, id: %s, name: %s", user, repoId, repoName)

	res, err := seahub.HandleRepoPatch(user, repoId, repoName, "", "rename")
	if err != nil {
		klog.Errorf("rename repo error: %v, id: %s, name: %s", err, repoId, repoName)
		return nil, err
	}

	klog.Infof("Repo rename success, user: %s, repo id: %s, repo name: %s, result: %s", user, repoId, repoName, string(res))

	return nil, nil

}

/**
 * get nodes
 */
func NodesGetHandler(contextQueryArgs *models.QueryParam) ([]byte, error) {
	var nodes = global.GlobalNode.GetNodes()

	var data = make(map[string]interface{})
	data["nodes"] = nodes
	data["currentNode"] = common.NodeName

	var result = make(map[string]interface{})
	result["code"] = http.StatusOK
	result["data"] = data

	res, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return res, nil
}
