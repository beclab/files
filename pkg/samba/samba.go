package samba

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	v1 "files/pkg/apis/sys.bytetrade.io/v1"
	"files/pkg/client"
	k8sclient "files/pkg/client"
	"files/pkg/common"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/model/api/share"
	"files/pkg/models"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var SambaGVR = schema.GroupVersionResource{Group: "sys.bytetrade.io", Version: "v1", Resource: "sharesambas"}

const (
	shareTypeSmb = "smb"
	timeFormat   = "2006-01-02T15:04:05Z"
)

//go:embed template/samba.conf.tmpl
var sambaConfTemplateContent string

type SambaShares struct {
	Paths []SambaShare
}

type SambaShare struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Comment    string `json:"comment"`
	ValidUsers string `json:"valid_users"`
	Writable   string `json:"writable"`
	ReadOnly   string `json:"read_only"`
	ReadList   string `json:"read_list"`
	WriteList  string `json:"write_list"`
	ForceUser  string `json:"force_user"`
	ForceGroup string `json:"force_group"`
	Anonymous  bool   `json:"anonymous"`
}

type SambaSharePathAccount struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

var SambaService *samba

type samba struct {
	ctx      context.Context
	factory  k8sclient.Factory
	commands *commands
	users    []string
	runTime  time.Time
	sync.RWMutex
}

func NewSambaManager(f k8sclient.Factory) {
	SambaService = &samba{
		ctx:      context.Background(),
		factory:  f,
		runTime:  time.Now(),
		commands: new(commands),
	}
}

func (s *samba) Start() {
	s.getUsers()
	s.generateConf()
	s.commands.Run()

	s.deleteExpiredShares()
}

func (s *samba) CreateShareSamba(sharePath []*share.SmbCreate, operator string) error {
	cli, _ := s.factory.DynamicClient()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var items []string
	for _, sp := range sharePath {
		items = append(items, common.ParseString(sp))
	}

	var data = &v1.ShareSamba{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "sys.bytetrade.io/v1",
			Kind:       "ShareSamba",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid.New().String(),
			Namespace: common.DefaultNamespace,
		},
		Spec: v1.ShareSambaSpec{
			ShareItems: items,
			Operator:   operator,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(data)
	if err != nil {
		klog.Errorf("samba convert error: %v, operator: %s", err, operator)
		return err
	}

	res, err := cli.Resource(SambaGVR).Namespace(common.DefaultNamespace).Create(ctx, &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("samba create error: %v, operator: %s", err, operator)
		return err
	}

	klog.Infof("samba create share: %v", res.UnstructuredContent())

	return nil
}

func (s *samba) UserHandlerEvent() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				s.getUsers()

				klog.Infof("samba user addFunc, users: %v", s.users)

			},
			DeleteFunc: func(obj interface{}) {
				user := obj.(*models.User)
				klog.Infof("samba user deleteFunc, user: %s", user.Name)

				s.getUsers()
				s.generateConf()
				s.deleteUserGroup(user.Name)
			},
		},
	}
}

func (s *samba) HandlerEvent() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sambaObj := obj.(*v1.ShareSamba)
				if sambaObj.CreationTimestamp.Time.After(s.runTime) {
					klog.Info("samba addFunc")
					s.generateConf()

					switch sambaObj.Spec.Operator {
					case "del":
						s.recoverSharedOwner(sambaObj.Spec.ShareItems)
					case "deluser":
						s.deleteUser(sambaObj.Spec.ShareItems)
					}
				}
			},
		},
	}
}

func (s *samba) deleteExpiredShares() {
	go func() {
		for range time.NewTicker(30 * time.Minute).C {
			klog.Info("samba delete crds with ticker")
			cli, err := s.factory.DynamicClient()
			if err != nil {
				klog.Errorf("samba get dynamic client error: %v", err)
				continue
			}

			res, err := cli.Resource(SambaGVR).Namespace(common.DefaultNamespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				klog.Errorf("samba get shares list error: %v", err)
				continue
			}

			for _, item := range res.Items {
				var v v1.ShareSamba
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &v)
				if err != nil {
					klog.Error("samba delete, convert to unstructured error, ", err, ", ", item)
					continue
				}

				if !metav1.Now().Time.Add(-6 * time.Hour).After(v.CreationTimestamp.Time) {
					continue
				}

				if err := cli.Resource(SambaGVR).Namespace(common.DefaultNamespace).Delete(context.Background(), v.Name, metav1.DeleteOptions{}); err != nil {
					klog.Errorf("samba delete, delete failed, error: %v, operate: %s", err, v.Spec.Operator)
					continue
				}

				klog.Infof("samba delete, delete done, operate: %s", v.Spec.Operator)
			}
		}
	}()
}

func (s *samba) getUsers() {
	s.Lock()
	defer s.Unlock()

	config, err := s.factory.ClientConfig()
	if err != nil {
		klog.Errorf("samba user addFunc clientConfig get error: %v", err)
		return
	}
	cli, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Errorf("samba user addFunc dynamicClient get error: %v", err)
		return
	}

	users, err := client.GetUser(cli)
	if err != nil {
		klog.Errorf("samba user addFunc getusers error: %v", err)
		return
	}

	s.users = []string{}

	for _, user := range users {
		s.users = append(s.users, user.Name)
	}
}

func (s *samba) generateConf() {
	s.Lock()
	defer s.Unlock()

	smbUsers, _ := s.commands.ListUser(s.users)

	smbShareData, err := database.QuerySmbShares("", shareTypeSmb, nil, nil)
	if err != nil {
		klog.Errorf("samba get shares data error: %v", err)
		return
	}

	smbShares := FormatSharePathViews(smbShareData)

	klog.Infof("samba get system users: %v", smbUsers)

	if len(smbShares) == 0 {
		klog.Infof("samba shares not found")
	}

	s.deleteExcludeUsers(smbUsers, smbShares)

	smbShareBytes, _ := json.Marshal(smbShares)
	klog.Infof("samba share paths: %s", string(smbShareBytes))

	var shares = SambaShares{}
	for _, item := range smbShares { // ~ map
		if !s.checkUserExists(item.Owner) {
			continue
		}
		expire, err := time.Parse(timeFormat, item.ExpireTime)
		if err != nil {
			klog.Errorf("samba sharePath time expired, error: %v, time: %s", err, item.ExpireTime)
			continue
		}

		if time.Now().UTC().After(expire) { // exclude expired shares
			klog.Warningf("samba sharePath expired, time: %s, id: %s, name: %s, owner: %s", item.ExpireTime, item.Id, item.Name, item.Owner)
			continue
		}

		fPath := fmt.Sprintf("/%s/%s%s", item.FileType, item.Extend, item.Path)
		fp, err := models.CreateFileParam(item.Owner, fPath)
		if err != nil {
			klog.Errorf("samba create fileParam error: %v", err)
			return
		}

		fileUri, err := fp.GetResourceUri()
		if err != nil {
			klog.Errorf("samba get fileParam uri error: %v", err)
			return
		}

		var validUser = []string{fmt.Sprintf("@%s", item.Owner)}
		var writeUser []string
		var readUser []string

		var anonymous bool
		var sharePwd string
		if !item.PublicShare {
			if len(item.Members) > 0 {
				for _, user := range item.Members {
					sharePwd, err = s.getUser(user.Password)
					if err != nil {
						klog.Errorf("samba get user pwd error: %v, password: %s, userId: %s, userName: %s, owner: %s", err, user.Password, user.UserId, user.UserName, item.Owner)
						continue
					}

					validUser = append(validUser, user.UserName)

					if err := s.commands.CreateGroup(item.Owner, ""); err != nil {
						klog.Errorf("samba create group %s error: %v", item.Owner, err)
						return
					}

					if err := s.commands.CreateUser(user.UserName, sharePwd, item.Owner); err != nil {
						klog.Errorf("samba create user %s error: %v", user.UserName, err)
						return
					}

					if user.Permission > 1 {
						if err := s.commands.SetAcl(user.UserName, item.Owner, "-m", "rwx", fileUri+fp.Path); err != nil {
							klog.Errorf("samba setfacl error: %v", err)
							return
						}
						writeUser = append(writeUser, user.UserName)
					} else {
						readUser = append(readUser, user.UserName)
					}
				}
			}
		} else {
			// anonymous
			anonymous = true
			s.commands.SetAnonymousPermission(item.Owner, fileUri+fp.Path)
		}

		var smbShare = SambaShare{
			Name:       item.Name,
			Path:       fileUri + strings.TrimSuffix(fp.Path, "/"),
			Comment:    fmt.Sprintf("%s_%s_%s", item.Owner, item.Id, item.Name),
			ValidUsers: item.Owner,
			Writable:   "yes",
			ReadOnly:   "no",
			ReadList:   strings.Join(readUser, ","),
			WriteList:  strings.Join(writeUser, ","),
			ForceUser:  strings.Join(validUser, ","),
			ForceGroup: item.Owner,
			Anonymous:  anonymous,
		}
		shares.Paths = append(shares.Paths, smbShare)
	}

	tmpl, err := template.New("samba.conf").Parse(sambaConfTemplateContent)
	if err != nil {
		klog.Errorf("samba get template error: %v", err)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, shares); err != nil {
		klog.Errorf("samba template generate error: %v", err)
		return
	}

	content := buf.Bytes()

	klog.Infof("samba conf content: \n%s\n", string(content))

	err = ioutil.WriteFile(common.SambaConfTemplatePath, content, 0700)
	if err != nil {
		klog.Errorf("samba write conf error: %v", err)
		return
	}

	if err := s.commands.Update(); err != nil {
		klog.Errorf("samba reload error: %v", err)
		return
	}

	klog.Info("samba conf update done")
}

func (s *samba) getUser(pass string) (pwd string, err error) {
	password, err := common.Base64Decode(pass)
	if err != nil {
		klog.Errorf("samba decode sharePath info error: %v, data: %s", err, pass)
		return
	}

	pwd = password
	return
}

func (s *samba) deleteExcludeUsers(sysUsers []string, smbShareData map[string]*models.SambaShares) error {
	if len(smbShareData) == 0 {
		s.commands.DeleteUser(sysUsers)
		return nil
	}

	var excludes []string
	var includes []string
	for _, ssd := range smbShareData {
		for _, user := range ssd.Members {
			shareUser := user.UserName
			includes = append(includes, shareUser)
		}

	}

	if len(includes) > 0 {
		for _, su := range sysUsers {
			var f bool
			for _, ic := range includes {
				if su == ic {
					f = true
					break
				}
			}

			if !f {
				excludes = append(excludes, su)
			}
		}
	}

	klog.Infof("samba delete exclude users: %v", excludes)

	if len(excludes) > 0 {
		s.commands.DeleteUser(excludes)
	}

	return nil
}

func (s *samba) deleteUserGroup(owner string) {
	s.Lock()
	defer s.Unlock()
	users, _ := s.commands.ListUser([]string{owner})

	s.commands.DeleteUser(users)
	s.commands.DeleteGroup(owner)
}

func (s *samba) checkUserExists(owner string) bool {
	var f bool
	for _, e := range s.users {
		if e == owner {
			f = true
			break
		}
	}

	return f
}

func (s *samba) deleteUser(sharedPaths []string) {
	var paths []*share.SmbCreate
	for _, p := range sharedPaths {
		var smb *share.SmbCreate
		if err := json.Unmarshal([]byte(p), &smb); err != nil {
			klog.Errorf("samba deleteUser unmarshal error: %v, content: %s", err, p)
			continue
		}
		paths = append(paths, smb)
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i].User < paths[j].User
	})

	var deleteUsersIdx = make(map[string]string)
	for _, p := range paths {
		m, err := models.CreateFileParam(p.Owner, p.Path)
		if err != nil {
			klog.Errorf("samba deleteUser create file param error: %v, owner: %s, path: %s", err, p.Owner, p.Path)
			continue
		}

		if _, ok := deleteUsersIdx[p.User]; !ok {
			deleteUsersIdx[p.User] = p.User
		}

		uri, err := m.GetResourceUri()
		if err != nil {
			klog.Errorf("samba deleteUser get uri error: %v", err)
			continue
		}
		var dir = uri + m.Path
		if err := s.commands.SetAcl(p.User, p.Owner, "-x", "", dir); err != nil {
			klog.Errorf("samba deleteUser setfacl remove error: %v, user: %s, path: %s", err, p.User, dir)
			continue
		}
	}

	var deleteUsers []string
	for k := range deleteUsersIdx {
		deleteUsers = append(deleteUsers, k)
	}

	s.commands.DeleteUser(deleteUsers)

}

func (s *samba) recoverSharedOwner(sharedPaths []string) {
	for _, p := range sharedPaths {
		var smb *share.SmbCreate
		if err := json.Unmarshal([]byte(p), &smb); err != nil {
			klog.Errorf("samba recover unmarshal error: %v, content: %s", err, p)
			continue
		}

		m, err := models.CreateFileParam(smb.Owner, smb.Path)
		if err != nil {
			klog.Errorf("samba recover create file param error: %v, owner: %s, path: %s", err, smb.Owner, smb.Path)
			continue
		}

		uri, err := m.GetResourceUri()
		if err != nil {
			klog.Errorf("samba recover get uri error: %v", err)
			continue
		}

		if smb.User != "" {
			if err := s.commands.SetAcl(smb.User, smb.Owner, "-x", "", uri+m.Path); err != nil { // remove acl
				klog.Errorf("samba recover, setfacl remove error: %v", err)
				continue
			}
		}

	}
}

func (s *samba) CreateSambaSharePath(owner string, smbSharePublicLevel int32, users []*share.CreateSmbSharePathMembers, fileParam *models.FileParam, reqExpireIn int64, reqExpireTime string) (*share.SharePath, error) {
	s.Lock()
	defer s.Unlock()

	var smbSharePathExists bool
	var err error

	var smdSharePath *share.SharePath
	smdSharePath, err = database.GetSmbSharePathByPath(common.ShareTypeSMB, fileParam.FileType, fileParam.Extend, fileParam.Path, owner, smbSharePublicLevel)
	if err != nil {
		klog.Errorf("get smb share error: %v", err)
		return nil, err
	}
	if smdSharePath != nil {
		smbSharePathExists = true
	}

	if smbSharePathExists {
		return nil, errors.New(common.ErrorMessageShareExists)
	}

	var smbSharePaths []*share.SharePath
	smbSharePaths, err = database.QuerySharePathByType(common.ShareTypeSMB)
	if err != nil {
		klog.Errorf("query smb shares error: %v", err)
		return nil, err
	}

	var smbShareName string
	smbShareName, err = GetSambaShareName(fileParam.Path)
	if err != nil {
		klog.Errorf("get smb share name by path invalid: %v", err)
		return nil, err
	}

	if len(smbSharePaths) > 0 {
		var allSmbSharePaths []string
		for _, item := range smbSharePaths {
			allSmbSharePaths = append(allSmbSharePaths, item.Path)
		}
		smbShareName, err = GetSambaShareDupName(smbShareName, allSmbSharePaths)
		if err != nil {
			klog.Errorf("get smb share new name error: %v", err)
			return nil, err
		}
	}

	expireIn, expireTime := common.AdjustExpire(reqExpireIn, reqExpireTime)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var sharePathId = uuid.New().String()
	var newSmbSharePath = &share.SharePath{
		ID:             sharePathId,
		Owner:          owner,
		FileType:       fileParam.FileType,
		Extend:         fileParam.Extend,
		Path:           fileParam.Path,
		ShareType:      common.ShareTypeSMB,
		Name:           smbShareName,
		ExpireIn:       expireIn,
		ExpireTime:     expireTime,
		SmbSharePublic: smbSharePublicLevel,
		CreateTime:     now,
		UpdateTime:     now,
	}

	var newSmbShareMembers []*share.ShareSmbMember
	for _, u := range users {
		var newSmbShareMember = &share.ShareSmbMember{
			Owner:      owner,
			PathID:     sharePathId,
			UserID:     u.ID,
			Permission: u.Permission,
			CreateTime: now,
			UpdateTime: now,
		}
		newSmbShareMembers = append(newSmbShareMembers, newSmbShareMember)
	}

	if err := database.CreateSmbSharePathTx(newSmbSharePath, newSmbShareMembers); err != nil {
		return nil, err
	}

	return newSmbSharePath, nil
}

func (s *samba) DeleteSambaShareUsers(owner string, users []string) error {
	s.Lock()
	defer s.Unlock()

	return database.DeleteSmbUserTx(owner, users)
}

func (s *samba) ModifySambaShareMembers(owner string, sharePath *share.SharePath, modifyMembers []*share.CreateSmbSharePathMembers) error {
	s.Lock()
	defer s.Unlock()

	members, err := database.QuerySmbMembers(sharePath.ID)
	if err != nil {
		klog.Errorf("[samba] query smb share members failed, owner: %s, pathId: %s, error: %v", owner, sharePath.ID, err)
		return err
	}

	addMembers, editMembers, delMembers := CompareSmbShareMembers(members, modifyMembers)

	klog.Infof("[samba] modify smb share members, owner: %s, pathId: %s, add: %s, edit: %s, del: %s", owner, sharePath.ID, common.ParseString(addMembers), common.ParseString(editMembers), common.ParseString(delMembers))

	if err := database.ModifySmbMembersTx(owner, sharePath.ID, addMembers, editMembers, delMembers); err != nil {
		klog.Errorf("[samba] modify smb share members failed, owner: %s, pathId: %s, error: %v", owner, sharePath.ID, err)
		return err
	}

	var crd = &share.SmbCreate{
		Owner: owner,
		ID:    sharePath.ID,
		Path:  fmt.Sprintf("/%s/%s%s", sharePath.FileType, sharePath.Extend, sharePath.Path),
		User:  "",
	}

	if err := s.CreateShareSamba([]*share.SmbCreate{crd}, "add"); err != nil {
		klog.Errorf("[samba] modify smb share members, update crd failed, owner: %s, pathId: %s, error: %v", owner, sharePath.ID, err)
		return err
	}

	return nil
}
