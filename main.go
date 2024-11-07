package main

import (
	"bufio"
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
	"golang.org/x/net/proxy"
)

const version = "1.0"

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
)

type Blob struct {
	Name string `xml:"Name"`
	URL  string `xml:"Url"`
}

type BlobListResp struct {
	Blobs []Blob `xml:"Blobs>Blob"`
}

func readFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func containerExists(client *http.Client, account, container string, verbose bool) bool {
	url := fmt.Sprintf("https://%s.blob.core.windows.net/%s?restype=container", account, container)
	resp, err := client.Head(url)
	if err != nil {
		if verbose {
			fmt.Printf("%sError checking container %s: %v%s\n", colorRed, container, err, colorReset)
		}
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		fmt.Printf("%sContainer \"%s\" found!%s\n", colorGreen, container, colorReset)
		return true
	}
	if verbose {
		fmt.Printf("%sContainer \"%s\" not found, skipping.%s\n", colorRed, container, colorReset)
	}
	return false
}

func downloadFile(client *http.Client, url, filepath string) error {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	outFile, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, response.Body)
	return err
}

func downloadWorker(client *http.Client, jobs <-chan Blob, wg *sync.WaitGroup, verbose bool) {
	defer wg.Done()
	for blob := range jobs {
		if verbose {
			fmt.Printf("Downloading %s to %s\n", blob.URL, blob.Name)
		}
		if err := downloadFile(client, blob.URL, blob.Name); err != nil {
			fmt.Printf("Failed to download %s: %v\n", blob.URL, err)
		} else {
			fmt.Printf("%sSuccessfully downloaded blob file to %s%s\n", colorBlue, blob.Name, colorReset)
		}
	}
}

func main() {
	figure.NewColorFigure("AzBlobGob", "small", "Blue", true).Print()
	color.Blue("\t\t\t\t\t@h0useh3ad\n\n")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interrupt
		fmt.Println("\nExiting...")
		os.Exit(0)
	}()

	account := flag.String("account", "", "Azure Blob Storage account name")
	containersFile := flag.String("containers", "names.txt", "Container names file (default: names.txt)")
	dirPrefixesFile := flag.String("dirprefixes", "names.txt", "Directory prefix name files")
	destinationDir := flag.String("dest", "", "Directory to save downloaded blobs (default: provided account name in current directory)")
	socksProxy := flag.String("socks", "", "SOCKS5 proxy address (e.g., 127.0.0.1:1080)")
	showVersion := flag.Bool("version", false, "Display version information")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	flag.Parse()

	if *showVersion {
		fmt.Printf("Version: %s\n", version)
		return
	}

	if *account == "" || *containersFile == "" || *dirPrefixesFile == "" {
		fmt.Println("Provide the account, containers file, and directory prefixes file.")
		flag.Usage()
		os.Exit(1)
	}

	if *destinationDir == "" {
		*destinationDir = strings.Split(*account, ".")[0]
	}

	containers, err := readFile(*containersFile)
	if err != nil {
		fmt.Printf("Error reading containers file: %v\n", err)
		os.Exit(1)
	}

	dirPrefixes, err := readFile(*dirPrefixesFile)
	if err != nil {
		fmt.Printf("Error reading directory prefixes file: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*destinationDir, os.ModePerm); err != nil {
		fmt.Printf("Error creating destination directory: %v\n", err)
		os.Exit(1)
	}

	// SOCKS proxy support
	var httpClient *http.Client
	if *socksProxy != "" {
		proxyURL, err := url.Parse("socks5://" + *socksProxy)
		if err != nil {
			fmt.Printf("Invalid SOCKS proxy address: %v\n", err)
			os.Exit(1)
		}

		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			fmt.Printf("Error setting up SOCKS proxy: %v\n", err)
			os.Exit(1)
		}

		httpTransport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
		httpClient = &http.Client{Transport: httpTransport, Timeout: 10 * time.Second}
	} else {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	validContainers := []string{}
	for _, container := range containers {
		if containerExists(httpClient, *account, container, *verbose) {
			validContainers = append(validContainers, container)
		}
	}

	for _, container := range validContainers {
		for _, prefix := range dirPrefixes {
			url := fmt.Sprintf("https://%s.blob.core.windows.net/%s?restype=container&comp=list&prefix=%s", *account, container, prefix)
			if *verbose {
				fmt.Printf("%s\nRequesting Blob: %s%s\n", colorYellow, url, colorReset)
			}

			response, err := httpClient.Get(url)
			if err != nil {
				fmt.Printf("Error accessing URL %s: %v\n", url, err)
				continue
			}
			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil {
				fmt.Printf("Error reading response body: %v\n", err)
				continue
			}

			var blobList BlobListResp
			if err := xml.Unmarshal(body, &blobList); err != nil {
				fmt.Printf("Error parsing XML: %v\n", err)
				fmt.Printf("Response Content: %s\n", string(body))
				continue
			}

			if len(blobList.Blobs) == 0 {
				if *verbose {
					fmt.Printf("%s\nPrefix \"%s\" has no blobs in container \"%s\"!%s\n\n", colorRed, prefix, container, colorReset)
				}
				continue
			} else {
				fmt.Printf("%s\nPrefix \"%s\" has blobs in container \"%s\"!%s\n\n", colorGreen, prefix, container, colorReset)
			}

			jobs := make(chan Blob, 10)
			var wg sync.WaitGroup

			for i := 0; i < 10; i++ {
				wg.Add(1)
				go downloadWorker(httpClient, jobs, &wg, *verbose)
			}

			for _, blob := range blobList.Blobs {
				fullPath := filepath.Join(*destinationDir, blob.Name)
				if err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
					fmt.Printf("Error creating directory %s: %v\n", filepath.Dir(fullPath), err)
					continue
				}
				blob.Name = fullPath
				jobs <- blob
			}

			close(jobs)
			wg.Wait()
		}
	}

	color.Green("****** Finished ******")
	fmt.Println()
}
