package request

//
//import (
//	"ImageCacheProject/internal/caching"
//	"ImageCacheProject/internal/db"
//	"context"
//	"log"
//	"net/http"
//	"sync"
//)
//
//type Payload struct {
//	UserID int `json:"userID"`
//	Date   int `json:"date"`
//}
//
//type httpReq struct {
//	w http.ResponseWriter
//	r *http.Request
//}
//
//type WorkerManager struct {
//	db    db.LoveAppDB
//	cache caching.CacheManager
//	wg    sync.WaitGroup
//}
//
//func (w *WorkerManager) imageWorker(ctx context.Context, jobs <-chan string) {
//	defer w.wg.Done()
//	for {
//		select {
//		case <-ctx.Done():
//			log.Printf("work done")
//			return
//		case job, ok := <-jobs:
//			if !ok {
//				log.Printf("no more work")
//				return
//			}
//			err := w.handleImage(job)
//			if err != nil {
//				//TODO: check err
//
//			}
//		}
//	}
//}
//
//func (w *WorkerManager) handleImage(name string) error {
//	path, err := w.cache.GetImg(name)
//	if err == nil {
//		// TODO: fill in http resp and send via ServeFile
//		_ = path
//		return nil
//	}
//	bytes, err := w.cache.LoadNGetImg(name)
//	if err != nil {
//		//	TODO: check error
//		return err
//	}
//	//TODO: fill in http resp + image as bytes
//	_ = bytes
//	return err
//}
