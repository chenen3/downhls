# downhls

This repository implements a HLS downloader, it is forked from [oopsguy/m3u8](https://github.com/oopsguy/m3u8). I made some changes to meet my needs.

```sh
Usage of downhls:
  -c int
    	Maximum number of occurrences (default 25)
  -o string
    	Output file path, required
  -u string
    	M3U8 URL, required

Example:
	downhls -u http://example.com/index.m3u8 -o example.ts
```
