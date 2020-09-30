package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
)

func checkErr(err error) error{
	if err == nil {
		return nil
	}
	log("its me!")
	log(err.Error())
	return fmt.Errorf("")
}

func log(msg string) {
	if msg != "" {
		fmt.Println(msg)
	}
}

func split(file io.Reader, keys []io.Writer) error {
	data, err := ioutil.ReadAll(file)
	if err = checkErr(err); err != nil {
		return err
	}
	l := len(keys)
	if l < 2 {
		return fmt.Errorf("less than 2 key files provided")
	}

	rands := make([]byte, l-1)

	for _, b := range data {
		sum := byte(0)
		for i := 0; i < l-1; i++ {
			r := byte(rand.Intn(256))
			rands[i] = r
			sum += r
		}

		keys[0].Write([]byte{b-sum})
        for i, key := range keys[1:] {
        	key.Write([]byte{rands[i]})
		}
	}
	return nil
}

func join(file io.Writer, keys []io.Reader) error {
	l := len(keys)
	if l < 2 {
		return fmt.Errorf("less than 2 key files provided")
	}
    contents := make([][]byte, l)
    var err error

    for i, key := range keys {
    	contents[i], err = ioutil.ReadAll(key)
    	if err = checkErr(err); err != nil {
    		return err
		}
	}

    maxlength := len(contents[0])
    for _, d := range contents {
    	if maxlength < len(d) {
    		maxlength = len(d)
		}
	}

	for i := 0; i < maxlength; i++ {
        sum := byte(0)
        for j := 0; j < l; j++ {
        	if len(contents[j]) > i {
        		sum += contents[j][i]
			}
		}
		_, err := file.Write([]byte{sum})
		if err = checkErr(err); err != nil {
			return err
		}
	}
	return nil
}

func openViaInfo(infFileName string) (*os.File, []*os.File, error) {
	infoBytes, err := ioutil.ReadFile(infFileName)
	if err = checkErr(err); err != nil {
		return nil, nil, err
	}
	info := string(infoBytes)
	lines := strings.Split(info, "\n")
	fileName := lines[0]
	keyFileNames := lines[1:]

	file, err := os.Create(fileName)
	if err = checkErr(err); err != nil {
		return nil, nil, err
	}

	keyFiles := make([]*os.File, 0)
	for _, keyName := range keyFileNames {
		if keyName == "" {
			continue
		}
		keyFile, err := os.Open(keyName)
		if err = checkErr(err); err != nil {
			return nil, nil, err
		}
		keyFiles = append(keyFiles, keyFile)
	}

	return file, keyFiles, nil
}

func main() {
	restoreMode := flag.Bool("restore", false, "")
	infoFile := flag.String("info", "", "info file name")
	splitNum := flag.Int("n", 2, "number of files to split to")
	splitName := flag.String("split", "", "name of the file to split")
	keepFiles := flag.Bool("keep", false, "if specified, will not remove files")

	flag.Parse()

	if *restoreMode  {
		if *infoFile != "" {
			file, keyFileRefs, _ := openViaInfo(*infoFile)
			defer func() {
				file.Close()
				if !*keepFiles {
					os.Remove(*infoFile)
				}
				for _, file := range keyFileRefs {
					file.Close()
					if !*keepFiles {
						os.Remove(file.Name())
					}
				}
			}()
			keyFiles := make([]io.Reader, len(keyFileRefs))
			for i, key := range keyFileRefs {
				keyFiles[i] = key
			}
			join(file, keyFiles)


		} else {
			log("cannot restore using given information")
			os.Exit(1)
		}
	} else {
        if *splitNum < 2 {
        	log("split number should be greater than 2")
        	os.Exit(1)
		}
        if *splitName == "" {
        	log("no split file name given")
        	os.Exit(1)
		}
		infoFile, err := os.Create(fmt.Sprintf("%s.info", *splitName))
		defer infoFile.Close()
		if err = checkErr(err); err != nil {
			os.Exit(1)
		}
		file, err := os.Open(*splitName)
		if err = checkErr(err); err != nil {
			os.Exit(1)
		}
		infoFile.Write([]byte(*splitName + "\n"))

		keys := make([]*os.File, *splitNum)
		for i := 0; i < *splitNum; i++ {
			keyFileName := fmt.Sprintf("%s.key%d", *splitName, i)
			keyFile, err := os.Create(keyFileName)
			if err = checkErr(err); err != nil {
				os.Exit(1)
			}
			infoFile.Write([]byte(keyFileName + "\n"))
			keys[i] = keyFile
		}
		defer func() {
			infoFile.Close()
			file.Close()
			if !*keepFiles {
				os.Remove(file.Name())
			}
			for _, key := range keys {
				key.Close()
			}
		}()

		keysWriters := make([]io.Writer, len(keys))
		for i, key := range keys {
			keysWriters[i] = key
		}
		err = split(file, keysWriters)
		if err = checkErr(err); err != nil {
			os.Exit(1)
		}
	}
}

func testSplit() {
	file, _ := os.Open("code.txt")
	key1, _ := os.Create("code.key1")
	key2, _ := os.Create("code.key2")
	defer file.Close()
	defer key1.Close()
	defer key2.Close()
	split(file, []io.Writer{key1, key2})
}

func testJoin() {
	file, _ := os.Create("code.txt")
	key1, _ := os.Open("code.key1")
	key2, _ := os.Open("code.key2")
	defer file.Close()
	defer key1.Close()
	defer key2.Close()
	join(file, []io.Reader{key1, key2})
}
