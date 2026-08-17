package util

//
//import (
//	"errors"
//)
//
//var StartCapacityRC uint64 = 64 * 1024 * 1024
//var StartCapacityDC uint64 = 256 * 1024 * 1024
//
//var MegaByte uint64 = 1024 * 1024
//
//type CapStatus int
//
//const (
//	StatusNEW CapStatus = iota
//	StatusINIT
//	StatusUSED
//	StatusFULL
//	StatusDEAD
//)
//
//func (cs *CapStatus) String() string {
//	switch *cs {
//	case StatusNEW:
//		return "NEW"
//	case StatusUSED:
//		return "USED"
//	case StatusFULL:
//		return "FULL"
//	default:
//		return "DEAD"
//	}
//}
//
//type cacheKey struct {
//	userID int
//	date   int
//}
//
//type CacheTable struct {
//	PhotoMap    map[cacheKey]ImgHolder
//	TableStatus CapStatus
//	buff        cacheBuffer
//}
//
//type ImgHolder interface {
//	GetDate() int
//	GetUserID() int
//	GetNames() []string
//	GetImage(name string, userID int) ([]byte, error)
//
//}
//
//type cacheBuffer interface {
//	GetFileOffset() int64
//}
//
//func (ct *CacheTable) GetListDesc(date int, userID int) (ImgHolder, error) {
//	key := cacheKey{userID: userID, date: date}
//	pld, ok := ct.PhotoMap[key]
//	if !ok {
//		return nil, errors.New("ld not found")
//	}
//	return pld, nil
//}
//
//func (ct *CacheTable) SetBuff(buffer cacheBuffer) {
//	ct.buff = buffer
//}
