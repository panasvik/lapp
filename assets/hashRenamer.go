package main

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

var numWorkers = 4

type errInfo struct {
	path string
	err  error
	i    int
}

type work struct {
	ctx      context.Context
	workerWG sync.WaitGroup
	errWG    sync.WaitGroup
	errChan  chan errInfo
}

//func main() {
//	ctx, cancel := context.WithCancel(context.Background())
//	scanner := bufio.NewScanner(os.Stdin)
//	go readIn(cancel, scanner)
//
//	jobs := make(chan string, numWorkers)
//
//	workers := makeWork(ctx)
//	workers.initWorkers(jobs)
//	root := filepath.Join(".", "assets", "uploads")
//	err := walkMyDir(ctx, root, jobs)
//	if err != nil {
//		fmt.Printf("Error walking dir: %v", err.Error())
//	}
//
//	workers.wait(jobs)
//	fmt.Println("done??")
//}

func rename(ctx context.Context, root string) {
	jobs := make(chan string, numWorkers)
	workers := makeWork(ctx)
	workers.initRenameWorkers(jobs)
	err := walkMyDir(ctx, root, jobs)
	if err != nil {
		fmt.Printf("Error walking dir: %v", err.Error())
	}

	workers.wait(jobs)
	fmt.Println("done??")
}

func renameFile(ctx context.Context, jobs <-chan string, wg *sync.WaitGroup, errChan chan<- errInfo, i int) {
	defer func() {
		wg.Done()
		fmt.Printf("worker %d done\n", i)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-jobs:
			{
				if !ok {
					return
				}
				bytes, err := os.ReadFile(path)
				if err != nil {
					errChan <- errInfo{err: err,
						path: path, i: i}
					continue
				}
				h := fnv.New64a()
				h.Write(bytes)
				hashValue := h.Sum64()
				ext := filepath.Ext(path)
				str := strconv.FormatUint(hashValue, 10)
				newName := str + ext
				dir := filepath.Dir(path)

				newPath := filepath.Join(dir, newName)
				if path == newPath {
					continue
				}
				err = os.Rename(path, newPath)
				if err != nil {
					errChan <- errInfo{err: err,
						path: path, i: i}
				}
			}
		}
	}

}

func printErr(errChan <-chan errInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	for errI := range errChan {
		if errI.err != nil {
			fmt.Printf("worker %d failed renaming %s: %v\n", errI.i, errI.path, errI.err.Error())
		}
	}

}

func readIn(cancel context.CancelFunc, scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		if line == "exit" {
			cancel()
			break
		}
	}

}
func makeWork(ctx context.Context) *work {
	return &work{
		ctx:     ctx,
		errChan: make(chan errInfo, numWorkers),
	}
}

func (w *work) initRenameWorkers(jobs <-chan string) {
	w.errWG.Add(1)
	go printErr(w.errChan, &w.errWG)
	for i := range numWorkers {
		w.workerWG.Add(1)
		go func(workerID int) {
			fmt.Println("worker started")
			renameFile(w.ctx, jobs, &w.workerWG, w.errChan, i)
		}(i)
	}
}

func (w *work) wait(jobs chan string) {
	close(jobs)
	fmt.Println("waiting")

	w.workerWG.Wait()
	close(w.errChan)
	w.errWG.Wait()
}

func walkMyDir(ctx context.Context, root string, jobs chan<- string) error {
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		select {
		case jobs <- path:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	return err
}

func readMyDir(ctx context.Context, root string, jobs chan<- string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Printf("Error reading dir: %v\n", err)
	} else {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			path := filepath.Join(root, entry.Name())

			select {
			case jobs <- path:
			case <-ctx.Done():
				break
			}
		}
	}
	return err
}
