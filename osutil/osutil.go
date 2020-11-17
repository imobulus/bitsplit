package osutil

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/imobulus/bitsplit"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func Prompt(action string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(action + " [y/n] ")
	text, _ := reader.ReadString('\n')
	text = text[:len(text) - 2]
	return (strings.ToLower(text) == "y") || (strings.ToLower(text) == "yes")
}

func Promptf(format string, a ...string) bool {
	return Prompt(fmt.Sprintf(format, a))
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DirExists(directory string) bool {
	info, err := os.Stat(directory)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func GetDrives() (r []string){
	//goland:noinspection SpellCheckingInspection
	switch runtime.GOOS {
	case "windows":
		for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ"{
			f, err := os.Open(string(drive)+":\\")
			if err == nil {
				r = append(r, string(drive)+":\\")
				f.Close()
			}
		}
	}
	return
}

func IsFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func IsFlagPassedInSet(set *flag.FlagSet, name string) bool {
	found := false
	set.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func HideFile(filename string) error {
	filenameW, err := syscall.UTF16PtrFromString(filename)
	if err != nil {
		return err
	}
	err = syscall.SetFileAttributes(filenameW, syscall.FILE_ATTRIBUTE_HIDDEN)
	if err != nil {
		return err
	}
	return nil
}

func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

func CopyDir(src, dst string) error {
	if !DirExists(src) {
		return fmt.Errorf("no such directory %s", src)
	}
	err := os.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return bitsplit.OSError{Details: fmt.Sprintf("can't make directory %s", dst), Contents: err}
	}

	err = filepath.Walk(src, func (path string, info os.FileInfo, err error) error {
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("while walking path %s", path), Contents: err}
		}
		if path == src {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return bitsplit.OSError{
				Details: "this shouldn't happen ever, but file path mismatch occurred", Contents: err}
		}

		if info.IsDir() {
			err := os.Mkdir(filepath.Join(dst, relPath), os.ModePerm)
			if err != nil {
				return bitsplit.OSError{Details: fmt.Sprintf("while creating path %s", relPath), Contents: err}
			}
		} else {
			fileData, err := ioutil.ReadFile(path)
			if err != nil {
				return bitsplit.OSError{Details: fmt.Sprintf("while reading file %s", path), Contents: err}
			}
			err = ioutil.WriteFile(filepath.Join(dst, relPath), fileData, 0644)
			if err != nil {
				return bitsplit.OSError{Details: fmt.Sprintf("while creating file %s", relPath), Contents: err}
			}
		}
		return nil
	})
	return err
}
