package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const outputFileName = "allow-cloudflare-only.conf"

var IP_URLS = [...]string{
	"https://www.cloudflare.com/ips-v4",
	"https://www.cloudflare.com/ips-v6",
	"https://uptimerobot.com/inc/files/ips/IPv4.txt",
	"https://uptimerobot.com/inc/files/ips/IPv6.txt",
}

var CUSTOM_ALLOW_LIST = [...]string{
	"192.168.50.0/24",
}

func getOutputDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "/etc/nginx/conf/", nil
	case "windows":
		return "C:/nginx/conf/", nil
	default:
		return "", errors.New("unsupported platform")
	}
}

func fetchIPList(url string, wg *sync.WaitGroup, mu *sync.Mutex, ipv4List, ipv6List *[]string) {
	defer wg.Done()
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Failed to fetch:", url, "Error:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Failed to fetch:", url, "Status:", resp.Status)
		return
	}

	var list []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		list = append(list, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading response from:", url, "Error:", err)
		return
	}

	mu.Lock()
	if strings.Contains(url, "v4") {
		*ipv4List = append(*ipv4List, list...)
	} else {
		*ipv6List = append(*ipv6List, list...)
	}
	mu.Unlock()
}

func writeFile(outputDir, fileName string, ipv4List, ipv6List []string) error {
	filePath := outputDir + fileName
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	writer.WriteString("# This file was automatically generated on: " + time.Now().String() + "\n")
	writer.WriteString("# This file was automatically generated from the following sources:\n")
	for _, url := range IP_URLS {
		writer.WriteString("# " + url + "\n")
	}

	writer.WriteString("\n# User defined list\n")
	for _, ip := range CUSTOM_ALLOW_LIST {
		writer.WriteString("allow " + ip + ";\n")
	}

	writer.WriteString("\n# IPv4\n")
	for _, ip := range ipv4List {
		writer.WriteString("allow " + ip + ";\n")
	}

	writer.WriteString("\n# IPv6\n")
	for _, ip := range ipv6List {
		writer.WriteString("allow " + ip + ";\n")
	}

	writer.WriteString("\n# Deny all remaining ips\n")
	writer.WriteString("deny all;")

	return writer.Flush()
}

func main() {
	outputDir, err := getOutputDir()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			fmt.Println("Failed to create directory:", err)
			return
		}
	}

	var ipv4List, ipv6List []string
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, url := range IP_URLS {
		wg.Add(1)
		go fetchIPList(url, &wg, &mu, &ipv4List, &ipv6List)
	}

	wg.Wait()

	err = writeFile(outputDir, outputFileName, ipv4List, ipv6List)
	if err != nil {
		fmt.Println("Failed to write file:", err)
	} else {
		fmt.Println("File written successfully!")
	}
}
