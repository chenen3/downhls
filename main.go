package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenen3/downhls/dl"
)

var (
	url      string
	output   string
	chanSize int
)

func init() {
	flag.StringVar(&url, "i", "", "M3U8 URL, required")
	flag.IntVar(&chanSize, "c", 25, "Maximum number of occurrences")
	flag.StringVar(&output, "o", "", "Output file path, required")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Downhls is a tool for downloading HTTP Live Streaming.\n\n")
		fmt.Fprintln(flag.CommandLine.Output(), "Usage:")
		flag.PrintDefaults()
		fmt.Fprint(flag.CommandLine.Output(), `
Examples:
  output to transport stream file:
    	downhls -i http://example.com/index.m3u8 -o output.ts

  output to mp4 file (required ffmpeg):
    	downhls -i http://example.com/index.m3u8 -o output.mp4
`)
	}
}

func main() {
	flag.Parse()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[error]", r)
			os.Exit(-1)
		}
	}()
	if url == "" {
		flag.Usage()
		return
	}
	if output == "" {
		flag.Usage()
		return
	}
	if ext := filepath.Ext(output); ext != ".ts" && ext != ".mp4" {
		panic("the output file name extension must be .ts or .mp4")
	}
	if chanSize <= 0 {
		panic("parameter 'c' must be greater than 0")
	}
	downloader, err := dl.NewTask(output, url)
	if err != nil {
		panic(err)
	}
	if err := downloader.Start(chanSize); err != nil {
		panic(err)
	}
	fmt.Println("Done!")
}
