package crdt

import (
	"fmt"
	"testing"
)

func Worker(id int, jobs <-chan string, results chan<- string) {
	for job := range jobs {
		fmt.Printf("Worker %d: processing job %s\n", id, job)
		results <- fmt.Sprintf("Worker %d: processed job %s", id, job)
	}
}

func TestWorkerPool(t *testing.T) {
	jobs := make(chan string, 5)
	results := make(chan string, 5)
	arr := []string{"task1", "task2", "task3", "task4", "task5"}

	for i := 1; i <= 3; i++ {
		go Worker(i, jobs, results)
	}

	for _, task := range arr {
		jobs <- task
	}
	close(jobs)

	for a := 0; a < 5; a++ {
		result := <-results
		fmt.Println(result)
	}
}
