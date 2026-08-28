package request

import (
	"ImageCacheProject/internal/caching"
	"ImageCacheProject/internal/db"
	"ImageCacheProject/internal/upload"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type HandlerManager struct {
	ctx context.Context
	ldb *db.LoveAppDB
	cm  *caching.CacheManager
	um  *upload.Manager
}

type ManifestReq struct {
	Date   int `json:"date"`
	UserID int `json:"UserID"`
}

func StartReqHandling(srv *http.Server, h *HandlerManager) {
	http.HandleFunc("/api/image/", h.imgHandler)
	http.HandleFunc("/api/manifest", h.manifestHandler)
	http.HandleFunc("/api/upload/images", h.uploadHandler)

	fmt.Println("Server started and listening to 8080...")

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

	imgName := strings.TrimPrefix(r.URL.Path, "/api/image/")

	if imgName == "" {
		http.Error(w, "missing image path", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	cacheImgPath, err := h.cm.GetImg(imgName)
	if err == nil {
		err = h.cm.LockFile(cacheImgPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.cm.UnlockFile(cacheImgPath)
		http.ServeFile(w, r, cacheImgPath)
		return
	}

	imgBytes, err := h.cm.LoadNGetImg(imgName)
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
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var req ManifestReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	imgPaths, err := h.ldb.GetNames(req.Date, req.UserID)
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
	err = h.um.SaveUploadedFile(header.Filename, file)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := []string{"/uploads/" + header.Filename}
	json.NewEncoder(w).Encode(response)
}

func NewHandler(ctx context.Context, ldb *db.LoveAppDB, cm *caching.CacheManager, um *upload.Manager) *HandlerManager {
	return &HandlerManager{
		ctx, ldb, cm, um}
}
