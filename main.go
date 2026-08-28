package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Entry struct {
	Time    time.Time
	Message string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide an argument.")
		return
	}

	entries := []Entry{}

	file, err := os.Open("tasks.json")

	if err != nil {
		if os.IsNotExist(err) {
			// this is okay: first run
		} else {
			fmt.Println("Error opening file:", err)
			return
		}
	} else {
		defer file.Close()

		decoder := json.NewDecoder(file)
		err = decoder.Decode(&entries)
		if err != nil {
			fmt.Println("Error decoding JSON:", err.Error())
			file.Close()
			return
		}
		
	}
	entries = append(entries, Entry{Time: time.Now(), Message: strings.Join(os.Args[1:], " ")})

	file,err = os.Create("tasks.json") 
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(entries)
	if err != nil {
		fmt.Println("Error encoding JSON:", err.Error())
		return
	}

	fmt.Println(entries)

}
