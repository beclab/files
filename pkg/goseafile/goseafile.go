package goseafile

import (
	"files/pkg/gosearpc"
)

type NamedPipeClient = gosearpc.NamedPipeClient
type SearpcClient = gosearpc.SearpcClient

var CreateRPCMethod = gosearpc.CreateRPCMethod

type SeafServerThreadedRpcClient struct {
	*NamedPipeClient
}

func NewSeafServerClient(pipePath string) *SeafServerThreadedRpcClient {
	return &SeafServerThreadedRpcClient{
		NamedPipeClient: gosearpc.NewNamedPipeClient(pipePath, "seafserv-threaded-rpcserver", 5),
	}
}

func (c *SeafServerThreadedRpcClient) SeafileCreateRepo(name, desc, ownerEmail, passwd string, encVersion int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_create_repo", "string", []string{"string", "string", "string", "string", "int"})(
		name, desc, ownerEmail, passwd, encVersion)
}

func (c *SeafServerThreadedRpcClient) SeafileCreateEncRepo(repoID, name, desc, ownerEmail, magic, key, salt string, encVersion int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_create_enc_repo", "string", []string{"string", "string", "string", "string", "string", "string", "string", "int"})(
		repoID, name, desc, ownerEmail, magic, key, salt, encVersion)
}

func (c *SeafServerThreadedRpcClient) SeafileGetReposByIdPrefix(idPrefix string, start, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_repos_by_id_prefix", "objlist", []string{"string", "int", "int"})(
		idPrefix, start, limit)
}

func (c *SeafServerThreadedRpcClient) SeafileGetRepo(repoID string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_get_repo", "object", []string{"string"})(repoID)
}

func (c *SeafServerThreadedRpcClient) SeafileDestroyRepo(repoID string) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_destroy_repo", "int", []string{"string"})(repoID)
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

func (c *SeafServerThreadedRpcClient) GetOwnedRepoList(username string, retCorrupted int, start int, limit int) (interface{}, error) {
	return CreateRPCMethod(c, "seafile_list_owned_repos", "objlist", []string{"string", "int", "int", "int"})(
		username, retCorrupted, start, limit)
}
