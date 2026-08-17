package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	root := filepath.Join(".", "assets", "uploads")

	scanner := bufio.NewScanner(os.Stdin)
	go readIn(cancel, scanner)

	rename(ctx, root)
	sort(ctx, root)
	remove(ctx, root)
}

func remove(ctx context.Context, root string) {

	jobs := make(chan string, numWorkers)

	workers := makeWork(ctx)
	workers.initDeleteWorkers(jobs)
	err := readMyDir(ctx, root, jobs)
	if err != nil {
		fmt.Printf("Error walking dir: %v", err.Error())
	}

	workers.wait(jobs)
	fmt.Println("done??")
}

func deleteWorker(ctx context.Context, jobs <-chan string, wg *sync.WaitGroup, errChan chan<- errInfo, i int) {
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
				err := os.Remove(path)
				if err != nil {
					errChan <- errInfo{"", err, i}
				}
			}
		}
	}
}

func (w *work) initDeleteWorkers(jobs <-chan string) {
	w.errWG.Add(1)
	go printErr(w.errChan, &w.errWG)
	for i := range numWorkers {
		w.workerWG.Add(1)
		go func(workerID int) {
			fmt.Println("worker started")
			deleteWorker(w.ctx, jobs, &w.workerWG, w.errChan, i)
		}(i)
	}
}
