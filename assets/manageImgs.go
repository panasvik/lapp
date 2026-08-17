package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	numDirs = 32
)

//
//func main() {
//	ctx, cancel := context.WithCancel(context.Background())
//	root := filepath.Join(".", "assets", "uploads")
//
//	//b := createDirs(root)
//	//if !b {
//	//	fmt.Println("unable to make dirs")
//	//	return
//	//}
//	scanner := bufio.NewScanner(os.Stdin)
//	go readIn(cancel, scanner)
//
//	jobs := make(chan string, numWorkers)
//
//	workers := makeWork(ctx)
//	workers.initSortWorkers(jobs)
//	err := walkMyDir(ctx, root, jobs)
//	if err != nil {
//		fmt.Printf("Error walking dir: %v", err.Error())
//	}
//
//	workers.wait(jobs)
//	fmt.Println("done??")
//}

func sort(ctx context.Context, root string) {
	b := createDirs(root)
	if !b {
		fmt.Println("unable to make dirs")
		return
	}
	jobs := make(chan string, numWorkers)

	workers := makeWork(ctx)
	workers.initSortWorkers(jobs)
	err := readMyDir(ctx, root, jobs)
	if err != nil {
		fmt.Printf("Error walking dir: %v", err.Error())
	}

	workers.wait(jobs)
	fmt.Println("done??")
}

func createDirs(root string) bool {
	flag := true
	for i := range numDirs {
		path := filepath.Join(root, strconv.Itoa(i))
		err := os.Mkdir(path, 0664)
		if err != nil {
			fmt.Println("unable to mkdir: ", err.Error())
			flag = false
		}
	}
	return flag
}

func (w *work) initSortWorkers(jobs <-chan string) {
	w.errWG.Add(1)
	go printErr(w.errChan, &w.errWG)
	for i := range numWorkers {
		w.workerWG.Add(1)
		go func(workerID int) {
			fmt.Println("worker started")
			sortWorker(w.ctx, jobs, &w.workerWG, w.errChan, i)
		}(i)
	}
}

func sortWorker(ctx context.Context, jobs <-chan string, wg *sync.WaitGroup, errChan chan<- errInfo, i int) {
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
				err := copyContent(path)
				if err != nil {
					errChan <- errInfo{"", err, i}
				}
			}
		}
	}
}

func copyContent(path string) error {
	name := filepath.Base(path)
	dir := filepath.Dir(path)

	onlyName := strings.TrimSuffix(name, filepath.Ext(name))
	name64, err := strconv.ParseUint(onlyName, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to turn %s to int: %w", onlyName, err)
	}
	subDir := int(name64 % numDirs)
	newPath := filepath.Join(dir, strconv.Itoa(subDir), name)
	srcFile, err := os.Open(path)
	defer srcFile.Close()
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", path, err)

	}
	dstFile, err := os.Create(newPath)
	defer dstFile.Close()

	if err != nil {
		return fmt.Errorf("unable to open %s: %w", newPath, err)

	}
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("unable to copy data from %s to %s: %w", path, newPath, err)
	}
	return nil
}
