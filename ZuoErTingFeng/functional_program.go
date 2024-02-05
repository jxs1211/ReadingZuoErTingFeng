package main

import (
	"fmt"
	"strings"
)

func lower(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	for i, v := range s {
		s[i] = strings.ToLower(v)
	}
	return s
}

func lowerFunc(s string) string {
	return strings.ToLower(s)
}

func mapFunc(lowerFunc func(string) string, s []string) []string {
	for i, v := range s {
		s[i] = lowerFunc(v)
	}
	return s
}

func main() {
	data := []string{"HI", "Hello", "WORLD"}
	fmt.Println(lower(data))
	fmt.Println(mapFunc(lowerFunc, data))
}
