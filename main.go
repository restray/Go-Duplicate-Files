package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/gookit/color"
)

type File struct {
	name string
	path string
	info os.FileInfo
	hash string ""
}

type UniqueFiles struct {
	path     []string
	files    map[string][]File
	rawFiles []File
}

func getFilesFromArgs(args []string, showError bool) []string {
	paths := []string{}

	if len(args) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		paths = []string{cwd}
		if showError {
			color.Warn.Tips("No path specified, using current directory: %s", cwd)
		}
	} else {
		path := args
		for _, path := range path {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if showError {
					fmt.Printf("Path %s does not exist\n", path)
				}
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
			files = append(files, File{filepath.Base(pathWalk), pathWalk, info, ""})
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	return files
}

func remove(s []File, i int) []File {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func main() {
	noVerb := flag.Bool("no-verbose", false, "Don't display verbose output and colors")
	strict := flag.Bool("strict", false, "Only display strict equals files")
	flag.Parse()

	if !*noVerb {
		color.Warn.Tips("[!] Verbose mode enabled, to disable it specify add the option: -no-verbose")
	} else {
		color.Disable()
	}

	if *strict && !*noVerb {
		color.Warn.Tips("[!] Strict mode enabled, to disable it specify remove the option: -strict")
	}

	// Define the global struct
	prog := UniqueFiles{}

	// Load the paths from the command line or set this up from the current directory
	prog.path = getFilesFromArgs(flag.Args(), !*strict)
	if len(prog.path) == 0 {
		fmt.Println("No specified path")
		panic("No specified path")
	}

	// Get the files in the directory
	for _, path := range prog.path {
		prog.rawFiles = append(prog.rawFiles, getFilesInDirectory(path)...)
	}

	if !*noVerb {
		fmt.Println("Duplicated files:")
	}

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

	if *strict {
		for filename, files := range prog.files {
			if len(files) > 1 {
				for i, file := range files {
					hasher := sha256.New()
					f, err := os.Open(file.path)
					if err != nil {
						fmt.Println(err)
						panic(err)
					}
					defer f.Close()
					if _, err := io.Copy(hasher, f); err != nil {
						fmt.Println(err)
						panic(err)
					}
					prog.files[filename][i].hash = fmt.Sprintf("%x", hasher.Sum(nil))
				}

				duplicateFiles := []File{}
				for i, file := range files {
					for j, file2 := range files {
						if j > i && file.hash == file2.hash {
							if len(duplicateFiles) == 0 {
								duplicateFiles = append(duplicateFiles, file)
							}
							duplicateFiles = append(duplicateFiles, file2)
						}
					}
				}
				prog.files[filename] = duplicateFiles
			}
		}
	}

	totalFiles := 0
	totalDupFile := 0
	for filename, files := range prog.files {
		lenFile := len(files)
		if lenFile > 1 {
			totalDupFile++
			totalFiles += lenFile

			if !*noVerb {
				color.Bold.Println(filename, ":")
			}

			for index, file := range files {
				// Calc color (from green to red)
				incrSize := uint8(float64(index) / float64(lenFile-1) * 255)
				s := color.RGB(incrSize, 255-incrSize, 0)

				s.Print(file.path)

				if !*noVerb {
					s.Printf(" (%db) - Modified on: %s", file.info.Size(), file.info.ModTime())
					if *strict {
						s.Printf(" - Hash: %s", file.hash)
					}
				}
				s.Println()
			}
			fmt.Println()
		}
	}

	if !*noVerb {
		color.Bold.Printf("Total files: %d (with %d uniques)\n", totalFiles, totalDupFile)
	}
}
