package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func checkErr(err error) error{
	if err == nil {
		return nil
	}
	log("its me!")
	log(err.Error())
	return fmt.Errorf("")
}

func log(msg interface{}) {
	if msg != "" {
		fmt.Println(msg)
	}
}

func getSeed() int64 {
	seed := time.Now().UTC().UnixNano()
	resp, err := http.Get("http://www.random.org/integers/?num=2&min=-999999999&max=999999999&col=1&base=10&format=plain&rnd=new")
	numBytes, err0 := ioutil.ReadAll(resp.Body)
	numStrings := strings.Split(string(numBytes), "\n")
	num1, err1 := strconv.ParseInt(numStrings[0], 10, 64)
	num2, err2 := strconv.ParseInt(numStrings[1], 10, 64)
	if err == nil && err0 == nil && err1 == nil && err2 == nil {
		log("adding numbers from random.org")
		seed += num1 + (1000000000 * num2)
	}
	return seed
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

func splitIntoFiles(file io.Reader, keys []*os.File) error {
	keyWriters := make([]io.Writer, len(keys))
	for i, key := range keys {
		keyWriters[i] = key
	}
	return split(file, keyWriters)
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

func joinFromFiles(file io.Writer, keys []*os.File) error {
	keyWriters := make([]io.Reader, len(keys))
	for i, key := range keys {
		keyWriters[i] = key
	}
	return join(file, keyWriters)
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

func runCommandLine() {
	splitMode := flag.NewFlagSet("split", flag.ExitOnError)
	splitKeyCount := splitMode.Int("k", 2, "the number of summons file will be split to")

	joinMode := flag.NewFlagSet("join", flag.ExitOnError)
	joinConfig := joinMode.String("config", "", "configuration file with the list of keys, optional")
	//joinOutput := joinMode.String("file", "", "output file name")

	switch os.Args[1] {
	case "split":
		rand.Seed(getSeed())

		splitMode.Parse(os.Args[2:])
		splitTail := splitMode.Args()
		if len(splitTail) == 0 {
			log("error: no specification given")
			os.Exit(1)
		}
		splitFileName, splitTail := splitTail[0], splitTail[1:]

        if splitFileName == "" {
        	log("error: -file not given")
        	os.Exit(1)
		}

		file, err := os.Open(splitFileName)
		if err != nil {
			log("error while opening input file: " + err.Error())
			os.Exit(1)
		}
		keyFiles := make([]*os.File, *splitKeyCount)

		if len(splitTail) > 0 {
			if len(splitTail) < *splitKeyCount {
				log("error: provided less key files than keys stated")
				os.Exit(1)
			}
			for i := 0; i < *splitKeyCount; i++ {
				keyFiles[i], err = os.Create(splitTail[i])
				if err != nil {
					log("error while opening output file: " + err.Error())
					os.Exit(1)
				}
			}
		} else {
			for i := 0; i < *splitKeyCount; i++ {
				keyFiles[i], err = os.Create(fmt.Sprintf("%s.key%d", splitFileName, i))
				if err != nil {
					log("error while opening output file: " + err.Error())
					os.Exit(1)
				}
			}
		}

		defer func() {
			file.Close()
			for _, tempFile := range keyFiles {
				tempFile.Close()
			}
		}()

		err = splitIntoFiles(file, keyFiles)
		if err != nil {
			log("error while splitting: " + err.Error())
		}

	case "join":
		joinMode.Parse(os.Args[2:])
		joinTail := joinMode.Args()
		if *joinConfig != "" {
			file, keyFiles, err := openViaInfo(*joinConfig)
			if err != nil {
				log("error while opening via config: " + err.Error())
				os.Exit(1)
			}
			err = joinFromFiles(file, keyFiles)
			if err != nil {
				log("error while joining: " + err.Error())
			}
		} else {
			if len(joinTail) == 0 {
				log("error: no files specified")
				os.Exit(1)
			}
			joinOutput, joinTail := joinTail[0], joinTail[1:]
			if joinOutput == "" {
				log("no output file given")
				os.Exit(1)
			}
			file, err := os.Create(joinOutput)
			if err != nil {
				log("error while opening output: " + err.Error())
			    os.Exit(1)
			}
			keyFiles := make([]*os.File, len(joinTail))
			for i, keyName := range joinTail {
				keyFiles[i], err = os.Open(keyName)
				if err != nil {
					log("error while opening key: " + err.Error())
				}
			}

			defer func() {
				file.Close()
				for _, key := range keyFiles {
					key.Close()
				}
			}()

			err = joinFromFiles(file, keyFiles)
			if err != nil {
				log("error while joining: " + err.Error())
			}
		}
	}
}

func main() {
	runCommandLine()
}

func _runCommandLine() {
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
