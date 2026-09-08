package request

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/caching"
	"ImageCacheProject/internal/db"
	"ImageCacheProject/internal/upload"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/h2non/bimg"
)

type HandlerManager struct {
	ctx           context.Context
	imageDB       db.ImageDB
	tokenDB       db.TokenDB
	userDB        db.UserDB
	cacheManager  *caching.CacheManager
	uploadManager *upload.Manager
	dbHandler     *brocker.DBHandler
	auth          *authManager
}

var (
	ErrAuthHeaderMissing = errors.New("authorization header is missing")
	ErrWrongHeaderFormat = errors.New("wrong header format (expected Bearer <token>)")
)

type ManifestReq struct {
	Date int `json:"date"`
}

type ClientMeta struct {
	CreationTime int64 `json:"creationTime"`
}

type LogInReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceName string `json:"DeviceName"`
}

type RefreshReq struct {
	RefreshToken string `json:"refreshToken"`
}

type RefreshResp struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type LogOutReq struct {
	DeviceName string `json:"DeviceName"`
}

func (h *HandlerManager) subscribeDB() {
	h.dbHandler.InitDBHandler()
	h.dbHandler.Subscribe(brocker.InsertNewImage, h.imageDB)
	h.dbHandler.Subscribe(brocker.RevokeToken, h.tokenDB)
	h.dbHandler.Subscribe(brocker.InsertNewToken, h.tokenDB)
}

func StartReqHandling(srv *http.Server, h *HandlerManager) {
	h.subscribeDB()

	router := chi.NewRouter()
	router.Use(chimiddleware.Logger)
	router.Use(chimiddleware.Recoverer)

	router.Group(func(r chi.Router) {
		r.Post("/auth/register", h.handleSignIn)
		r.Post("/auth/login", h.handleLogIn)
		r.Post("/auth/refresh", h.handleRefresh)

		r.Get("/api/image/*", h.handleImage)
	})

	router.Group(func(r chi.Router) {
		r.Use(h.auth.AuthMiddleware)

		r.Post("/api/manifest", h.handleManifest)
		r.Post("/api/random/manifest", h.handleRandomManifest)
		r.Get("/api/random/image", h.handleRandomImage)
		r.Post("/api/library/refresh", h.handleLibRefresh)
		r.Post("/api/upload/images", h.handleUpload)
		r.Post("/auth/logout", h.handleLogOut)
	})

	srv.Handler = router

	go startListening(srv)
}

func startListening(srv *http.Server) {
	err := srv.ListenAndServe()
	if err != nil {
		err = fmt.Errorf("error starting server %v", err)
	}
}

func (h *HandlerManager) handleImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	imgName := chi.URLParam(r, "*")

	if imgName == "" {
		http.Error(w, "missing image path", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	cOpt := GetImgOptions(r)

	cacheImgPath, err := h.cacheManager.GetImg(imgName, cOpt.Category)
	if err == nil {
		rFunc := func(path string) { http.ServeFile(w, r, path) }
		err = h.cacheManager.UseFile(cacheImgPath, rFunc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	imgBytes, err := h.cacheManager.LoadNGetImg(imgName, cOpt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", http.DetectContentType(imgBytes))
	_, err = w.Write(imgBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *HandlerManager) handleManifest(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var req ManifestReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	imgPaths, err := h.imageDB.GetNames(req.Date, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(imgPaths); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *HandlerManager) handleUpload(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "unable to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	tNet := time.Since(t0)
	t1 := time.Now()
	path, callback, err := h.uploadManager.SaveUploadedFile(header.Filename, file)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	tDisk := time.Since(t1)
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	t2 := time.Now()

	clientDate := extractDate(r)
	imgName := filepath.Base(path)
	img := db.ImgData{UserID: userID, Path: imgName, Date: clientDate, Callback: callback}
	h.dbHandler.Publish(img, brocker.InsertNewImage)
	tDB := time.Since(t2)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := []string{"/uploads/" + header.Filename}
	json.NewEncoder(w).Encode(response)
	fmt.Println("net time: " + tNet.String() + " disk time: " + tDisk.String() + " db time: " + tDB.String())
}

func (h *HandlerManager) handleRefresh(w http.ResponseWriter, r *http.Request) {

	var req RefreshReq
	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}

	dbinfo, err := h.tokenDB.GetRefreshTokenInfo(req.RefreshToken)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if dbinfo.IsRevoked {
		http.Error(w, "token is revoked", http.StatusUnauthorized)
		return
	}

	info, err := h.auth.authenticator.CheckToken(req.RefreshToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	access, refresh, err := h.auth.authenticator.GetUserTokens(info.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newRTokenInfo, err := h.auth.authenticator.CheckToken(refresh)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ud := db.TokenData{UserID: newRTokenInfo.UserID, DeviceName: dbinfo.DeviceName, RefreshToken: refresh, Exp: newRTokenInfo.Exp, Iat: newRTokenInfo.Iat}
	h.dbHandler.Publish(ud, brocker.InsertNewToken)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RefreshResp{RefreshToken: refresh, AccessToken: access})
}

func (h *HandlerManager) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var req LogInReq
	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	userID, err := h.userDB.RegisterNewUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	access, refresh, err := h.auth.authenticator.GetUserTokens(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	rInfo, err := h.auth.authenticator.CheckToken(refresh)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rData := db.TokenData{UserID: userID, DeviceName: req.DeviceName, RefreshToken: refresh, Exp: rInfo.Exp, Iat: rInfo.Iat, IsRevoked: false}
	h.dbHandler.Publish(rData, brocker.InsertNewToken)
	json.NewEncoder(w).Encode(RefreshResp{AccessToken: access, RefreshToken: refresh})

}

func (h *HandlerManager) handleLogIn(w http.ResponseWriter, r *http.Request) {
	var req LogInReq
	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	userID, err := h.userDB.CheckUserPassword(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	access, refresh, err := h.auth.authenticator.GetUserTokens(userID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rInfo, err := h.auth.authenticator.CheckToken(refresh)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rData := db.TokenData{UserID: userID, DeviceName: req.DeviceName, RefreshToken: refresh, Exp: rInfo.Exp, Iat: rInfo.Iat, IsRevoked: false}
	h.dbHandler.Publish(rData, brocker.InsertNewToken)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RefreshResp{AccessToken: access, RefreshToken: refresh})
}

func (h *HandlerManager) handleLogOut(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req LogOutReq
	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	logoutInfo := db.UserLogOut{UserID: userID, DeviceName: req.DeviceName}
	h.dbHandler.Publish(logoutInfo, brocker.RevokeToken)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (h *HandlerManager) handleLibRefresh(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	imgNames, err := h.imageDB.GetLibsNames(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(imgNames); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *HandlerManager) handleRandomManifest(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	defer r.Body.Close()

	dates, err := h.imageDB.GetDates(userID)
	if err != nil {
		http.Error(w, "user has no photos", http.StatusBadRequest)
		return
	}
	date := dates[rand.N(len(dates))]
	imgPaths, err := h.imageDB.GetNames(date, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(imgPaths); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *HandlerManager) handleRandomImage(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")

	defer r.Body.Close()
	cOpt := GetImgOptions(r)

	imgNames, err := h.imageDB.GetLibsNames(userID)
	if err != nil {
		http.Error(w, "user has no photos", http.StatusBadRequest)
		return
	}
	imgName := imgNames[rand.N(len(imgNames))]
	cacheImgPath, err := h.cacheManager.GetImg(imgName, cOpt.Category)
	if err == nil {
		rFunc := func(path string) { http.ServeFile(w, r, path) }
		err = h.cacheManager.UseFile(cacheImgPath, rFunc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	imgBytes, err := h.cacheManager.LoadNGetImg(imgName, cOpt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", http.DetectContentType(imgBytes))
	_, err = w.Write(imgBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func NewHandler(ctx context.Context, cm *caching.CacheManager, um *upload.Manager, handler *brocker.DBHandler, tdb db.TokenDB, ldb db.ImageDB, udb db.UserDB) *HandlerManager {
	auth := NewAuthenticator()

	return &HandlerManager{
		ctx: ctx, imageDB: ldb, tokenDB: tdb, userDB: udb, cacheManager: cm, uploadManager: um, dbHandler: handler, auth: &authManager{auth}}
}

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
