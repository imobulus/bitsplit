package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/imobulus/bitsplit"
	"github.com/imobulus/bitsplit/osutil"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
)

const (
	LockFileName = ".lock"
	CodeSuccess = 0
	CodeLockDirNotExist = 1
	CodeKeyDirNotExist = 2
	CodeCantReadKeyFile = 3
)

var (
	errLog   = log.New(os.Stderr, "error: ", 0)
	stdLog = log.New(os.Stdout, "", 0)
)

func errorFatal(message string, err error) {
	if err != nil {
		if message != "" {
			errLog.Println(message)
		}
		errLog.Println(err)
		os.Exit(1)
	}
}

// returns a non-zero code and error if some directory not exists, otherwise kills the program. Fix in future
func Lock(lockDir, keyDir string) (int, error) {
	if !osutil.DirExists(lockDir) {
		return CodeLockDirNotExist, fmt.Errorf("can't find directory %s", lockDir)
		}
	if !osutil.DirExists(keyDir) {
		return CodeKeyDirNotExist, fmt.Errorf("can't find directory %s", keyDir)
	}

	lockDir, _ = filepath.Abs(lockDir)
	if lockDir == os.TempDir() {
		errLog.Fatal("can't lock tmp directory")
	}

	err := os.Chdir(lockDir)
	errorFatal("can't set working directory for some reason", err)

	if osutil.FileExists(LockFileName) &&
		!osutil.Promptf(
			"lock file %s already exists, the directory could already be locked. Do you want to lock it again?",
			LockFileName) {
		os.Exit(0)
	}

	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	errorFatal("can't get directory of executable for some reason", err)
	if lockDir == execDir && !osutil.Prompt(
		"this action will encrypt the program binary file. You will need to get it again to decrypt. Proceed?"){
		os.Exit(0)
	}

	tempDir, err := ioutil.TempDir("", "~temp")
	errorFatal("can't create temporary directory", err)

	// copying entire directory into temporary dir to avoid partial encrypting
    err = osutil.CopyDir(".", tempDir)

	//if there's an error while copying, abort entire process
	if err != nil {
		err1 := os.RemoveAll(tempDir)
		errorFatal(
			fmt.Sprintf(
				"can't remove temporary directory %s while aborting. Please remove manually", tempDir), err1)
		errLog.Fatal("copying unsuccessful\n" + err.Error())
	}

	//if there are further errors we call abortIfError() to undo changes
	abortIfError := func(errMaster error) {
		if errMaster == nil {
			return
		}
		errLog.Println(err, "\naborting...")

		_ = os.Chdir("..")
		err = os.RemoveAll(lockDir)
		if err != nil {
			errLog.Fatalf("abort removing current dir unsuccessful. All files are stored in %s\n%e", tempDir, err)
		}

		err = osutil.CopyDir(tempDir, lockDir)
		if err != nil {
			errLog.Fatalf("abort copying unsuccessful. All files are stored in %s\n%e", tempDir, err)
		}

		err = os.RemoveAll(tempDir)
		if err != nil {
			errLog.Fatalf("abort removing temp dir unsuccessful. Remove %s manually.\n%e", tempDir, err)
		}
		os.Exit(1)
	}

    h := sha1.New()
    key := make([]byte, 32)
    rand.Seed(bitsplit.GetSeed())
    rand.Read(key)
    h.Write(key)

    //encrypting
	err = filepath.Walk(".", func (path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't open %s", path), Contents: err}
		}

		fileContents, err := ioutil.ReadAll(file)
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't read %s", path), Contents: err}
		}
		err = file.Close()
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't close %s", path), Contents: err}
		}
		h.Write(fileContents)

		var buf bytes.Buffer
		err = bitsplit.AesGCMEncrypt(bytes.NewReader(fileContents), &buf, key)
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't encrypt %s", path), Contents: err}
		}

		err = ioutil.WriteFile(path, buf.Bytes(), 0644)
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't write to %s", path), Contents: err}
		}

		return nil
	})
	abortIfError(err)

	hash := hex.EncodeToString(h.Sum(nil))
	abortIfError( ioutil.WriteFile(filepath.Join(keyDir, hash), key, 0644) )

	abortIfError( osutil.HideFile(filepath.Join(keyDir, hash)) )

	abortIfError( ioutil.WriteFile(LockFileName, []byte(hash), 0644) )
	_ = osutil.HideFile(LockFileName)

	err = os.RemoveAll(tempDir)
	if err != nil {
		errLog.Printf("cant remove temporary directory %s. Please, remove manually\n%e", tempDir, err)
	}
	return CodeSuccess, nil
}

// returns a non-zero code and error if some directory not exists, otherwise kills the program. Fix in future
func Unlock(unlockDir, keyDir string) (int, error) {
	if !osutil.DirExists(unlockDir) {
		return CodeLockDirNotExist, fmt.Errorf("can't find directory %s", unlockDir)
	}
	if !osutil.DirExists(keyDir) {
		return CodeKeyDirNotExist, fmt.Errorf("can't find directory %s", keyDir)
	}
	if tmp, _ := filepath.Abs(unlockDir); tmp == os.TempDir() {
		errLog.Fatal("can't unlock tmp directory")
	}

	unlockDir, _ = filepath.Abs(unlockDir)
	err := os.Chdir(unlockDir)
	errorFatal("can't set working directory for some reason", err)

	if !osutil.FileExists(LockFileName){
		errLog.Fatalf("lock file %s does not exist", LockFileName)
	}
	keyFileBytes, err := ioutil.ReadFile(LockFileName)
	if err != nil {
		errLog.Fatalf("can't read lock file %s", LockFileName)
	}
	keyFileName := string(keyFileBytes)

	key, err := ioutil.ReadFile(filepath.Join(keyDir, keyFileName))
	if err != nil {
		return CodeCantReadKeyFile, fmt.Errorf("can't read key file %s", filepath.Join(keyDir, keyFileName))
	}

	//create temporary dir
	tempDir, err := ioutil.TempDir("", "~temp")
	errorFatal("can't create temporary directory", err)

	//copy the entire directory into temporary dir to avoid partial encrypting
	err = osutil.CopyDir(".", tempDir)

	//if there's an error while copying, abort entire process
	if err != nil {
		err1 := os.RemoveAll(tempDir)
		errorFatal(
			fmt.Sprintf(
				"couldn't remove temporary directory %s while aborting. Please remove manually", tempDir), err1)
		errLog.Fatal("copying unsuccessful\n" + err.Error())
	}

	//if there are further errors we call abortIfError() to undo changes
	abortIfError := func(errMaster error) {
		if errMaster == nil {
			return
		}
		errLog.Println(err, "\naborting...")

		_ = os.Chdir("..")
		err = os.RemoveAll(unlockDir)
		if err != nil {
			errLog.Fatalf("abort removing current dir unsuccessful. All files are stored in %s\n%e", tempDir, err)
		}

		err = osutil.CopyDir(tempDir, unlockDir)
		if err != nil {
			errLog.Fatalf("abort copying unsuccessful. All files are stored in %s\n%e", tempDir, err)
		}

		err = os.RemoveAll(tempDir)
		if err != nil {
			errLog.Fatalf("abort removing temp dir unsuccessful. Remove %s manually.\n%e", tempDir, err)
		}
		os.Exit(1)
	}

	//remove the lock file since it is not encrypted
	abortIfError( os.Remove(LockFileName) )

	//decrypting
	err = filepath.Walk(".", func (path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		var buf bytes.Buffer
		file, err := os.Open(path)
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't open %s", path), Contents: err}
		}

		err = bitsplit.AesGCMDecrypt(file, &buf, key)
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't decrypt %s", path), Contents: err}
		}
		err = file.Close()
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't close %s", path), Contents: err}
		}

		err = ioutil.WriteFile(path, buf.Bytes(), 0644)
		if err != nil {
			return bitsplit.OSError{Details: fmt.Sprintf("can't write to %s", path), Contents: err}
		}

		return nil
	})
	abortIfError(err)

	err = os.Remove(filepath.Join(keyDir, keyFileName))
	if err != nil {
		errLog.Printf("removing key file %s unsuccessful. Please remove manually", filepath.Join(keyDir, keyFileName))
	}

	err = os.RemoveAll(tempDir)
	if err != nil {
		errLog.Printf("cant remove temporary directory %s. Please, remove manually\n%e", tempDir, err)
	}
	return CodeSuccess, nil
}

func runCommandLine() {
	if len(os.Args) < 2 {
		stdLog.Println("this is directory locker")
		return
	}

	lock := flag.NewFlagSet("lock", flag.ExitOnError)
	lockKeyDir := lock.String("keydir", "", "specify key directory")
	lockDir := lock.String("dir", ".", "specify lock directory")

	switch os.Args[1] {
	case "lock":
		err := lock.Parse(os.Args[2:])
		errorFatal("can't parse flags", err)
		if !osutil.IsFlagPassedInSet(lock,"keydir") {
			drives := osutil.GetDrives()
			if len(drives) == 0 {
				errLog.Fatal("specify your key directory using -keydir")
			}
			if !osutil.Promptf("use disk %s as key storage?", drives[len(drives) - 1]) {
				os.Exit(1)
			}
			*lockKeyDir = drives[len(drives) - 1]
		}

		_, err = Lock(*lockDir, *lockKeyDir)
		errorFatal("", err)
	case "unlock":
		err := lock.Parse(os.Args[2:])
		errorFatal("can't parse flags", err)
		if !osutil.IsFlagPassedInSet(lock,"keydir") {
			for _, d := range osutil.GetDrives() {
				code, err := Unlock(*lockDir, d)
				switch code {
				case CodeSuccess:
					os.Exit(0)
				case CodeCantReadKeyFile:
					continue
				default:
					errLog.Fatal(err)
				}
			}
			errLog.Fatal("can't find external key. Specify key directory using -keydir")
		} else {
			_, err := Unlock(*lockDir, *lockKeyDir)
			errorFatal("", err)
		}
	}
}

func main() {
	runCommandLine()
}
