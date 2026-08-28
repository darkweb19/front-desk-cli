package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Entry struct {
	Time    time.Time
	Message string
}

func main() {

	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Println("Error getting config directory:", err)
		return
	}

	appDir := filepath.Join(configDir, "tm")
	tasksFile := filepath.Join(appDir, "tasks.json")

	err = os.MkdirAll(appDir, 0755)
	if err != nil {
		fmt.Println("Error creating app directory:", err)
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("Please provide an argument.")
		return
	}

	if os.Args[1] == "--generate" {


		fmt.Println("Generate command detected")
		entries, err := loadEntries(tasksFile)
		
		if err != nil {
			fmt.Println("Error loading entries:", err)
			return
		}
		fmt.Println("Entries loaded:", entries)
		return
	}

	entries, err := loadEntries(tasksFile)

	if err != nil {
		fmt.Println("Error loading entries:", err)
		return
	}
	entries = append(entries, Entry{Time: time.Now(), Message: strings.Join(os.Args[1:], " ")})

	file, err := os.Create(tasksFile)
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

func loadEntries(tasksFile string) ([]Entry, error) {

	entries := []Entry{}
	file, err := os.Open(tasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			// this is okay: first run
		} else {
			return nil, fmt.Errorf("error opening file: %v", err)
		}
	} else {
		defer file.Close()

		decoder := json.NewDecoder(file)
		err = decoder.Decode(&entries)
		if err != nil {

			return nil, fmt.Errorf("error decoding JSON: %v", err)
		}

	}
	return entries, nil

}
