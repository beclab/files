package seafile

import (
	"files/pkg/drivers/sync/seahub/searpc"
)

type NamedPipeClient = searpc.NamedPipeClient
type SearpcClient = searpc.SearpcClient

var CreateRPCMethod = searpc.CreateRPCMethod

type SeafServerThreadedRpcClient struct {
	*NamedPipeClient
}

func NewSeafServerClient(pipePath string) *SeafServerThreadedRpcClient {
	return &SeafServerThreadedRpcClient{
		NamedPipeClient: searpc.NewNamedPipeClient(pipePath, "seafserv-threaded-rpcserver", 5),
	}
}

func (c *SeafServerThreadedRpcClient) SeafileCreateRepo(name, desc, ownerEmail string, passwd *string, encVersion int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_create_repo", "string", []string{"string", "string", "string", "string", "int"})(
		name, desc, ownerEmail, passwd, encVersion)
}

func (c *SeafServerThreadedRpcClient) GetOwnedRepoList(username string, retCorrupted int, start int, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_list_owned_repos", "objlist", []string{"string", "int", "int", "int"})(
		username, retCorrupted, start, limit)
}

func (c *SeafServerThreadedRpcClient) SeafileCreateEncRepo(repoId, name, desc, ownerEmail, magic, key, salt string, encVersion int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_create_enc_repo", "string", []string{"string", "string", "string", "string", "string", "string", "string", "int"})(
		repoId, name, desc, ownerEmail, magic, key, salt, encVersion)
}

func (c *SeafServerThreadedRpcClient) SeafileGetReposByIdPrefix(idPrefix string, start, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_repos_by_id_prefix", "objlist", []string{"string", "int", "int"})(
		idPrefix, start, limit)
}

func (c *SeafServerThreadedRpcClient) SeafileGetRepo(repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_repo", "object", []string{"string"})(repoId)
}

func (c *SeafServerThreadedRpcClient) SeafileDestroyRepo(repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_destroy_repo", "int", []string{"string"})(repoId)
}

func (c *SeafServerThreadedRpcClient) SeafileGetRepoOwner(repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_repo_owner", "string", []string{"string"})(repoId)
}

func (c *SeafServerThreadedRpcClient) DeleteRepoTokensByEmail(email string) (interface{}, error) {
	return CreateRPCMethod(c, "delete_repo_tokens_by_email", "int", []string{"string"})(email)
}

func (c *SeafServerThreadedRpcClient) GetSystemDefaultRepoId() (interface{}, error) {
	return CreateRPCMethod(c, "get_system_default_repo_id", "string", []string{})()
}

func (c *SeafServerThreadedRpcClient) SeafileGetDirIdByPath(repoId, path string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_dir_id_by_path", "string", []string{"string", "string"})(
		repoId, path)
}

func (c *SeafServerThreadedRpcClient) SeafileListDir(repoId, dirId string, offset, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_list_dir", "objlist", []string{"string", "string", "int", "int"})(
		repoId, dirId, offset, limit)
}

func (c *SeafServerThreadedRpcClient) ListDirWithPerm(repoId, dirPath, dirId, user string, offset, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "list_dir_with_perm", "objlist", []string{"string", "string", "string", "string", "int", "int"})(
		repoId, dirPath, dirId, user, offset, limit)
}

func (c *SeafServerThreadedRpcClient) SeafileCopyFile(srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename, user string,
	needProgress, synchronous int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_copy_file", "object", []string{"string", "string", "string", "string", "string", "string", "string", "int", "int"})(
		srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename, user, needProgress, synchronous)
}

func (c *SeafServerThreadedRpcClient) SeafileRemoveShare(repoId, fromEmail, toEmail string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_remove_share", "int", []string{"string", "string", "string"})(
		repoId, fromEmail, toEmail)
}

func (c *SeafServerThreadedRpcClient) SeafileListShareRepos(email, queryCol string, start, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_list_share_repos", "objlist", []string{"string", "string", "int", "int"})(
		email, queryCol, start, limit)
}

func (c *SeafServerThreadedRpcClient) IsInnerPubRepo(repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "is_inner_pub_repo", "int", []string{"string"})(repoId)
}

func (c *SeafServerThreadedRpcClient) CheckQuota(repoId string, delta int64) (interface{}, error) {
	return CreateRPCMethod(c, "check_quota", "int", []string{"string", "int64"})(repoId, delta)
}

func (c *SeafServerThreadedRpcClient) CheckPermissionByPath(repoId, path, user string) (interface{}, error) {
	return CreateRPCMethod(c, "check_permission_by_path", "string", []string{"string", "string", "string"})(
		repoId, path, user)
}

func (c *SeafServerThreadedRpcClient) RepoHasBeenShared(repoId string, includingGroups int) (interface{}, error) {
	return CreateRPCMethod(c, "repo_has_been_shared", "int", []string{"string", "int"})(
		repoId, includingGroups)
}

func (c *SeafServerThreadedRpcClient) GetRepoStatus(repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "get_repo_status", "int", []string{"string"})(repoId)
}

func (c *SeafServerThreadedRpcClient) SeafileIsPasswdSet(repoId, user string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_is_passwd_set", "int", []string{"string", "string"})(repoId, user)
}

func (c *SeafServerThreadedRpcClient) PublishEvent(channel, content string) (interface{}, error) {
	return CreateRPCMethod(c, "publish_event", "int", []string{"string", "string"})(channel, content)
}

func (c *SeafServerThreadedRpcClient) AddEmailuser(email string, passwd string, isStaff int, isActive int) (interface{}, error) {
	return CreateRPCMethod(c, "add_emailuser", "int", []string{"string", "string", "int", "int"})(
		email, passwd, isStaff, isActive)
}

func (c *SeafServerThreadedRpcClient) RemoveEmailuser(source, email string) (interface{}, error) {
	return CreateRPCMethod(c, "remove_emailuser", "int", []string{"string", "string"})(
		source, email)
}

func (c *SeafServerThreadedRpcClient) GetEmailuser(email string) (interface{}, error) {
	return CreateRPCMethod(c, "get_emailuser", "object", []string{"string"})(email)
}

func (c *SeafServerThreadedRpcClient) GetEmailusers(source string, start int, limit int, status string) (interface{}, error) {
	return CreateRPCMethod(c, "get_emailusers", "objlist", []string{"string", "int", "int", "string"})(
		source, start, limit, status)
}

func (c *SeafServerThreadedRpcClient) CountEmailusers(source string) (interface{}, error) {
	return CreateRPCMethod(c, "count_emailusers", "int64", []string{"string"})(
		source)
}

func (c *SeafServerThreadedRpcClient) CountInactiveEmailusers(source string) (interface{}, error) {
	return CreateRPCMethod(c, "count_inactive_emailusers", "int64", []string{"string"})(
		source)
}

func (c *SeafServerThreadedRpcClient) UpdateEmailuser(source string, userId int, password string, isStaff int, isActive int) (interface{}, error) {
	return CreateRPCMethod(c, "update_emailuser", "int", []string{"string", "int", "string", "int", "int"})(
		source, userId, password, isStaff, isActive)
}

func (c *SeafServerThreadedRpcClient) RemoveGroupUser(username string) (interface{}, error) {
	return CreateRPCMethod(c, "remove_group_user", "int", []string{"string"})(username)
}
