package watcher

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"slices"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	ignoreFile = ".rignore"
)

var (
	ignoreList map[string]bool = make(map[string]bool)
)

type fileTracker struct {
	wd2Children map[int][]int
	wd2Path     map[int]string
	path2Wd     map[string]int
}

func (ft *fileTracker) GetPathByWd(wd int) (string, bool) {
	path, ok := ft.wd2Path[wd]
	return path, ok
}

func (ft *fileTracker) GetWdByPath(path string) (int, bool) {
	wd, ok := ft.path2Wd[path]
	return wd, ok
}

func (ft *fileTracker) GetWdChildren(parentWd int) []int {
	return ft.wd2Children[parentWd]
}

func addFile(ft *fileTracker, wd int, filePath string, parentWd int) {
	ft.wd2Children[parentWd] = append(ft.wd2Children[parentWd], wd)
	ft.wd2Path[wd] = filePath
	ft.path2Wd[filePath] = wd
}

func removeDir(ft *fileTracker, wd int) {
	if filePath, ok := ft.wd2Path[wd]; ok {
		delete(ft.wd2Path, wd)
		delete(ft.path2Wd, filePath)
	}

	delete(ft.wd2Children, wd)
}

func removeChildDir(ft *fileTracker, childWd int, parentWd int) {
	if _, parentExists := ft.wd2Children[parentWd]; !parentExists {
		return
	}

	ft.wd2Children[parentWd] = slices.DeleteFunc(ft.wd2Children[parentWd], func(n int) bool {
		return n == childWd
	})
}

func StartWatcher(ch chan int) {
	fd, err := unix.InotifyInit()
	if err != nil {
		log.Fatalf("Inotify init error: %s\n", err)
	}

	if _, err := os.Stat(ignoreFile); err == nil {
		fileBytes, readFileErr := os.ReadFile(ignoreFile)
		if readFileErr == nil {
			for _, f := range strings.Split(string(fileBytes), "\n") {
				ignoreList[f] = true
			}
		}

	}

	ft := &fileTracker{
		wd2Children: make(map[int][]int),
		wd2Path:     make(map[int]string),
		path2Wd:     make(map[string]int),
	}

	watchDir(fd, -1, ".", ft)

	// 1. read can contain one or more unix.InotifyEvent so we create space for 20
	// 2. inotify_event structure differs from unix.InotifyEvent: it also has char name[] at the end. That's why we add unix.NAME_MAX to unix.SizeofInotifyEvent
	//    see https://linux.die.net/man/7/inotify
	// 3. since char name[] is null-terminated we also add 1 bytes for '\0'
	// 4. and most important - I might be wrong about everything above
	var buf [(unix.SizeofInotifyEvent + unix.NAME_MAX + 1) * 20]byte
	for {
		bytesRead, readErr := unix.Read(fd, buf[:])
		if readErr != nil {
			fmt.Println("Read event error", readErr)
			continue
		}

		eventCounter := 0

		offset := 0
		for offset < bytesRead {
			// to deserialize InotifyEvent we have 2 options:
			// 1. use unsafe and get pointer to the event structure
			// 2. determine system endianness and then read the whole structure or its fields by offset (for example, binary.LittleEndian.Uint32(buf[4:8]) to get a mask)
			event := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))

			nameBuf := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(event.Len)]
			fileName := string(bytes.TrimRight(nameBuf, "\x00"))

			switch {
			/*
				new file / dir
			*/
			case event.Mask&unix.IN_CREATE == unix.IN_CREATE ||
				event.Mask&unix.IN_MOVED_TO == unix.IN_MOVED_TO:
				{
					eventCounter += 1

					currDir, _ := ft.GetPathByWd(int(event.Wd))
					filePath := path.Join(currDir, fileName)

					fmt.Println(currDir, fileName)

					stat, err := os.Stat(filePath)
					if err != nil {
						fmt.Println("Stats error", err)
						offset += int(unix.SizeofInotifyEvent + event.Len)
						continue
					}

					if !stat.IsDir() {
						offset += int(unix.SizeofInotifyEvent + event.Len)
						continue
					}

					wd, watchDescErr := unix.InotifyAddWatch(fd, filePath,
						unix.IN_CREATE|
							unix.IN_MODIFY|
							unix.IN_DELETE|
							unix.IN_CLOSE_WRITE|
							unix.IN_MOVED_TO|
							unix.IN_MOVED_FROM|
							unix.IN_MOVE_SELF|
							unix.IN_DELETE_SELF)
					if watchDescErr != nil {
						fmt.Println("Add watch error", watchDescErr)
						// event.Len is a name size
						offset += int(unix.SizeofInotifyEvent + event.Len)
						continue
					}

					addFile(ft, wd, filePath, int(event.Wd))
				}
			/*
				delete of nested file / dir
			*/
			case event.Mask&unix.IN_MOVED_FROM == unix.IN_MOVED_FROM ||
				event.Mask&unix.IN_DELETE == unix.IN_DELETE:
				{
					eventCounter += 1

					parentWd := int(event.Wd)
					currDir, _ := ft.GetPathByWd(parentWd)
					deletedPath := path.Join(currDir, fileName)

					deletedWd, ok := ft.GetWdByPath(deletedPath)
					if !ok {
						// event.Len is a name size
						offset += int(unix.SizeofInotifyEvent + event.Len)
						continue
					}

					unwatchDir(fd, deletedWd, ft)
					removeChildDir(ft, deletedWd, parentWd)
				}

			case event.Mask&unix.IN_MODIFY == unix.IN_MODIFY:
				{
					eventCounter += 1
				}

			}

			// event.Len is a name size
			offset += int(unix.SizeofInotifyEvent + event.Len)
		}

		if eventCounter > 0 {
			ch <- 0x1
		}
	}

}

func watchDir(fd int, parentWd int, filePath string, ft *fileTracker) {
	if ignoreList[filePath] {
		fmt.Printf("\"%s\" is ignored\n", filePath)
		return
	}

	wd, err := unix.InotifyAddWatch(fd, filePath,
		unix.IN_CREATE|
			unix.IN_MODIFY|
			unix.IN_DELETE|
			unix.IN_CLOSE_WRITE|
			unix.IN_MOVED_TO|
			unix.IN_MOVED_FROM|
			unix.IN_MOVE_SELF|
			unix.IN_DELETE_SELF,
	)
	if err != nil {
		log.Fatalf("Add watch error: %s", err)
	}

	addFile(ft, wd, filePath, parentWd)

	entries, err := os.ReadDir(filePath)
	if err != nil {
		fmt.Println("Read dir error", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			watchDir(fd, wd, path.Join(filePath, e.Name()), ft)
		}
	}
}

func unwatchDir(fd int, wd int, ft *fileTracker) {
	for _, cWd := range ft.GetWdChildren(wd) {
		unwatchDir(fd, cWd, ft)
	}

	_, err := unix.InotifyRmWatch(fd, uint32(wd))
	if err != nil {
		fmt.Printf("Remove watch error: %s\n", err)
	}

	removeDir(ft, wd)
}


