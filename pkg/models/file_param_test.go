package models

import (
	"files/pkg/files"
	"files/pkg/global"
	"testing"

	"github.com/go-playground/assert/v2"
	v1 "k8s.io/api/core/v1"
)

var owner string
var path string
var url string
var err error
var resUri string
var param *FileParam

func setPath() {
	// ~ drive
	path = "/api/resources/drive/Home/"

	/** {"fileType":"drive", "extend":"", "path":"/Home/Downloads/abc/def/"} */
	// path = "/api/resources/drive/Home/Downloads/abc/def/"

	/** {"fileType":"drive", "extend":"", "path":"/Home/Home/abc/def/"} */
	// path = "/api/resources/drive/Home/Home/abc/def/"

	/** {"fileType":"drive", "extend":"", "path":"/Home/Pictures/wp3067715-the-expanse-wallpapers.jpg?size=thumb&auth=&inline=true&key=1750235796179"} */
	// path = "/api/preview/drive/Home/Pictures/wp3067715-the-expanse-wallpapers.jpg?size=thumb&auth=&inline=true&key=1750235796179"

	// ~ data
	path = "/api/resources/drive/Data/"

	/** {"fileType":"data", "extend":"", "path":"/Data/studio/helm-repo-dev/"} */
	// path = "/api/resources/drive/Data/studio/helm-repo-dev/"

	// ~ cache
	/** {"fileType":"cache", "extend":"", "path":"/"} */
	// path = "/api/resources/cache"

	/** {"fileType":"cache", "extend":"olares", "path":"/"} */
	// path = "/api/resources/cache/olares/"

	/** {"fileType":"cache", "extend":"olares", "path":"tailscale/"} */
	// path = "/api/resources/cache/olares/tailscale/"

	/** {"fileType":"cache", "extend":"olares", "path":"olares/tailscale/hello/world.jpg"} */
	// path = "/api/resources/cache/olares/olares/tailscale/hello/world.jpg"

	/** {"fileType":"cache", "extend":"dell1", "path":"tailscale/"} */
	// path = "/api/resources/cache/dell1/tailscale/"

	path = "/api/preview/cache/wp3067715-the-expanse-wallpapers.jpg"

	// ~ sync
	/** {"fileType":"sync", "extend":"", "path":"/"} */
	// path = "/api/resources/sync/"

	/** {"fileType":"sync", "extend":"93e5145f-5dd8-4051-98bd-30720ddd820b", "path":"/"} */
	// path = "/api/resources/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/"

	/** {"fileType":"sync", "extend":"93e5145f-5dd8-4051-98bd-30720ddd820b", "path":"type/"} */
	// path = "/api/resources/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/type/"

	/** {"fileType":"sync", "extend":"93e5145f-5dd8-4051-98bd-30720ddd820b", "path":"type/fasdf/s/"} */
	// path = "/api/resources/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/type/fasdf/s/"

	/** {"fileType":"sync", "extend":"93e5145f-5dd8-4051-98bd-30720ddd820b", "path":"wp3067715-the-expanse-wallpapers.jpg"} */
	// path = "/api/preview/sync/93e5145f-5dd8-4051-98bd-30720ddd820b/pics/wp3067715-the-expanse-wallpapers.jpg?size=thumb&auth=&inline=true&key=1750248849000"

	// ~ dropbox
	/** {"fileType":"dropbox", "extend":"2579250305", "path":"/"} */
	// path = "/api/resources/dropbox/2579250305/"

	/** {"fileType":"dropbox", "extend":"2579250305", "path":"test_dropbox/"} */
	// path = "/api/resources/dropbox/2579250305/test_dropbox/"

	/** {"fileType":"dropbox", "extend":"2579250305", "path":"%E5%B0%8F%E6%96%87%E4%BB%B6/abc/"} */
	// path = "/api/resources/dropbox/2579250305/%E5%B0%8F%E6%96%87%E4%BB%B6/abc/"

	/** {"fileType":"dropbox", "extend":"2579250305", "path":"wp3067715-the-expanse-wallpapers.jpg"} */
	// path = "/api/preview/dropbox/2579250305/wp3067715-the-expanse-wallpapers.jpg?size=thumb&auth=&inline=true&key=1749732951000"

	// ~ google
	/** {"fileType":"google", "extend":"test10001@gmail.com", "path":"root/"} */
	// path = "/api/resources/google/test10001@gmail.com/root/"

	/** {"fileType":"google", "extend":"test10001@gmail.com", "path":"1OSdkRStgb6AygrKZI890OYjHJPdjdMDi/"} */
	// path = "/api/resources/google/test10001@gmail.com/1OSdkRStgb6AygrKZI890OYjHJPdjdMDi/"

	// ~ awss3
	/** {"fileType":"awss3", "extend":"AAAAAAAAAAAAAAAAAAAAA", "path":""} */
	// path = "/api/resources/awss3/AAAAAAAAAAAAAAAAAAAAA/"

	// ~ external
	/** {"fileType": "external", "extend": "", path: "/"} */
	// path = "/api/resources/external"     // Return node list
	// path = "/api/resources/external/" 		// Return node list

	/** {"fileType":"external", "extend":"node1", "path":"/"} */
	// path = "/api/resources/external/node1/" //  Return all directories and mounted devices under the current node, like /api/resources/cache

	// ~ internal usb smb hdd
	/** {"fileType":"internal", "extend":"node1", "path":"fasdf/afsdf/"} */
	// path = "/api/resources/external/node1/fasdf/afsdf/"

	/** {"fileType":"usb", "extend":"olares", "path":"VendorCo-0/"} */
	// path = "/api/resources/external/olares/VendorCo-0/"

	/** {"fileType":"usb", "extend":"olares", "path":"VendorCo-0/System%20Volume%20Information/"} */
	// path = "/api/resources/external/olares/VendorCo-0/System%20Volume%20Information/"

	/** {"fileType":"hdd", "extend":"olares", "path":"hdd0/afsdf/"} */
	// path = "/api/resources/external/olares/hdd0/afsdf/"

	/** {"fileType":"hdd", "extend":"olares", "path":"hdd1/test/"} */
	// path = "/api/resources/external/olares/hdd1/test/"

	/** {"fileType":"smb", "extend":"node3", "path":"smbshare/fas/df/adsf/"} */
	// path = "/api/resources/external/node3/smbshare/fas/df/adsf/"
}

func initGlobal(owner string) {
	global.GlobalData = &global.Data{
		UserPvcMap: map[string]string{
			owner: "user-pvc-user1",
		},
		CachePvcMap: map[string]string{
			owner: "cache-pvc-user1",
		},
	}

	global.GlobalNode = &global.Node{
		Nodes: map[string]*v1.Node{
			"olares": &v1.Node{},
		},
	}

	global.GlobalMounted = &global.Mount{
		Mounted: map[string]*files.DiskInfo{
			"hdd0": &files.DiskInfo{
				Type: "hdd",
			},
			"smbshare": &files.DiskInfo{
				Type: "smb",
			},
			"VendorCo-0": &files.DiskInfo{
				Type: "usb",
			},
		},
	}
}

func TestAll(t *testing.T) {
	TestHome(t)
	TestData(t)
	TestCache(t)
	TestSync(t)
	TestCloud(t)
	TestExternal(t)
}

func TestHome(t *testing.T) {
	owner = "user1"
	initGlobal(owner)

	url = "drive/Home/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "drive")
	assert.Equal(t, param.Extend, "Home")
	assert.Equal(t, param.Path, "/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/user-pvc-user1/Home/")

	url = "drive/Home/folder/subfolder/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "drive")
	assert.Equal(t, param.Extend, "Home")
	assert.Equal(t, param.Path, "/folder/subfolder/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/user-pvc-user1/Home/folder/subfolder/")

	url = "drive/Home/folder/pic.jpg"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "drive")
	assert.Equal(t, param.Extend, "Home")
	assert.Equal(t, param.Path, "/folder/pic.jpg")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/user-pvc-user1/Home/folder/pic.jpg")
}

func TestData(t *testing.T) {
	owner = "user1"
	initGlobal(owner)

	url = "drive/Data/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "drive")
	assert.Equal(t, param.Extend, "Data")
	assert.Equal(t, param.Path, "/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/user-pvc-user1/Data/")

	url = "drive/Data/hello/world/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "drive")
	assert.Equal(t, param.Extend, "Data")
	assert.Equal(t, param.Path, "/hello/world/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/user-pvc-user1/Data/hello/world/")

	url = "drive/Data/hello/world/test_pic.jpg"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "drive")
	assert.Equal(t, param.Extend, "Data")
	assert.Equal(t, param.Path, "/hello/world/test_pic.jpg")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/user-pvc-user1/Data/hello/world/test_pic.jpg")
}

func TestCache(t *testing.T) {
	owner = "user1"
	initGlobal(owner)

	url = "cache/olares/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "cache")
	assert.Equal(t, param.Extend, "olares")
	assert.Equal(t, param.Path, "/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/appcache/cache-pvc-user1/")

	url = "cache/olares/test/folder/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "cache")
	assert.Equal(t, param.Extend, "olares")
	assert.Equal(t, param.Path, "/test/folder/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/appcache/cache-pvc-user1/test/folder/")
}

func TestSync(t *testing.T) {
	owner = "user1"
	initGlobal(owner)

	url = "sync/93e5145f-5dd8-4051-98bd-30720ddd820b/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "sync")
	assert.Equal(t, param.Extend, "93e5145f-5dd8-4051-98bd-30720ddd820b")
	assert.Equal(t, param.Path, "/")

	url = "sync/93e5145f-5dd8-4051-98bd-30720ddd820b/folder/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "sync")
	assert.Equal(t, param.Extend, "93e5145f-5dd8-4051-98bd-30720ddd820b")
	assert.Equal(t, param.Path, "/folder/")
}

func TestCloud(t *testing.T) {
	owner = "user1"
	initGlobal(owner)

	// google
	url = "google/account@gmail.com/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "google")
	assert.Equal(t, param.Extend, "account@gmail.com")
	assert.Equal(t, param.Path, "/")

	url = "google/account@gmail.com/AAA/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "google")
	assert.Equal(t, param.Extend, "account@gmail.com")
	assert.Equal(t, param.Path, "/AAA/")

	url = "google/account@gmail.com/BBB"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "google")
	assert.Equal(t, param.Extend, "account@gmail.com")
	assert.Equal(t, param.Path, "/BBB")

	// dropbox
	url = "dropbox/2222222222222/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "dropbox")
	assert.Equal(t, param.Extend, "2222222222222")
	assert.Equal(t, param.Path, "/")

	url = "dropbox/2222222222222/folder/subfolder/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "dropbox")
	assert.Equal(t, param.Extend, "2222222222222")
	assert.Equal(t, param.Path, "/folder/subfolder/")

	url = "dropbox/2222222222222/folder/subfolder/pic.jpg"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "dropbox")
	assert.Equal(t, param.Extend, "2222222222222")
	assert.Equal(t, param.Path, "/folder/subfolder/pic.jpg")

	// aws
	url = "awss3/AKIDxxxxxxxxx/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "awss3")
	assert.Equal(t, param.Extend, "AKIDxxxxxxxxx")
	assert.Equal(t, param.Path, "/")

	url = "awss3/AKIDxxxxxxxxx/folder/subfolder/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "awss3")
	assert.Equal(t, param.Extend, "AKIDxxxxxxxxx")
	assert.Equal(t, param.Path, "/folder/subfolder/")

	url = "awss3/AKIDxxxxxxxxx/folder/subfolder/pic.jpg"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "awss3")
	assert.Equal(t, param.Extend, "AKIDxxxxxxxxx")
	assert.Equal(t, param.Path, "/folder/subfolder/pic.jpg")
}

func TestExternal(t *testing.T) {
	owner = "user1"
	initGlobal(owner)

	url = "external/olares/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "external")
	assert.Equal(t, param.Extend, "olares")
	assert.Equal(t, param.Path, "/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/External/")

	url = "external/olares/data/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "internal")
	assert.Equal(t, param.Extend, "olares")
	assert.Equal(t, param.Path, "/data/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/External/data/")

	url = "external/olares/data/folder/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "internal")
	assert.Equal(t, param.Extend, "olares")
	assert.Equal(t, param.Path, "/data/folder/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/External/data/folder/")

	url = "external/olares/hdd0/folder/"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "hdd")
	assert.Equal(t, param.Extend, "olares")
	assert.Equal(t, param.Path, "/hdd0/folder/")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/External/hdd0/folder/")

	url = "external/olares/VendorCo-0/folder/pic.jpg"
	param, err = CreateFileParam(owner, url)
	assert.Equal(t, err, nil)
	assert.Equal(t, param.FileType, "usb")
	assert.Equal(t, param.Extend, "olares")
	assert.Equal(t, param.Path, "/VendorCo-0/folder/pic.jpg")
	resUri, err = param.GetResourceUri()
	assert.Equal(t, err, nil)
	assert.Equal(t, resUri+param.Path, "/data/External/VendorCo-0/folder/pic.jpg")
}
