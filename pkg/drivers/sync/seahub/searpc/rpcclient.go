package searpc

//type NamedPipeClient = NamedPipeClient
//type SearpcClient = SearpcClient
//
//var CreateRPCMethod = CreateRPCMethod

type SeafServerThreadedRpcClient struct {
	*NamedPipeClient
}

func NewSeafServerClient(pipePath string) *SeafServerThreadedRpcClient {
	return &SeafServerThreadedRpcClient{
		NamedPipeClient: NewNamedPipeClient(pipePath, "seafserv-threaded-rpcserver", 5),
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

func (c *SeafServerThreadedRpcClient) SeafileEditRepo(repoId, name, description, user string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_edit_repo", "int", []string{"string", "string", "string", "string"})(
		repoId, name, description, user)
}

func (c *SeafServerThreadedRpcClient) SeafileIsRepoOwner(userId, repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_is_repo_owner", "int", []string{"string", "string"})(
		userId, repoId)
}

func (c *SeafServerThreadedRpcClient) SeafileGetRepoOwner(repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_repo_owner", "string", []string{"string"})(repoId)
}

func (c *SeafServerThreadedRpcClient) SeafileGetCommitList(repoId string, offset, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_commit_list", "objlist", []string{"string", "int", "int"})(
		repoId, offset, limit)
}

func (c *SeafServerThreadedRpcClient) SeafileGenerateRepoToken(repoId, email string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_generate_repo_token", "string", []string{"string", "string"})(repoId, email)
}

func (c *SeafServerThreadedRpcClient) DeleteRepoTokensByEmail(email string) (interface{}, error) {
	return CreateRPCMethod(c, "delete_repo_tokens_by_email", "int", []string{"string"})(email)
}

func (c *SeafServerThreadedRpcClient) GetSystemDefaultRepoId() (interface{}, error) {
	return CreateRPCMethod(c, "get_system_default_repo_id", "string", []string{})()
}

func (c *SeafServerThreadedRpcClient) SeafileGetDirIdByCommitAndPath(repoId, commitId, path string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_dir_id_by_commit_and_path", "string", []string{"string", "string", "string"})(
		repoId, commitId, path)
}

func (c *SeafServerThreadedRpcClient) SeafileGetFileIdByPath(repoId, path string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_file_id_by_path", "string", []string{"string", "string"})(
		repoId, path)
}

func (c *SeafServerThreadedRpcClient) SeafileGetDirIdByPath(repoId, path string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_dir_id_by_path", "string", []string{"string", "string"})(
		repoId, path)
}

func (c *SeafServerThreadedRpcClient) SeafileGetDirentByPath(repoId, path string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_dirent_by_path", "object", []string{"string", "string"})(
		repoId, path)
}

func (c *SeafServerThreadedRpcClient) SeafileListRepoSharedTo(fromUser, repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_list_repo_shared_to", "objlist", []string{"string", "string"})(
		fromUser, repoId)
}

func (c *SeafServerThreadedRpcClient) SeafileRenameFile(repoId, parentDir, oldname, newname, user string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_rename_file", "int", []string{"string", "string", "string", "string", "string"})(
		repoId, parentDir, oldname, newname, user)
}

func (c *SeafServerThreadedRpcClient) SeafileGetCommit(repoId string, version int, commitId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_commit", "object", []string{"string", "int", "string"})(
		repoId, version, commitId)
}

func (c *SeafServerThreadedRpcClient) SeafileListDir(repoId, dirId string, offset, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_list_dir", "objlist", []string{"string", "string", "int", "int"})(
		repoId, dirId, offset, limit)
}

func (c *SeafServerThreadedRpcClient) ListDirWithPerm(repoId, dirPath, dirId, user string, offset, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "list_dir_with_perm", "objlist", []string{"string", "string", "string", "string", "int", "int"})(
		repoId, dirPath, dirId, user, offset, limit)
}

func (c *SeafServerThreadedRpcClient) SeafileGetFileSize(storeId string, version int, fileId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_file_size", "int64", []string{"string", "int", "string"})(
		storeId, version, fileId)
}

func (c *SeafServerThreadedRpcClient) SeafilePostDir(repoId, parentDir, newDirName, user string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_post_dir", "int", []string{"string", "string", "string", "string"})(
		repoId, parentDir, newDirName, user)
}

func (c *SeafServerThreadedRpcClient) SeafileDelFile(repoId, parentDir, filename, user string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_del_file", "int", []string{"string", "string", "string", "string"})(
		repoId, parentDir, filename, user)
}

func (c *SeafServerThreadedRpcClient) SeafileCopyFile(srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename, user string,
	needProgress, synchronous int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_copy_file", "object", []string{"string", "string", "string", "string", "string", "string", "string", "int", "int"})(
		srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename, user, needProgress, synchronous)
}

func (c *SeafServerThreadedRpcClient) SeafileMoveFile(srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename string,
	replace int, user string, needProgress, synchronous int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_move_file", "object", []string{"string", "string", "string", "string", "string", "string", "int", "string", "int", "int"})(
		srcRepo, srcDir, srcFilename, dstRepo, dstDir, dstFilename, replace, user, needProgress, synchronous)
}

func (c *SeafServerThreadedRpcClient) SeafileIsValidFilename(repoId string, filename string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_is_valid_filename", "int", []string{"string", "string"})(
		repoId, filename)
}

func (c *SeafServerThreadedRpcClient) GetSharedRepoByPath(repoId, path, sharedTo string, isOrg int) (interface{}, error) {
	return CreateRPCMethod(c, "get_shared_repo_by_path", "object", []string{"string", "string", "string", "int"})(
		repoId, path, sharedTo, isOrg)
}

func (c *SeafServerThreadedRpcClient) SeafileRemoveShare(repoId, fromEmail, toEmail string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_remove_share", "int", []string{"string", "string", "string"})(
		repoId, fromEmail, toEmail)
}

func (c *SeafServerThreadedRpcClient) SetSharePermission(repoId, fromEmail, toEmail, permission string) (interface{}, error) {
	return CreateRPCMethod(c, "set_share_permission", "int", []string{"string", "string", "string", "string"})(
		repoId, fromEmail, toEmail, permission)
}

func (c *SeafServerThreadedRpcClient) SeafileGetSharedUsersForSubdir(repoId, path, fromUser string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_shared_users_for_subdir", "objlist", []string{"string", "string", "string"})(
		repoId, path, fromUser)
}

func (c *SeafServerThreadedRpcClient) SeafileAddShare(repoId, fromEmail, toEmail, permission string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_add_share", "string", []string{"string", "string", "string", "string"})(
		repoId, fromEmail, toEmail, permission)
}

func (c *SeafServerThreadedRpcClient) SeafileListShareRepos(email, queryCol string, start, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_list_share_repos", "objlist", []string{"string", "string", "int", "int"})(
		email, queryCol, start, limit)
}

func (c *SeafServerThreadedRpcClient) ShareSubdirToUser(repoId, path, owner, shareUser, permission, passwd string) (interface{}, error) {
	return CreateRPCMethod(c, "share_subdir_to_user", "string", []string{"string", "string", "string", "string", "string", "string"})(
		repoId, path, owner, shareUser, permission, passwd)
}

func (c *SeafServerThreadedRpcClient) UnshareSubdirForUser(repoId, path, owner, shareUser string) (interface{}, error) {
	return CreateRPCMethod(c, "unshare_subdir_for_user", "int", []string{"string", "string", "string", "string"})(
		repoId, path, owner, shareUser)
}

func (c *SeafServerThreadedRpcClient) UpdateShareSubdirPermForUser(repoId, path, owner, shareUser, permission string) (interface{}, error) {
	return CreateRPCMethod(c, "update_share_subdir_perm_for_user", "int", []string{"string", "string", "string", "string", "string"})(
		repoId, path, owner, shareUser, permission)
}

func (c *SeafServerThreadedRpcClient) ListInnerPubReposByOwner(user string) (interface{}, error) {
	return CreateRPCMethod(c, "list_inner_pub_repos_by_owner", "objlist", []string{"string"})(user)
}

func (c *SeafServerThreadedRpcClient) IsInnerPubRepo(repoId string) (interface{}, error) {
	return CreateRPCMethod(c, "is_inner_pub_repo", "int", []string{"string"})(repoId)
}

func (c *SeafServerThreadedRpcClient) GetUserQuotaUsage(userId string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_user_quota_usage", "int64", []string{"string"})(userId)
}

func (c *SeafServerThreadedRpcClient) GetUserQuota(user string) (interface{}, error) {
	return CreateRPCMethod(c, "get_user_quota", "int64", []string{"string"})(user)
}

func (c *SeafServerThreadedRpcClient) CheckQuota(repoId string, delta int64) (interface{}, error) {
	return CreateRPCMethod(c, "check_quota", "int", []string{"string", "int64"})(repoId, delta)
}

func (c *SeafServerThreadedRpcClient) SeafileGetUploadTmpFileOffset(repoId, filePath string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_upload_tmp_file_offset", "int64", []string{"string", "string"})(
		repoId, filePath)
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

func (c *SeafServerThreadedRpcClient) SeafileWebGetAccessToken(repoId, objId, op, username string, useOnetime int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_web_get_access_token", "string", []string{"string", "string", "string", "string", "int"})(
		repoId, objId, op, username, useOnetime)
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
