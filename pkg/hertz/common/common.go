package common

import (
	"files/pkg/common"
	"files/pkg/global"
	"fmt"
	"os"
)

func FormatSmbLink(fileType string, extend string, smbName string) string {
	if fileType == common.External || fileType == common.Cache {
		return fmt.Sprintf("smb://%s/%s", os.Getenv("NODE_IP"), smbName)
	}

	var masterNodeName = global.GlobalNode.GetMasterNode()
	return fmt.Sprintf("smb://%s/%s", global.GlobalNode.GetNodeIp(masterNodeName), smbName)
}
