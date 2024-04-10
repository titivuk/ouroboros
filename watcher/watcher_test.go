package watcher

import (
	"reflect"
	"testing"
)

func TestAddFile(t *testing.T) {
	ft := &fileTracker{
		wd2Children: make(map[int][]int),
		wd2Path:     make(map[int]string),
		path2Wd:     make(map[string]int),
	}
	addFile(ft, 1, "a", -1)
	addFile(ft, 2, "a/b", 1)
	addFile(ft, 3, "a/c", 1)
	addFile(ft, 4, "a/b/d", 2)

	path2Wd := map[string]int{
		"a":     1,
		"a/b":   2,
		"a/c":   3,
		"a/b/d": 4,
	}

	wd2Path := map[int]string{
		1: "a",
		2: "a/b",
		3: "a/c",
		4: "a/b/d",
	}

	wd2Children := map[int][]int{
		-1: {1},
		1:  {2, 3},
		2:  {4},
	}

	if !reflect.DeepEqual(ft.path2Wd, path2Wd) {
		t.Fatalf("file path -> watch descriptor mapping is not correct")
	}
	if !reflect.DeepEqual(ft.wd2Path, wd2Path) {
		t.Fatalf("watch descriptor -> file path mapping is not correct")
	}
	if !reflect.DeepEqual(ft.wd2Children, wd2Children) {
		t.Fatalf("watch desriptor children hierarchy is not correct")
	}
}

func TestRemoveFile(t *testing.T) {
	tests := []struct {
		wdToRemove  int
		path2Wd     map[string]int
		wd2Path     map[int]string
		wd2Children map[int][]int
	}{
		{
			3,
			map[string]int{
				"a":     1,
				"a/b":   2,
				"a/b/d": 4,
			},
			map[int]string{
				1: "a",
				2: "a/b",
				4: "a/b/d",
			},
			map[int][]int{
				-1: {1},
				1:  {2, 3},
				2:  {4},
			},
		},
		{
			2,
			map[string]int{
				"a":     1,
				"a/b/d": 4,
			},
			map[int]string{
				1: "a",
				4: "a/b/d",
			},
			map[int][]int{
				-1: {1},
				1:  {2, 3},
			},
		},
	}

	ft := &fileTracker{
		wd2Children: make(map[int][]int),
		wd2Path:     make(map[int]string),
		path2Wd:     make(map[string]int),
	}
	addFile(ft, 1, "a", -1)
	addFile(ft, 2, "a/b", 1)
	addFile(ft, 3, "a/c", 1)
	addFile(ft, 4, "a/b/d", 2)

	for _, tt := range tests {
		removeDir(ft, tt.wdToRemove)

		if !reflect.DeepEqual(ft.path2Wd, tt.path2Wd) {
			t.Fatalf("file path -> watch descriptor mapping is not correct. Expected: %v. Actual: %v",
				tt.path2Wd, ft.path2Wd)
		}
		if !reflect.DeepEqual(ft.wd2Path, tt.wd2Path) {
			t.Fatalf("watch descriptor -> file path mapping is not correct. Expected: %v. Actual: %v",
				tt.wd2Path, ft.wd2Path)
		}
		if !reflect.DeepEqual(ft.wd2Children, tt.wd2Children) {
			t.Fatalf("watch desriptor children hierarchy is not correct. Expected: %v. Actual: %v",
				tt.wd2Children, ft.wd2Children)
		}
	}
}
