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
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
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
		r.Post("/auth/refresh", h.refreshHandler)

		r.Get("/api/image/*", h.imgHandler)
	})

	router.Group(func(r chi.Router) {
		r.Use(h.auth.AuthMiddleware)

		r.Post("/api/manifest", h.manifestHandler)
		r.Post("/api/upload/images", h.uploadHandler)
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

func (h *HandlerManager) imgHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	imgName := chi.URLParam(r, "*")

	if imgName == "" {
		http.Error(w, "missing image path", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	cacheImgPath, err := h.cacheManager.GetImg(imgName)
	if err == nil {
		err = h.cacheManager.LockFile(cacheImgPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.cacheManager.UnlockFile(cacheImgPath)
		http.ServeFile(w, r, cacheImgPath)
		return
	}

	imgBytes, err := h.cacheManager.LoadNGetImg(imgName)
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

func (h *HandlerManager) manifestHandler(w http.ResponseWriter, r *http.Request) {
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

func (h *HandlerManager) uploadHandler(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "unable to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	path, callback, err := h.uploadManager.SaveUploadedFile(header.Filename, file)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	img, err := db.MakeImgData(path, userID, callback)
	if err != nil {
		http.Error(w, "Error opening file", http.StatusInternalServerError)
		return
	}
	h.dbHandler.Publish(img, brocker.InsertNewImage)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := []string{"/uploads/" + header.Filename}
	json.NewEncoder(w).Encode(response)
}

func (h *HandlerManager) refreshHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshReq
	defer r.Body.Close()

	errt := json.NewDecoder(r.Body).Decode(&req)
	if errt != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}

	dbinfo, errdb := h.tokenDB.GetRefreshTokenInfo(req.RefreshToken)

	if errdb != nil {
		http.Error(w, errdb.Error(), http.StatusUnauthorized)
		return
	}

	if dbinfo.IsRevoked {
		http.Error(w, "token is revoked", http.StatusUnauthorized)
		return
	}

	info, errauth := h.auth.authenticator.CheckToken(req.RefreshToken)
	if errauth != nil {
		http.Error(w, errauth.Error(), http.StatusUnauthorized)
		return
	}

	access, refresh, errt := h.auth.authenticator.GetUserTokens(info.UserID)
	if errt != nil {
		http.Error(w, errt.Error(), http.StatusInternalServerError)
		return
	}
	newRTokenInfo, errtn := h.auth.authenticator.CheckToken(refresh)
	if errtn != nil {
		http.Error(w, errtn.Error(), http.StatusInternalServerError)
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

func NewHandler(ctx context.Context, cm *caching.CacheManager, um *upload.Manager, handler *brocker.DBHandler, tdb db.TokenDB, ldb db.ImageDB, udb db.UserDB) *HandlerManager {
	auth := NewAuthenticator()

	return &HandlerManager{
		ctx: ctx, imageDB: ldb, tokenDB: tdb, userDB: udb, cacheManager: cm, uploadManager: um, dbHandler: handler, auth: &authManager{auth}}
}
