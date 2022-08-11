package dl

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/chenen3/downhls/parse"
	"github.com/chenen3/downhls/tool"
)

const (
	tsExt            = ".ts"
	tsTempFileSuffix = "_tmp"
	progressWidth    = 40
)

type Downloader struct {
	lock   sync.Mutex
	queue  []int
	output string
	tmpDir string
	finish int32
	segLen int

	result *parse.Result
}

// NewTask returns a Task instance
func NewTask(output string, url string) (*Downloader, error) {
	result, err := parse.FromURL(url)
	if err != nil {
		return nil, err
	}
	tmpDir, err := os.MkdirTemp("", "downhls*")
	if err != nil {
		return nil, err
	}
	d := &Downloader{
		output: output,
		tmpDir: tmpDir,
		result: result,
	}
	d.segLen = len(result.M3u8.Segments)
	d.queue = genSlice(d.segLen)
	return d, nil
}

// Start runs downloader
func (d *Downloader) Start(concurrency int) error {
	var wg sync.WaitGroup
	// struct{} zero size
	limitChan := make(chan struct{}, concurrency)
	for {
		tsIdx, end, err := d.next()
		if err != nil {
			if end {
				break
			}
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := d.download(idx); err != nil {
				// Back into the queue, retry request
				fmt.Printf("[failed] %s\n", err.Error())
				if err := d.back(idx); err != nil {
					fmt.Println(err)
				}
			}
			<-limitChan
		}(tsIdx)
		limitChan <- struct{}{}
	}
	wg.Wait()
	return d.merge()
}

func (d *Downloader) download(segIndex int) error {
	tsFilename := tsFilename(segIndex)
	tsUrl := d.tsURL(segIndex)
	b, e := tool.Get(tsUrl)
	if e != nil {
		return fmt.Errorf("request %s, %s", tsUrl, e.Error())
	}
	//noinspection GoUnhandledErrorResult
	defer b.Close()
	fPath := filepath.Join(d.tmpDir, tsFilename)
	fTemp := fPath + tsTempFileSuffix
	f, err := os.Create(fTemp)
	if err != nil {
		return fmt.Errorf("create file: %s, %s", tsFilename, err.Error())
	}
	bytes, err := ioutil.ReadAll(b)
	if err != nil {
		return fmt.Errorf("read bytes: %s, %s", tsUrl, err.Error())
	}
	sf := d.result.M3u8.Segments[segIndex]
	if sf == nil {
		return fmt.Errorf("invalid segment index: %d", segIndex)
	}
	key, ok := d.result.Keys[sf.KeyIndex]
	if ok && key != "" {
		bytes, err = tool.AES128Decrypt(bytes, []byte(key),
			[]byte(d.result.M3u8.Keys[sf.KeyIndex].IV))
		if err != nil {
			return fmt.Errorf("decryt: %s, %s", tsUrl, err.Error())
		}
	}
	// https://en.wikipedia.org/wiki/MPEG_transport_stream
	// Some TS files do not start with SyncByte 0x47, they can not be played after merging,
	// Need to remove the bytes before the SyncByte 0x47(71).
	syncByte := uint8(71) //0x47
	bLen := len(bytes)
	for j := 0; j < bLen; j++ {
		if bytes[j] == syncByte {
			bytes = bytes[j:]
			break
		}
	}
	w := bufio.NewWriter(f)
	if _, err := w.Write(bytes); err != nil {
		return fmt.Errorf("write to %s: %s", fTemp, err.Error())
	}
	// Release file resource to rename file
	_ = f.Close()
	if err = os.Rename(fTemp, fPath); err != nil {
		return err
	}
	// Maybe it will be safer in this way...
	atomic.AddInt32(&d.finish, 1)
	//tool.DrawProgressBar("Downloading", float32(d.finish)/float32(d.segLen), progressWidth)
	fmt.Printf("[download %6.2f%%] %s\n", float32(d.finish)/float32(d.segLen)*100, tsUrl)
	return nil
}

func (d *Downloader) next() (segIndex int, end bool, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if len(d.queue) == 0 {
		err = fmt.Errorf("queue empty")
		if d.finish == int32(d.segLen) {
			end = true
			return
		}
		// Some segment indexes are still running.
		end = false
		return
	}
	segIndex = d.queue[0]
	d.queue = d.queue[1:]
	return
}

func (d *Downloader) back(segIndex int) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if sf := d.result.M3u8.Segments[segIndex]; sf == nil {
		return fmt.Errorf("invalid segment index: %d", segIndex)
	}
	d.queue = append(d.queue, segIndex)
	return nil
}

func (d *Downloader) merge() error {
	// In fact, the number of downloaded segments should be equal to number of m3u8 segments
	missingCount := 0
	for idx := 0; idx < d.segLen; idx++ {
		tsFilename := tsFilename(idx)
		f := filepath.Join(d.tmpDir, tsFilename)
		if _, err := os.Stat(f); err != nil {
			missingCount++
		}
	}
	if missingCount > 0 {
		fmt.Printf("[warning] %d files missing\n", missingCount)
	}

	switch filepath.Ext(d.output) {
	case ".ts":
		// Create a TS file for merging, all segment files will be written to this file.
		mFile, err := os.Create(d.output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %s", err.Error())
		}
		defer mFile.Close()
		mergedCount := 0
		for segIndex := 0; segIndex < d.segLen; segIndex++ {
			tsFilename := tsFilename(segIndex)
			f, err := os.Open(filepath.Join(d.tmpDir, tsFilename))
			if err != nil {
				fmt.Printf("merging: %s\n", err)
				continue
			}
			_, err = io.Copy(mFile, f)
			f.Close()
			if err != nil {
				fmt.Printf("merging: %s\n", err)
				continue
			}
			mergedCount++
			tool.DrawProgressBar("merge", float32(mergedCount)/float32(d.segLen), progressWidth)
		}
		if mergedCount != d.segLen {
			fmt.Printf("merge: %d files failed\n", d.segLen-mergedCount)
		}
	case ".mp4":
		var tsFiles []string
		for segIndex := 0; segIndex < d.segLen; segIndex++ {
			tsFiles = append(tsFiles, filepath.Join(d.tmpDir, tsFilename(segIndex)))
		}
		concat := strings.Join(tsFiles, "|")
		cmd := exec.Command("ffmpeg", "-i", "concat:"+concat, "-c", "copy", d.output)
		err := cmd.Run()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported file extension: %s", d.output)
	}

	_ = os.RemoveAll(d.tmpDir)
	fmt.Printf("\n[output] %s\n", d.output)
	return nil
}

func (d *Downloader) tsURL(segIndex int) string {
	seg := d.result.M3u8.Segments[segIndex]
	return tool.ResolveURL(d.result.URL, seg.URI)
}

func tsFilename(ts int) string {
	return strconv.Itoa(ts) + tsExt
}

func genSlice(len int) []int {
	s := make([]int, 0)
	for i := 0; i < len; i++ {
		s = append(s, i)
	}
	return s
}
