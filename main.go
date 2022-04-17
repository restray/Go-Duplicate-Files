package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type File struct {
	name string
	path string
	info os.FileInfo
}

type UniqueFiles struct {
	path     []string
	files    map[string][]File
	rawFiles []File
}

func getFilesFromArgs(args []string) []string {
	paths := []string{}

	if len(args) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		paths = []string{cwd}
		fmt.Printf("No path specified, using current directory: %s\n", cwd)
	} else {
		path := args
		for _, path := range path {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				fmt.Printf("Path %s does not exist\n", path)
				panic(err)
			}
		}

		paths = path
	}

	return paths
}

func getFilesInDirectory(path string) []File {
	var files []File
	err := filepath.WalkDir(path, func(pathWalk string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				fmt.Println(err)
				panic(err)
			}
			files = append(files, File{filepath.Base(pathWalk), pathWalk, info})
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	return files
}

func main() {
	// Define the global struct
	prog := UniqueFiles{}

	// Load the paths from the command line or set this up from the current directory
	prog.path = getFilesFromArgs(os.Args[1:])
	if len(prog.path) == 0 {
		fmt.Println("No specified path")
		panic("No specified path")
	}

	// Get the files in the directory
	for _, path := range prog.path {
		prog.rawFiles = append(prog.rawFiles, getFilesInDirectory(path)...)
	}

	fmt.Println("Duplicated files:")

	// Unique entry in list
	files := make(map[string][]File)

	// Get duplicate filenames
	for _, file := range prog.rawFiles {
		for _, file2 := range prog.rawFiles {
			if file.name == file2.name && file.path != file2.path {
				if _, exists := files[file.name]; !exists {
					files[file.name] = []File{file}
				} else {
					files[file.name] = append(files[file.name], file)
				}
			}
		}
	}

	prog.files = make(map[string][]File)

	for filename, dupFiles := range files {
		keys := make(map[File]bool)

		for _, file := range dupFiles {
			if _, value := keys[file]; !value {
				keys[file] = true
				prog.files[filename] = append(prog.files[filename], file)
			}
		}

		sort.Slice(prog.files[filename], func(i, j int) bool {
			return !prog.files[filename][i].info.ModTime().Before(prog.files[filename][j].info.ModTime())
		})
	}

	for _, files := range prog.files {
		if len(files) > 1 {
			for _, file := range files {
				fmt.Printf("%s (%db) - Modified on: %s\n", file.path, file.info.Size(), file.info.ModTime())
			}
			fmt.Println()
		}
	}
}
