package samba

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

/**
 * todo
 * > run
 * > update
 * > create user
 * > set password
 */

type commands struct{}

func (c *commands) Run() error {
	var cmd = exec.Command("/usr/bin/samba.sh")

	output, _ := cmd.CombinedOutput()
	klog.Infof("start smbd output: %s", string(output))

	return nil
}

func (c *commands) Update() error {
	var cmd = exec.Command("smbcontrol", "all", "reload-config")

	output, _ := cmd.CombinedOutput()
	klog.Infof("reload smbd output: %s", string(output))

	return nil
}

func (c *commands) CreateUser(userName, password string) error {
	klog.Infof("samba createUser, name: %s", userName)
	u, err := user.Lookup(userName)
	if err != nil {
		klog.Warning(err)
	}

	if u == nil {
		cmd := exec.Command("useradd", "-M", "-s", "/sbin/nologin", userName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			klog.Errorf("samba useradd error: %v, output: %s, cmd: %s", err, string(out), cmd.String())
		}
	}

	cmd := exec.Command("usermod", "-aG", "smb", userName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("samba usermod error: %v, output: %s, cmd: %s", err, string(out), cmd.String())
	}

	cmd = exec.Command("smbpasswd", "-c", "/etc/samba/smb.conf", "-a", "-s", userName)
	cmd.Stdin = bytes.NewBufferString(fmt.Sprintf("%s\n%s\n", password, password))
	out, err = cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("samba smbpasswd error: %v, output: %s, cmd: %s", err, string(out), cmd.String())
	}

	return nil
}

func (c *commands) DeleteUser(users []string) {
	for _, user := range users {
		output, err := exec.Command("smbpasswd", "-d", user).Output()
		if err != nil {
			klog.Errorf("samba smbpasswd delete user %s error: %v", user, string(output))
		}

		output, err = exec.Command("userdel", "-r", user).Output()
		if err != nil {
			klog.Errorf("samba delete user %s error: %v", user, string(output))
		}

		if err == nil {
			klog.Infof("samba delete user: %s done", user)
		}
	}
}

func (c *commands) ListUser(groupName string) ([]string, error) {
	outGrp, err := exec.Command("getent", "group", groupName).Output()
	if err != nil || len(outGrp) == 0 {
		return nil, fmt.Errorf("group %q not found via getent: %v", groupName, err)
	}
	line := strings.TrimSpace(string(outGrp))
	parts := strings.SplitN(line, ":", 4)
	if len(parts) < 3 {
		return nil, fmt.Errorf("unexpected group entry format: %q", line)
	}
	gid := parts[2]

	usersSet := make(map[string]struct{})

	// supplementary members (4th field, may be empty)
	if len(parts) == 4 && strings.TrimSpace(parts[3]) != "" {
		for _, u := range strings.Split(parts[3], ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				usersSet[u] = struct{}{}
			}
		}
	}

	// 2) scan passwd for primary group matches: name:passwd:uid:gid:gecos:home:shell
	cmd := exec.Command("getent", "passwd")
	outPwd, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("getent passwd failed: %v", err)
	}
	sc := bufio.NewScanner(bytes.NewReader(outPwd))
	for sc.Scan() {
		l := sc.Text()
		p := strings.SplitN(l, ":", 7)
		if len(p) < 4 {
			continue
		}
		if p[3] == gid { // primary group id equals target gid
			usersSet[p[0]] = struct{}{}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan passwd failed: %v", err)
	}

	// 3) to slice + sort
	users := make([]string, 0, len(usersSet))
	for u := range usersSet {
		users = append(users, u)
	}
	sort.Strings(users)
	return users, nil
}

func (c *commands) CreateGroup(groupName, groupId string) error {
	klog.Infof("samba CreateGroup, name: %s", groupName)
	g, err := user.LookupGroup(groupName)
	if err != nil {
		klog.Errorf("samba check group %s error: %v", groupName, err)
	}

	if g != nil {
		return nil
	}

	args := []string{groupName}
	if groupId != "" {
		args = []string{"-g", groupId, groupName}
	}
	cmd := exec.Command("groupadd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("samba groupadd error: %v, output: %s, cmd: %s", err, string(out), cmd.String())
	}

	return nil
}
