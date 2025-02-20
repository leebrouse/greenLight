package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup

	 for i := 0; i < 3; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			fmt.Println("Hello m in goroutine",i)
		}()
	} 

	wg.Wait()
	fmt.Println("Everying was done!!!")
}
