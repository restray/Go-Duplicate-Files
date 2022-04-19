package main

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/briandowns/spinner"
	"github.com/cheggaaa/pb"
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

var NoVerboseMode bool = false

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
		if os.IsPermission(err) {
			return nil
		}
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

func getFileHash(path string) string {
	hasher := sha256.New()
	f, err := os.Open(path)

	if os.IsPermission(err) {
		return ""
	}

	if os.IsPermission(err) {
		fmt.Println(err)
		panic(err)
	}

	defer f.Close()
	if _, err := io.Copy(hasher, f); err != nil {
		fmt.Println(err)
		panic(err)
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func loadPaths(paths []string) []File {
	rawFiles := []File{}

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

	if !NoVerboseMode {
		s.Start()
		defer s.Stop()
	}

	// Get the files in the directory
	for _, path := range paths {
		s.Prefix = "Loading files from " + path + " "
		rawFiles = append(rawFiles, getFilesInDirectory(path)...)
	}

	return rawFiles
}

type ByPath struct {
	files    []File
	filename string
}

func testDuplicateFilenames(done chan ByPath, fileToCheck []File, files []File) {
	for _, file := range fileToCheck {
		information := ByPath{nil, file.name}
		information.files = []File{}
		for _, file2 := range files {
			if file.name == file2.name && file.path != file2.path {
				information.files = append(information.files, file)
			}
		}
		done <- information
	}
}

func main() {
	noVerb := flag.Bool("no-verbose", false, "Don't display verbose output and colors")
	strict := flag.Bool("strict", false, "Only display strict equals files")
	outputFilename := flag.String("output", "", "Only display strict equals files")
	flag.Parse()

	NoVerboseMode = *noVerb

	if !NoVerboseMode {
		color.Warn.Tips("[!] Verbose mode enabled, to disable it specify add the option: -no-verbose")
	} else {
		color.Disable()
	}

	if *strict && !NoVerboseMode {
		color.Warn.Tips("[!] Strict mode enabled, to disable it specify remove the option: -strict")
	}

	f := os.Stdout

	if *outputFilename == "" {
		color.Warn.Tips("[!] No output file specified, printing to console. Specify -output filename to output to a file")
	} else {
		var err error
		// If the file doesn't exist, create it, or append to the file
		f, err = os.OpenFile(*outputFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			panic(err)
		}

		defer f.Close()
	}

	w := bufio.NewWriter(f)

	// Define the global struct
	prog := UniqueFiles{}

	// Load the paths from the command line or set this up from the current directory
	prog.path = getFilesFromArgs(flag.Args(), !*strict)
	if len(prog.path) == 0 {
		fmt.Println("No specified path")
		panic("No specified path")
	}

	prog.rawFiles = loadPaths(prog.path)

	if !NoVerboseMode {
		fmt.Println("Duplicated files:")
	}

	// Unique entry in list
	files := make(map[string][]File)

	bar := pb.StartNew(len(prog.rawFiles))

	done := make(chan ByPath)
	// Get duplicate filenames
	maxThreads := 50
	if len(prog.rawFiles) < maxThreads {
		maxThreads = len(prog.rawFiles)
	}
	for i := 0; i < maxThreads; i++ {
		min := i * (len(prog.rawFiles) / maxThreads)
		max := i*(len(prog.rawFiles)/maxThreads) + (len(prog.rawFiles) / maxThreads)
		if i == maxThreads {
			go testDuplicateFilenames(done, prog.rawFiles[min:], prog.rawFiles)
		} else {
			go testDuplicateFilenames(done, prog.rawFiles[i:max], prog.rawFiles)
		}
	}

	for i := 0; i < len(prog.rawFiles); i++ {
		bar.Increment()

		information := <-done
		if _, exists := files[information.filename]; !exists {
			files[information.filename] = information.files
		} else {
			files[information.filename] = append(files[information.filename], information.files...)
		}
	}
	bar.Finish()

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
					prog.files[filename][i].hash = getFileHash(file.path)
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

	if *outputFilename != "" {
		color.Disable()
	}

	for filename, files := range prog.files {
		lenFile := len(files)
		if lenFile > 1 {
			totalDupFile++
			totalFiles += lenFile

			if !NoVerboseMode {
				fmt.Fprintln(w, color.Bold.Sprint(filename, ":"))
			}

			for index, file := range files {
				// Calc color (from green to red)
				incrSize := uint8(float64(index) / float64(lenFile-1) * 255)
				s := color.RGB(incrSize, 255-incrSize, 0)

				fmt.Fprintf(w, s.Sprint(file.path))

				if !NoVerboseMode {
					fmt.Fprintf(w, s.Sprintf(" (%db) - Modified on: %s", file.info.Size(), file.info.ModTime()))
					if *strict {
						fmt.Fprintf(w, s.Sprintf(" - Hash: %s", file.hash))
					}
				}
				fmt.Fprintln(w, s.Sprint())
			}
			fmt.Fprintln(w, fmt.Sprint())
		}
	}

	if *outputFilename != "" {
		color.ResetOptions()
	}

	if !NoVerboseMode {
		color.Bold.Printf("Total files: %d (with %d uniques)\n", totalFiles, totalDupFile)
	}
}
