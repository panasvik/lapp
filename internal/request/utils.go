package request

import (
	"ImageCacheProject/internal/caching"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/h2non/bimg"
)

const Domain string = "pcloudcom.tech"

type contextKey string

func GetImgOptions(r *http.Request) caching.Options {
	u := r.URL.Query()
	Category := GetIntQueryParam(u, "img_type", 0, func(val int) bool { return true })
	Width := GetIntQueryParam(u, "width", 100, func(val int) bool { return val > 0 })
	Height := GetIntQueryParam(u, "height", 100, func(val int) bool { return val > 0 })
	Quality := GetIntQueryParam(u, "quality", 75, func(val int) bool { return val > 0 && val <= 100 })
	Type := GetBimgTypeParam(u, "type")
	return caching.Options{
		Category: caching.ImageCategory(Category), BimgOpt: bimg.Options{
			Width: Width, Height: Height, Quality: Quality,
			Type: Type, StripMetadata: true,
			Crop: true, Gravity: bimg.GravitySmart}}

}

func GetIntQueryParam(u url.Values, key string, defaultVal int, cond func(val int) bool) int {
	valS := u.Get(key)
	val, err := strconv.Atoi(valS)
	if err != nil || !cond(val) {
		val = defaultVal
	}
	return val
}

func GetBimgTypeParam(u url.Values, key string) bimg.ImageType {
	valS := u.Get(key)
	switch {
	case valS == "JPEG":
		return bimg.JPEG
	case valS == "PNG":
		return bimg.PNG
	}
	return bimg.JPEG
}

func extractDate(r *http.Request) (clientDate int) {
	if metaStr := r.FormValue("metadata"); metaStr != "" {
		var cm ClientMeta
		if err := json.Unmarshal([]byte(metaStr), &cm); err == nil && cm.CreationTime > 0 {
			t := time.UnixMilli(cm.CreationTime)
			clientDate, _ = strconv.Atoi(t.Format("20060102"))
		} else {
			clientDate, _ = strconv.Atoi(time.Now().Format("20060102"))
		}
	}
	return clientDate
}
