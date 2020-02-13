package file

import (
	"github.com/monochromegane/go-gitignore"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type File struct {
	Location string
	Filename string
}

type FileWalker struct {
	walkMutex              sync.Mutex
	isWalking              bool
	terminateWalking       bool
	directory              string
	fileListQueue          chan *File
	AllowListExtensions    []string
	LocationExcludePattern []string
	PathDenylist           []string
}

func NewFileWalker(directory string, fileListQueue chan *File) FileWalker {
	return FileWalker{
		walkMutex:              sync.Mutex{},
		fileListQueue:          fileListQueue,
		directory:              directory,
		terminateWalking:       false,
		isWalking:              false,
		AllowListExtensions:    []string{}, // What extensions are allowed
		LocationExcludePattern: []string{}, //
		PathDenylist:           []string{},
	}
}

func (f *FileWalker) Terminate() {
	f.walkMutex.Lock()
	defer f.walkMutex.Unlock()
	f.terminateWalking = true
}

func (f *FileWalker) Walking() bool {
	f.walkMutex.Lock()
	defer f.walkMutex.Unlock()
	return f.isWalking
}

// Starts walking the supplied directory with the supplied settings
func (f *FileWalker) WalkDirectory() error {
	f.walkMutex.Lock()
	f.isWalking = true
	f.walkMutex.Unlock()

	err := f.walkDirectoryRecursive(f.directory, []gitignore.IgnoreMatcher{})
	close(f.fileListQueue)

	f.walkMutex.Lock()
	f.terminateWalking = false
	f.isWalking = false
	f.walkMutex.Unlock()

	return err
}

func (f *FileWalker) walkDirectoryRecursive(directory string, ignores []gitignore.IgnoreMatcher) error {
	// Because this can work in a interactive mode we need a way to be able
	// to stop walking such as when the user starts a new search which this return should
	// take care of
	f.walkMutex.Lock()
	if f.terminateWalking == true {
		f.walkMutex.Unlock()
		return nil
	}
	f.walkMutex.Unlock()

	fileInfos, err := ioutil.ReadDir(directory)

	if err != nil {
		return err
	}

	files := []os.FileInfo{}
	dirs := []os.FileInfo{}

	// We want to break apart the files and directories from the
	// return as we loop over them differently and this avoids some
	// nested if logic at the expense of a "redundant" loop
	for _, file := range fileInfos {
		if file.IsDir() {
			dirs = append(dirs, file)
		} else {
			files = append(files, file)
		}
	}

	// Pull out all of the ignore and gitignore files and add them
	// to out collection of ignores to be applied for this pass
	// and later on
	for _, file := range files {
		if file.Name() == ".gitignore" || file.Name() == ".ignore" {
			ignore, err := gitignore.NewGitIgnore(filepath.Join(directory, file.Name()))
			if err == nil {
				ignores = append(ignores, ignore)
			}
		}
	}

	for _, file := range files {
		shouldIgnore := false
		for _, ignore := range ignores {
			if ignore.Match(filepath.Join(directory, file.Name()), file.IsDir()) {
				shouldIgnore = true
			}
		}

		if !shouldIgnore {
			for _, p := range f.LocationExcludePattern {
				if strings.Contains(filepath.Join(directory, file.Name()), p) {
					shouldIgnore = true
				}
			}

			if !shouldIgnore {
				f.fileListQueue <- &File{
					Location: filepath.Join(directory, file.Name()),
					Filename: file.Name(),
				}
			}
		}
	}

	// Now we process the directories
	for _, dir := range dirs {
		shouldIgnore := false
		for _, ignore := range ignores {
			if ignore.Match(filepath.Join(directory, dir.Name()), dir.IsDir()) {
				shouldIgnore = true
			}
		}

		for _, deny := range f.PathDenylist {
			if strings.HasSuffix(dir.Name(), deny) {
				shouldIgnore = true
			}
		}

		if !shouldIgnore {
			err = f.walkDirectoryRecursive(filepath.Join(directory, dir.Name()), ignores)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Walk the directory backwards looking for .git or .hg
// directories indicating we should start our search from that
// location as its the root
func FindRepositoryRoot(startDirectory string) string {
	// Firstly try to determine our real location
	curdir, err := os.Getwd()
	if err != nil {
		return startDirectory
	}

	// Check if we have .git or .hg where we are and if
	// so just return because we are already there
	if checkForGitOrMercurial(curdir) {
		return startDirectory
	}

	// We did not find something, so now we need to walk the file tree
	// backwards in a cross platform way and if we find
	// a match we return that
	lastIndex := strings.LastIndex(curdir, string(os.PathSeparator))
	for lastIndex != -1 {
		curdir = curdir[:lastIndex]

		if checkForGitOrMercurial(curdir) {
			return curdir
		}

		lastIndex = strings.LastIndex(curdir, string(os.PathSeparator))
	}

	// If we didn't find a good match return the supplied directory
	// so that we start the search from where we started at least
	// rather than the root
	return startDirectory
}

// Check if there is a .git or .hg folder in the supplied directory
func checkForGitOrMercurial(curdir string) bool {
	if stat, err := os.Stat(filepath.Join(curdir, ".git")); err == nil && stat.IsDir() {
		return true
	}

	if stat, err := os.Stat(filepath.Join(curdir, ".hg")); err == nil && stat.IsDir() {
		return true
	}

	return false
}

// A custom version of extracting extensions for a file
// which also has a case insensitive cache in order to save
// some needless processing
func GetExtension(name string) string {
	name = strings.ToLower(name)
	ext := filepath.Ext(name)

	if ext == "" || strings.LastIndex(name, ".") == 0 {
		ext = name
	} else {
		// Handling multiple dots or multiple extensions only needs to delete the last extension
		// and then call filepath.Ext.
		// If there are multiple extensions, it is the value of subExt,
		// otherwise subExt is an empty string.
		subExt := filepath.Ext(strings.TrimSuffix(name, ext))
		ext = strings.TrimPrefix(subExt+ext, ".")
	}

	return ext
}
