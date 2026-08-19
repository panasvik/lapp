package request

import (
	"ImageCacheProject/internal/caching"
	"ImageCacheProject/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type HandlerManager struct {
	ctx context.Context
	ldb *db.LoveAppDB
	cm  *caching.CacheManager
	fmu *caching.CacheTable
}

type ManifestReq struct {
	Date   int `json:"date"`
	UserID int `json:"id"`
}

type imgReq struct {
	Path string `json:"path"`
}

func StartReqHandling(srv *http.Server, h *HandlerManager) {
	http.HandleFunc("/api/image", h.imgHandler)
	http.HandleFunc("/api/dates", h.manifestHandler)

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
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var req imgReq

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	cacheImgPath, err := h.cm.GetImg(req.Path)
	if err == nil {
		err = h.cm.LockFile(req.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.ServeFile(w, r, cacheImgPath)
		h.cm.UnlockFile(req.Path)
		return
	}

	imgBytes, err := h.cm.LoadNGetImg(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

func NewHandler(ctx context.Context, ldb *db.LoveAppDB, cm *caching.CacheManager, fmu *caching.CacheTable) *HandlerManager {
	return &HandlerManager{
		ctx, ldb, cm, fmu}
}
