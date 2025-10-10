package samba

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	v1 "files/pkg/apis/sys.bytetrade.io/v1"
	k8sclient "files/pkg/client"
	"files/pkg/common"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/model/api/share"
	"files/pkg/models"
	"fmt"
	"io/ioutil"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	ForceUser  string `json:"force_user"`
	ForceGroup string `json:"force_group"`
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
	sync.RWMutex
}

func NewSambaManager(f k8sclient.Factory) {
	SambaService = &samba{
		ctx:      context.Background(),
		factory:  f,
		commands: new(commands),
	}
}

func (s *samba) Start() {
	s.deleteExpiredShares()
	s.generateConf()
	s.commands.Run()
}

func (s *samba) CreateShareSamba(owner string, ids string, operator string) error {
	cli, _ := s.factory.DynamicClient()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

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
			Owner:    owner,
			ShareIds: ids,
			Operator: operator,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(data)
	if err != nil {
		klog.Errorf("samba convert error: %v, id: %s, owner: %s, operator: %s", err, ids, owner, operator)
		return err
	}

	res, err := cli.Resource(SambaGVR).Namespace(common.DefaultNamespace).Create(ctx, &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("samba create error: %v, id: %s, owner: %s, operator: %s", err, ids, owner, operator)
		return err
	}

	klog.Infof("samba create share: %v", res.UnstructuredContent())

	return nil
}

func (s *samba) HandlerEvent() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				klog.Info("samba addFunc")
				s.generateConf()
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
					klog.Errorf("samba delete, delete failed, error: %v, owner: %s, operate: %s, ids: %s", err, v.Spec.Owner, v.Spec.Operator, v.Spec.ShareIds)
					continue
				}

				klog.Infof("samba delete, delete done, owner: %s, operate: %s, ids: %s", v.Spec.Owner, v.Spec.Operator, v.Spec.ShareIds)
			}
		}
	}()
}

func (s *samba) generateConf() {
	s.Lock()
	defer s.Unlock()

	smbUsers, _ := s.commands.ListUser(shareTypeSmb)

	smbShareData, err := database.QuerySharePathByType(shareTypeSmb)
	if err != nil {
		klog.Errorf("samba get shares data error: %v", err)
		return
	}

	klog.Infof("samba get users: %v", smbUsers)

	if len(smbShareData) == 0 {
		klog.Infof("samba shares not found")
	}

	s.deleteExcludeUsers(smbUsers, smbShareData)

	smbShareBytes, _ := json.Marshal(smbShareData)
	klog.Infof("samba share paths: %s", string(smbShareBytes))

	var shares = SambaShares{}
	for _, item := range smbShareData {
		expire, err := time.Parse(timeFormat, item.ExpireTime)
		if err != nil {
			klog.Errorf("samba sharePath time expired, error: %v, time: %s", err, item.ExpireTime)
			continue
		}

		if time.Now().UTC().After(expire) {
			klog.Warningf("samba sharePath expired, time: %s, id: %s, name: %s, owner: %s", item.ExpireTime, item.ID, item.Name, item.Owner)
			continue
		}

		shareUser, sharePwd, err := s.getUser(item.PasswordMd5)
		if err != nil {
			klog.Errorf("samba decode user error: %v, data: %s, id: %s, name: %s, owner: %s", err, item.PasswordMd5, item.ID, item.Name, item.Owner)
			continue
		}

		if err := s.commands.CreateUser(shareUser, sharePwd); err != nil {
			klog.Errorf("samba create user error: %v", err)
			return
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

		w, r := s.formatPrivilege(item.Permission)

		var smbShare = SambaShare{
			Name:       item.ID,
			Path:       fileUri + fp.Path,
			Comment:    item.Name,
			ValidUsers: shareTypeSmb,
			Writable:   w,
			ReadOnly:   r,
			ForceUser:  shareUser,
			ForceGroup: shareTypeSmb,
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

func (s *samba) getUser(v string) (name string, pwd string, err error) {
	var de []byte
	var p = v
	de, err = base64.URLEncoding.DecodeString(p)
	if err != nil {
		klog.Errorf("samba decode sharePath info error: %v, data: %s", err, p)
		return
	}
	var a SambaSharePathAccount
	if err = json.Unmarshal(de, &a); err != nil {
		klog.Errorf("samba unmarshal sharePath account error: %v, data: %s", err, string(de))
		return
	}

	name = a.User
	pwd = a.Password
	return
}

func (s *samba) formatPrivilege(permission int32) (string, string) {
	switch permission {
	case 1:
		return "no", "yes"
	case 2:
		return "yes", "yes"
	case 3, 4:
		return "yes", "yes"
	}

	return "no", "no"
}

func (s *samba) deleteExcludeUsers(sysUsers []string, smbShareData []*share.SharePath) error {
	if smbShareData == nil || len(smbShareData) == 0 {
		s.commands.DeleteUser(sysUsers)
		return nil
	}

	var excludes []string
	var includes []string
	for _, ssd := range smbShareData {
		shareUser, _, err := s.getUser(ssd.PasswordMd5)
		if err != nil {
			klog.Errorf("samba get exclude user error: %v, data: %s", err, ssd.PasswordMd5)
			continue
		}
		includes = append(includes, shareUser)
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
