package request

import (
	"ImageCacheProject/internal/brocker"
	"ImageCacheProject/internal/util"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type RecIDReq struct {
	RecID   int          `json:"recipientID"`
	MsgType util.MsgType `json:"MsgType"`
}

const groupIDKey contextKey = "groupID"

func (h *HandlerManager) groupMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groupID, err := GroupIDFromURL(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "userID missing", http.StatusBadRequest)
			return
		}
		if !h.isGroupMember(groupID, userID) {
			http.Error(w, "not a group member", http.StatusForbidden)
			return
		}
		ctx := ContextWithGroupID(r.Context(), groupID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GroupIDFromURL(r *http.Request) (int, error) {
	idStr := chi.URLParam(r, "groupID")
	return strconv.Atoi(idStr)
}

func ContextWithGroupID(ctx context.Context, groupID int) context.Context {
	return context.WithValue(ctx, groupIDKey, groupID)
}

func GroupIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(groupIDKey).(int)
	return id, ok
}

func (h *HandlerManager) handleNewGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var groupName string
	err := json.NewDecoder(r.Body).Decode(&groupName)
	if err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	groupID, err := h.db.RegisterNewGroup(groupName, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(groupID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *HandlerManager) handleGetGroupUserIDs(w http.ResponseWriter, r *http.Request) {
	groupID, ok := GroupIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ids, err := h.db.GetGroupUserIDs(groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
func (h *HandlerManager) handleGroupImage(w http.ResponseWriter, r *http.Request) {
	groupID, ok := GroupIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no group id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	rawImgName := chi.URLParam(r, "*")

	if rawImgName == "" {
		http.Error(w, "missing image path", http.StatusBadRequest)
		return
	}

	imgName := filepath.Base(rawImgName)

	isHolder, err := h.db.ImageBelongsToGroup(imgName, groupID)
	if !isHolder {
		http.Error(w, "group does not own the image", http.StatusForbidden)
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

func (h *HandlerManager) handleGroupManifest(w http.ResponseWriter, r *http.Request) {
	groupID, ok := GroupIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no group id", http.StatusBadRequest)
		return
	}
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	var req ManifestReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "wrong JSON format", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	imgNames, err := h.db.GetNamesGroup(req.Date, groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	signedURLs := make([]string, len(imgNames))
	for i, name := range imgNames {
		rawPath := fmt.Sprintf("/group/%d/image/%s", groupID, name)
		signedURLs[i] = h.signer.SignURL(rawPath, userID, 30*time.Minute)
	}

	json.NewEncoder(w).Encode(signedURLs)
}

func (h *HandlerManager) handleGroupUpload(w http.ResponseWriter, r *http.Request) {
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
	groupID, ok := GroupIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	senderID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no group id provided", http.StatusBadRequest)
		return
	}

	clientDate := extractDate(r)
	imgName := filepath.Base(path)
	img := util.ImgData{HolderID: groupID, Holder: util.Group, Path: imgName, Date: clientDate, Callback: callback}
	h.dbHandler.Publish(img, brocker.InsertNewImage)
	w.Header().Set("Content-Type", "application/json")

	response := []string{"/uploads/" + header.Filename}
	json.NewEncoder(w).Encode(response)
	// send refresh
	userIDs, err := h.db.GetGroupUserIDs(groupID)
	if err != nil {
		http.Error(w, "Error saving message", http.StatusInternalServerError)
		return
	}
	for _, userID := range userIDs {
		senderName, _ := h.db.GetUserNameByID(userID)
		groupName, _ := h.db.GetUserNameByID(groupID)
		msg := util.FormNewMessage(
			senderID,
			userID,
			groupID,
			util.LibUpdate,
			fmt.Sprintf("user %s updated the group library %s", senderName, groupName))

		h.dbHandler.Publish(msg, brocker.AddMessage)
	}
}

func (h *HandlerManager) handleGroupsReq(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no group id provided", http.StatusBadRequest)
		return
	}
	names, err := h.db.GetUserGroupNames(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(names); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *HandlerManager) handleGroupsIDReq(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no group id provided", http.StatusBadRequest)
		return
	}
	ids, err := h.db.GetUserGroupIDs(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
func (h *HandlerManager) handleGroupLibRefresh(w http.ResponseWriter, r *http.Request) {
	groupID, ok := GroupIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no group id provided", http.StatusBadRequest)
		return
	}
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	imgNames, err := h.db.GetLibsNamesGroup(groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	signedURLs := make([]string, len(imgNames))
	for i, name := range imgNames {
		// Формируем маршрут именно к групповому обработчику
		rawPath := fmt.Sprintf("/group/%d/image/%s", groupID, name)
		signedURLs[i] = h.signer.SignURL(rawPath, userID, 30*time.Minute)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signedURLs)
}

func (h *HandlerManager) handleGroupMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	groupID, err := GroupIDFromURL(r)
	if err != nil {
		http.Error(w, "no group id provided", http.StatusBadRequest)
		return
	}
	var req RecIDReq
	err = json.NewDecoder(r.Body).Decode(&req)

	if err != nil || req.RecID <= 0 {
		http.Error(w, "invalid recipient id provided", http.StatusBadRequest)
		return
	}

	if req.MsgType == util.Invitation || req.MsgType == util.LibUpdate {
		if !h.isGroupMember(groupID, userID) {
			http.Error(w, "not a group member", http.StatusForbidden)
			return
		}
	}

	senderName, _ := h.db.GetUserNameByID(userID)
	recName, _ := h.db.GetUserNameByID(req.RecID)
	groupName, _ := h.db.GetGroupName(groupID)
	content := fmt.Sprintf("user %s sends you, %s, a message of type %s, group %s", senderName, recName, (&req.MsgType).ToString(), groupName)
	msg := util.FormNewMessage(userID, req.RecID, groupID, req.MsgType, content)
	h.dbHandler.Publish(msg, brocker.AddMessage)

	w.WriteHeader(http.StatusOK)
}
func (h *HandlerManager) handleGroupMessageSync(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	messages, err := h.db.GetUnreadMessages(userID)
	if err != nil {
		fmt.Println("not all messages were ejected from db")
	}
	if err := json.NewEncoder(w).Encode(messages); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, message := range messages {
		message.Status = util.Delivered
		h.dbHandler.Publish(message, brocker.ChangeMessageStatus)
	}
}

func (h *HandlerManager) isGroupMember(groupID int, userID int) bool {
	ids, err := h.db.GetGroupUserIDs(groupID)
	if err != nil {
		fmt.Printf("[groupMiddleware] Error fetching members for group %d: %v\n", groupID, err)
		return false
	}
	isMember := slices.Contains(ids, userID)
	if !isMember {
		fmt.Printf("[groupMiddleware] FORBIDDEN: User %d is NOT in group %d. Existing members: %v\n", userID, groupID, ids)
	}
	return isMember
}
