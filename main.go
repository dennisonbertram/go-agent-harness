package main

import (
	"encoding/json"
	"fmt"
	"log"
)

func main() {
	raw := []byte(`{"user_id":42,"user_name":"alice","is_active":true,"created_at":"2024-01-15"}`)
	var r Response
	if err := json.Unmarshal(raw, &r); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ID=%d Name=%s Active=%v\n", r.UserID, r.UserName, r.IsActive)
}
