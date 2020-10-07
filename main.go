package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const VERSION = "v0.0.3"

func errorFatal(message string, err error) {
	if err != nil {
		log.Println(message)
		log.Println(err)
		os.Exit(1)
	}
}

func prompt(action string) bool {
	reader := bufio.NewReader(os.Stdin)
	log.Print(action + " [y/n] ")
	text, _ := reader.ReadString('\n')
	text = text[:len(text) - 2]
	return (strings.ToLower(text) == "y") || (strings.ToLower(text) == "yes")
}

func askForRewrite(fileName string) {
	if ! prompt(fmt.Sprintf("file %s already exists, do you want to overwrite it?", fileName)) {
		log.Fatal("error: file already exists")
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func getSeed() int64 {
	seed := time.Now().UTC().UnixNano()

	addRandOrg := func() {
		resp, err := http.Get("http://www.random.org/integers/?num=4&min=0&max=65535&col=1&base=10&format=plain&rnd=new")
		if err != nil {
			log.Println("warning: no response from random.org")
			log.Println(err)
			return
		}
		numBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("warning: reading response from random.org")
			log.Println(err)
			return
		}

		numStrings := strings.Split(string(numBytes), "\n")
		toAdd := int64(0)
		for _, ns := range numStrings[:4] {
			n, err := strconv.ParseInt(ns, 10, 64)
			if err != nil {
				log.Println("warning: cannot parse string to int")
				log.Println(err)
				return
			}
			toAdd *= 65536
			toAdd += n
		}
		seed += toAdd
		log.Println("using numbers from random.org")
	}
	addRandOrg()

	return seed
}

func split(file io.Reader, keys []io.Writer) error {
	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Println("while reading file contents")
		return err
	}

	l := len(keys)
	if l < 2 {
		return fmt.Errorf("less than 2 key files provided")
	}

	rands := make([]byte, l-1)

	for _, b := range data {
		sum := byte(0)
		rand.Read(rands)
		for _, k := range rands {
			sum += k
		}

		keys[0].Write([]byte{b - sum})
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
    	if err != nil {
    		log.Println("error reading key")
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
		if err != nil {
			log.Println("error writing result")
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
	if err != nil {
		log.Println("error reading config file")
		return nil, nil, err
	}
	info := string(infoBytes)
	lines := strings.Split(info, "\n")
	fileName := lines[0]
	keyFileNames := lines[1:]

	file, err := os.Create(fileName)
	if err != nil {
		log.Println("error creating output")
		return nil, nil, err
	}

	keyFiles := make([]*os.File, 0)
	for _, keyName := range keyFileNames {
		if keyName == "" {
			continue
		}
		keyFile, err := os.Open(keyName)
		if err != nil {
			log.Println("error opening keys")
			return nil, nil, err
		}
		keyFiles = append(keyFiles, keyFile)
	}

	return file, keyFiles, nil
}

func aesGCMEncrypt(file io.Reader, output io.Writer, key []byte) error {
	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		log.Println("while reading contents of encrypted file")
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Println("while creating cipher block")
		return err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		log.Println("while creating gcm encryptor")
		return err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	rand.Read(nonce)

	encryptedData := aesGCM.Seal(nonce, nonce, fileContents, nil)
	output.Write(encryptedData)
	return nil
}

func aesGCMDecrypt(file io.Reader, output io.Writer, key []byte) error {
    c, err := aes.NewCipher(key)
    if err != nil {
    	log.Println("error while creating decryption cipher")
    	return err
	}

	aesGCM, err := cipher.NewGCM(c)
	if err != nil {
		log.Println("error while creating decryption GCM")
		return err
	}

	nonceSize := aesGCM.NonceSize()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Println("error while reading the file")
		return err
	}

	if len(data) < nonceSize {
		return fmt.Errorf("encrypted file must be bigger than nonce size")
	}

	nonce, data := data[:nonceSize], data[nonceSize:]
	decrypted, err := aesGCM.Open(nil, nonce, data, nil)
	if err != nil {
		log.Println("error while decrypting")
		return err
	}

	output.Write(decrypted)
	return nil
}

func runCommandLine() {
	splitMode := flag.NewFlagSet("split", flag.ExitOnError)
	splitKeyCount := splitMode.Int("k", 2, "the number of summons file will be split to")
	splitForceRewrite := splitMode.Bool("f", false, "force rewriting key files")

	joinMode := flag.NewFlagSet("join", flag.ExitOnError)
	joinConfig := joinMode.String("config", "",
		                    "configuration file with the output file and list of keys, optional")

	aesEncMode := flag.NewFlagSet("encrypt-aes", flag.ExitOnError)
	aesEncKey := aesEncMode.String("key", "", "AES key in hex format")
	aesEncRewrite := aesEncMode.Bool("r", false, "use this flag to rewrite file with encrypted data")
	aesEncForce := aesEncMode.Bool("f", false, "use this flag to force rewriting")
	aesEncHex := aesEncMode.Bool("hex", false, "use this flag to save key in hex representation")

	aesDecMode := flag.NewFlagSet("decrypt-aes", flag.ExitOnError)
	aesDecKey := aesDecMode.String("key", "", "AES key in hex format")
	aesDecForce := aesDecMode.Bool("f", false, "use this flag to force rewriting")
	aesDecHex := aesDecMode.Bool("hex", false, "use this flag to load key, saved in hex representation")
	aesDecRewrite := aesDecMode.Bool("r", false, "use this flag to rewrite file with decrypted data")

	if len(os.Args) == 1 {
		log.Println(fmt.Sprintf("This is bitsplit %s. Visit github.com/imobulus/bitsplit or use --help", VERSION))
		os.Exit(0)
	}

	switch os.Args[1] {
	case "split":
		splitMode.Parse(os.Args[2:])
		splitTail := splitMode.Args()
		splitCountProvided := isFlagPassed("k")

		if len(splitTail) == 0 {
			log.Fatal("error: no specification given")
		}
		splitFileName, splitTail := splitTail[0], splitTail[1:]

        if splitFileName == "" {
        	log.Fatal("error: file not given")
		}

		file, err := os.Open(splitFileName)
		errorFatal("error while opening input file", err)

		var keyFiles []*os.File

		if len(splitTail) > 0 {
			if !splitCountProvided {
				*splitKeyCount = len(splitTail)
			}
			if len(splitTail) < *splitKeyCount {
				log.Fatal("error: provided less key files than keys stated")
			}

			if !*splitForceRewrite {
				for i := 0; i < *splitKeyCount; i++ {
					if fileExists(splitTail[i]) {
						log.Fatal(fmt.Sprintf("file %s already exists. Use -f to force rewriting", splitTail[i]))
					}
				}
			}

			keyFiles = make([]*os.File, *splitKeyCount)
			for i := 0; i < *splitKeyCount; i++ {
				keyFiles[i], err = os.Create(splitTail[i])
				errorFatal("error while opening output file", err)
			}
		} else {
			if !*splitForceRewrite {
				for i := 0; i < *splitKeyCount; i++ {
					fileName := fmt.Sprintf("%s.key%d", splitFileName, i)
					if fileExists(fileName) {
						log.Fatalf("file %s already exists. Use -f to force rewriting", fileName)
					}
				}
			}
			keyFiles = make([]*os.File, *splitKeyCount)
			for i := 0; i < *splitKeyCount; i++ {
				fileName := fmt.Sprintf("%s.key%d", splitFileName, i)
				keyFiles[i], err = os.Create(fileName)
				errorFatal("error while opening output file", err)
			}
		}

		defer func() {
			file.Close()
			for _, tempFile := range keyFiles {
				tempFile.Close()
			}
		}()

		rand.Seed(getSeed())
		err = splitIntoFiles(file, keyFiles)
		errorFatal("error while splitting", err)

	case "join":
		joinMode.Parse(os.Args[2:])
		joinTail := joinMode.Args()
		if isFlagPassed("config") {
			file, keyFiles, err := openViaInfo(*joinConfig)
			errorFatal("error while opening via config", err)

			err = joinFromFiles(file, keyFiles)
			errorFatal("error while joining", err)
		} else {
			if len(joinTail) == 0 {
				log.Fatal("error: no files specified")
			}
			joinOutput, joinTail := joinTail[0], joinTail[1:]
			if joinOutput == "" {
				log.Fatal("no output file given")
			}
			file, err := os.Create(joinOutput)
			if err != nil {
				log.Fatal("error while opening output: " + err.Error())
			}
			keyFiles := make([]*os.File, len(joinTail))
			for i, keyName := range joinTail {
				keyFiles[i], err = os.Open(keyName)
				errorFatal("error while opening key", err)
			}

			defer func() {
				file.Close()
				for _, key := range keyFiles {
					key.Close()
				}
			}()

			err = joinFromFiles(file, keyFiles)
			errorFatal("error while joining", err)
		}

	case "encrypt":
		if len(os.Args) == 2 {
			log.Fatal("error: encryption algorithm is not specified")
		}

		switch os.Args[2] {
		case "aes":
			aesEncMode.Parse(os.Args[3:])
     		aesEncTail := aesEncMode.Args()
     		var fileName, keyFileName, outputFileName string

     		// checking various conditions
			if len(aesEncTail) < 1 {
				log.Fatal("error: no input file given")
			}
			if len(aesEncTail) < 2 {
				if *aesEncRewrite {
					log.Fatal("error: no key file given")
				} else {
					log.Fatal("error: no output file given")
				}
			}
			if (!*aesEncRewrite) && len(aesEncTail) < 3 {
				log.Fatal("error: no key file given")
			}

			fileName = aesEncTail[0]
			if *aesEncRewrite {
				keyFileName = aesEncTail[1]
				outputFileName = fileName
			} else {
				outputFileName = aesEncTail[1]
				keyFileName = aesEncTail[2]
			}

			if !*aesEncForce && fileExists(keyFileName) {
				askForRewrite(keyFileName)
			}
			if !*aesEncForce && !*aesEncRewrite && fileExists(outputFileName) {
				askForRewrite(outputFileName)
			}

			// encrypting
			var key []byte
			if isFlagPassed("key") {
				var err error
				key, err = hex.DecodeString(*aesEncKey)
				errorFatal("error: invalid hex key", err)
			} else {
				key = make([]byte, 32)
				rand.Seed(getSeed())
				rand.Read(key)
			}

			file, err := os.Open(fileName)
			errorFatal("error while opening input file", err)

			keyFile, err := os.Create(keyFileName)
			errorFatal("error while creating key file", err)

			defer keyFile.Close()

			var buf bytes.Buffer
			err = aesGCMEncrypt(file, &buf, key)
			errorFatal("error while encrypting", err)

			file.Close()
			if *aesEncHex {
				hexString := hex.EncodeToString(key)
				fmt.Fprint(keyFile, hexString)
			} else {
				keyFile.Write(key)
			}

			file, err = os.Create(outputFileName)
			errorFatal("error while writing file", err)

			file.Write(buf.Bytes())

		default:
			log.Println("error: unknown encryption type")
			os.Exit(1)
		}

	case "decrypt":
		if len(os.Args) == 2 {
			log.Fatal("error: decryption algorithm is not specified")
		}

		switch os.Args[2] {
		case "aes":
    		aesDecMode.Parse(os.Args[3:])
    		aesDecTail := aesDecMode.Args()
    		keyPassed := isFlagPassed("key")

			var fileName, outputFileName, keyFileName string

    		if len(aesDecTail) < 1 {
    			log.Fatal("no input file given")
			}
			fileName, aesDecTail = aesDecTail[0], aesDecTail[1:]

			if !*aesDecRewrite && len(aesDecTail) < 1 {
				log.Fatal("no output file given")
			}
			if !*aesDecRewrite {
				outputFileName, aesDecTail = aesDecTail[0], aesDecTail[1:]
			} else {
				outputFileName = fileName
			}

			if !keyPassed && len(aesDecTail) < 1 {
				log.Fatal("no key file given")
			}
			if !keyPassed {
				keyFileName = aesDecTail[0]
			}

			var key []byte
			var err error
			if keyPassed {
				key, err = hex.DecodeString(*aesDecKey)
				errorFatal("error while decoding key", err)
			} else {
				key, err = ioutil.ReadFile(keyFileName)
				errorFatal("error while reading key", err)

				if *aesDecHex {
					keyBuf := make([]byte, hex.DecodedLen(len(key)))
					_, err := hex.Decode(keyBuf, key)
					key = keyBuf
					errorFatal("error while converting hex key", err)
				}
			}

			file, err := os.Open(fileName)
			errorFatal("error while opening input file", err)

			var buf bytes.Buffer
			err = aesGCMDecrypt(file, &buf, key)
			errorFatal("error while decrypting", err)

			_ = file.Close()
			if !*aesDecForce && !*aesDecRewrite && fileExists(outputFileName) {
				askForRewrite(outputFileName)
			}
			file, err = os.Create(outputFileName)
			errorFatal("error while creating output file", err)

			_, err = file.Write(buf.Bytes())
			errorFatal("error while writing to output", err)
			_ = file.Close()


		default:
			log.Fatal("error: unknown decryption type")
		}


	default:
		log.Printf("Unknown command %s. Visit github.com/imobulus/bitsplit or use --help for help\n", os.Args[1])
	}
}

func main() {
	runCommandLine()
}

