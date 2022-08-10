# downhls

This repository implements a tool for downloading HTTP Live Streaming (HLS), 
it is forked from [oopsguy/m3u8](https://github.com/oopsguy/m3u8).
I made some changes to convert TS files to MP4 files using ffmpeg.

## Install

Assuming you have the Golang environment ready:
```
go install github.com/chenen3/downhls@latest
```

## Usage
```sh
Avaiable options:
  -c int
    	Maximum number of occurrences (default 25)
  -i string
    	M3U8 URL, required
  -o string
    	Output file path, required

Examples:
  output to TS file:
    	downhls -i "http://example.com/index.m3u8" -o output.ts

  output to MP4 file (required ffmpeg):
    	downhls -i "http://example.com/index.m3u8" -o output.mp4
```
