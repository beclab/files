package proxy

import (
	"files/pkg/drives"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"k8s.io/klog/v2"
)

func TestCache(t *testing.T) {
	var node = []string{"olares"}
	var userPvc = "pvc-user-01"
	var cachePvc = "pvc-cache-01"
	var p = "/api/paste/data/appstore/data/charts/r4world-1.0.2.tgz?action=copy&destination=/cache/olares/r4world-1.0.2.tgz&override=false&rename=true"

	parsedURL, _ := url.Parse(p)

	var request = &http.Request{
		URL: parsedURL,
	}

	oldUrl := p
	fmt.Println("old url: ", oldUrl)

	parts := strings.Split(oldUrl, "?")
	src := parts[0]

	query := parsedURL.Query()
	dst := query.Get("destination")
	fmt.Println("DST:", dst)

	srcType, err := drives.ParsePathType(strings.TrimPrefix(src, API_PASTE_PREFIX), request, false, false)
	if err != nil {
		fmt.Println(err)
		srcType = "Parse Error"
	}

	dstType, err := drives.ParsePathType(dst, request, true, false)
	if err != nil {
		fmt.Println(err)
		dstType = "Parse Error"
	}
	fmt.Println("SRC_TYPE:", srcType)
	fmt.Println("DST_TYPE:", dstType)

	if srcType == drives.SrcTypeDrive || srcType == drives.SrcTypeData || srcType == drives.SrcTypeExternal {
		src = rewriteUrl(src, userPvc, "", true)
	} else if srcType == drives.SrcTypeCache {
		src = rewriteUrl(src, cachePvc, API_PASTE_PREFIX+"/AppData", true)
	}

	if dstType == drives.SrcTypeDrive || dstType == drives.SrcTypeData || dstType == drives.SrcTypeExternal {
		dst = rewriteUrl(dst, userPvc, "", false)
		query.Set("destination", dst)
	} else if dstType == drives.SrcTypeCache {
		if strings.HasPrefix(dst, "/cache") {
			dst = strings.TrimPrefix(dst, "/cache")
			var dstNode string
			dstParts := strings.SplitN(strings.TrimPrefix(dst, "/"), "/", 2)

			if len(dstParts) > 1 {
				dstNode = dstParts[0]
				if len(dst) > len("/"+dstNode) {
					dst = "/AppData" + dst[len("/"+dstNode):]
				} else {
					dst = "/AppData"
				}
				klog.Infoln("Node:", dstNode)
				klog.Infoln("New dst:", dst)
			} else if len(dstParts) > 0 {
				dstNode = dstParts[0]
				dst = "/AppData"
				klog.Infoln("Node:", dstNode)
				klog.Infoln("New dst:", dst)
			}

			if len(node) == 0 {
				fmt.Println("---1---", dstNode)

			}
		} // only for cache for compatible
		dst = rewriteUrl(dst, cachePvc, "/AppData", false)
		query.Set("destination", dst)
	}

	newURL := fmt.Sprintf("%s?%s", src, query.Encode())
	fmt.Println("New WHOLE URL:", newURL)
}
