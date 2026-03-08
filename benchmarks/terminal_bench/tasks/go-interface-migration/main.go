package main

import "fmt"

func main() {
	fs, err := NewFileStorage("/tmp/storage")
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	inv := NewInventory(fs)
	_ = inv
	fmt.Println("storage ready")
}
