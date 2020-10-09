package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/imobulus/bitsplit"
	"github.com/imobulus/bitsplit/osutil"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
)

var (
	errLog = log.New(os.Stderr, "error: ", 0)
	stdLog = log.New(os.Stdout, "", 0)
)

//---- small convenient functions ----
func errorFatal(message string, err error) {
	if err != nil {
		errLog.Println(message)
		errLog.Println(err)
		os.Exit(1)
	}
}

func askForRewrite(fileName string) {
	if ! osutil.Promptf("file %s already exists, do you want to overwrite it?", fileName) {
		errLog.Fatal("error: file already exists")
	}
}

//---- command line executives ----
func OpenViaInfo(infFileName string) (*os.File, []*os.File, error) {
	infoBytes, err := ioutil.ReadFile(infFileName)
	if err != nil {
		errLog.Println("reading config file")
		return nil, nil, err
	}
	info := string(infoBytes)
	lines := strings.Split(info, "\n")
	fileName := lines[0]
	keyFileNames := lines[1:]

	file, err := os.Create(fileName)
	if err != nil {
		errLog.Println("creating output")
		return nil, nil, err
	}

	keyFiles := make([]*os.File, 0)
	for _, keyName := range keyFileNames {
		if keyName == "" {
			continue
		}
		keyFile, err := os.Open(keyName)
		if err != nil {
			errLog.Println("opening keys")
			return nil, nil, err
		}
		keyFiles = append(keyFiles, keyFile)
	}

	return file, keyFiles, nil
}

func DoSplit(args []string) {
	splitMode := flag.NewFlagSet("split", flag.ExitOnError)
	splitKeyCount := splitMode.Int("k", 2, "the number of summons file will be split to")
	splitForceRewrite := splitMode.Bool("f", false, "force rewriting key files")

	splitMode.Parse(args)
	splitTail := splitMode.Args()
	splitCountProvided := osutil.IsFlagPassed("k")

	if len(splitTail) == 0 {
		errLog.Fatal("no specification given")
	}
	splitFileName, splitTail := splitTail[0], splitTail[1:]

	if splitFileName == "" {
		errLog.Fatal("file not given")
	}

	file, err := os.Open(splitFileName)
	errorFatal("while opening input file", err)

	var keyFiles []*os.File

	if len(splitTail) > 0 {
		if !splitCountProvided {
			*splitKeyCount = len(splitTail)
		}
		if len(splitTail) < *splitKeyCount {
			errLog.Fatal("provided less key files than keys stated")
		}

		if !*splitForceRewrite {
			for i := 0; i < *splitKeyCount; i++ {
				if osutil.FileExists(splitTail[i]) {
					askForRewrite(splitTail[i])
				}
			}
		}

		keyFiles = make([]*os.File, *splitKeyCount)
		for i := 0; i < *splitKeyCount; i++ {
			keyFiles[i], err = os.Create(splitTail[i])
			errorFatal("while creating output file", err)
		}
	} else {
		if !*splitForceRewrite {
			for i := 0; i < *splitKeyCount; i++ {
				fileName := fmt.Sprintf("%s.key%d", splitFileName, i)
				if osutil.FileExists(fileName) {
					askForRewrite(fileName)
				}
			}
		}
		keyFiles = make([]*os.File, *splitKeyCount)
		for i := 0; i < *splitKeyCount; i++ {
			fileName := fmt.Sprintf("%s.key%d", splitFileName, i)
			keyFiles[i], err = os.Create(fileName)
			errorFatal("while opening output file", err)
		}
	}

	defer func() {
		file.Close()
		for _, tempFile := range keyFiles {
			tempFile.Close()
		}
	}()

	rand.Seed(bitsplit.GetSeed())
	err = bitsplit.SplitIntoFiles(file, keyFiles)
	errorFatal("while splitting", err)
}

func DoJoin(args []string) {
	joinMode := flag.NewFlagSet("join", flag.ExitOnError)
	joinConfig := joinMode.String("config", "",
		"configuration file with the output file and list of keys, optional")

	joinMode.Parse(args)
	joinTail := joinMode.Args()
	if osutil.IsFlagPassed("config") {
		file, keyFiles, err := OpenViaInfo(*joinConfig)
		errorFatal("while opening via config", err)
		defer func() {
			file.Close()
			for _, key := range keyFiles {
				key.Close()
			}
		}()

		err = bitsplit.JoinFromFiles(file, keyFiles)
		errorFatal("while joining", err)
	} else {
		if len(joinTail) == 0 {
			errLog.Fatal("no files specified")
		}
		joinOutput, joinTail := joinTail[0], joinTail[1:]
		if joinOutput == "" {
			errLog.Fatal("no output file given")
		}
		file, err := os.Create(joinOutput)
		if err != nil {
			errLog.Fatal("while opening output: " + err.Error())
		}
		keyFiles := make([]*os.File, len(joinTail))
		for i, keyName := range joinTail {
			keyFiles[i], err = os.Open(keyName)
			errorFatal("while opening key", err)
		}

		defer func() {
			file.Close()
			for _, key := range keyFiles {
				key.Close()
			}
		}()

		err = bitsplit.JoinFromFiles(file, keyFiles)
		errorFatal("while joining", err)
	}

}

func DoEncryptAES(args []string) {
	aesEncMode := flag.NewFlagSet("encrypt-aes", flag.ExitOnError)
	aesEncKey := aesEncMode.String("key", "", "AES key in hex format")
	aesEncRewrite := aesEncMode.Bool("r", false, "use this flag to rewrite input file with encrypted data")
	aesEncForce := aesEncMode.Bool("f", false, "use this flag to force rewriting")
	aesEncHex := aesEncMode.Bool("hex", false, "use this flag to save key in hex representation")
	aesEncReuse := aesEncMode.Bool("reuse-key", false,
		"this flag uses key saved in <key file> if it exists. It does nothing when -key is specified")

	aesEncMode.Parse(args)
	aesEncTail := aesEncMode.Args()
	var fileName, keyFileName, outputFileName string

	// checking various conditions
	if len(aesEncTail) < 1 {
		errLog.Fatal("no input file given")
	}
	if len(aesEncTail) < 2 {
		if *aesEncRewrite {
			errLog.Fatal("no key file given")
		} else {
			errLog.Fatal("no output file given")
		}
	}
	if (!*aesEncRewrite) && len(aesEncTail) < 3 {
		errLog.Fatal("no key file given")
	}

	fileName = aesEncTail[0]
	if *aesEncRewrite {
		keyFileName = aesEncTail[1]
		outputFileName = fileName
	} else {
		outputFileName = aesEncTail[1]
		keyFileName = aesEncTail[2]
	}

	// encrypting
	// getting the key
	var key []byte
	var err error
	if osutil.IsFlagPassed("key") {
		key, err = hex.DecodeString(*aesEncKey)
		errorFatal("invalid hex key", err)
	} else if *aesEncReuse && osutil.FileExists(keyFileName){
		key, err = ioutil.ReadFile(keyFileName)
		errorFatal("while reading key file", err)

		if *aesEncHex {
			keyBuf := make([]byte, hex.DecodedLen(len(key)))
			hex.Decode(keyBuf, key)
			key = keyBuf
		}
	} else {
		key = make([]byte, 32)
		rand.Seed(bitsplit.GetSeed())
		rand.Read(key)
	}

	// saving the key if needed
	if !*aesEncReuse || !osutil.FileExists(keyFileName) { // we need to rewrite key only if we weren't said to reuse it or
		if !*aesEncForce && osutil.FileExists(keyFileName) { // the file does not exist
			askForRewrite(keyFileName)
		}
		keyFile, err := os.Create(keyFileName)
		errorFatal("while creating key file", err)

		defer keyFile.Close()

		if *aesEncHex {
			hexString := hex.EncodeToString(key)
			fmt.Fprint(keyFile, hexString)
		} else {
			keyFile.Write(key)
		}
	}

	// getting encrypted data
	file, err := os.Open(fileName)
	errorFatal("while opening input file", err)

	var buf bytes.Buffer
	err = bitsplit.AesGCMEncrypt(file, &buf, key)
	errorFatal("while encrypting", err)
	file.Close()

	// writing encrypted data
	if !*aesEncForce && !*aesEncRewrite && osutil.FileExists(outputFileName) {
		askForRewrite(outputFileName)
	}
	file, err = os.Create(outputFileName)
	errorFatal("while writing file", err)

	_, err = file.Write(buf.Bytes())
	errorFatal("writing encrypted text", err)
	_ = file.Close()

}

func DoDecryptAES(args []string) {
	aesDecMode := flag.NewFlagSet("decrypt-aes", flag.ExitOnError)
	aesDecKey := aesDecMode.String("key", "", "AES key in hex format")
	aesDecForce := aesDecMode.Bool("f", false, "use this flag to force rewriting")
	aesDecHex := aesDecMode.Bool("hex", false, "use this flag to load key, saved in hex representation")
	aesDecRewrite := aesDecMode.Bool("r", false, "use this flag to rewrite file with decrypted data")

	aesDecMode.Parse(args)
	aesDecTail := aesDecMode.Args()
	keyPassed := osutil.IsFlagPassed("key")

	var fileName, outputFileName, keyFileName string

	if len(aesDecTail) < 1 {
		errLog.Fatal("no input file given")
	}
	fileName, aesDecTail = aesDecTail[0], aesDecTail[1:]

	if !*aesDecRewrite && len(aesDecTail) < 1 {
		errLog.Fatal("no output file given")
	}
	if !*aesDecRewrite {
		outputFileName, aesDecTail = aesDecTail[0], aesDecTail[1:]
	} else {
		outputFileName = fileName
	}

	if !keyPassed && len(aesDecTail) < 1 {
		errLog.Fatal("no key file given")
	}
	if !keyPassed {
		keyFileName = aesDecTail[0]
	}

	var key []byte
	var err error
	if keyPassed {
		key, err = hex.DecodeString(*aesDecKey)
		errorFatal("while decoding key", err)
	} else {
		key, err = ioutil.ReadFile(keyFileName)
		errorFatal("while reading key", err)

		if *aesDecHex {
			keyBuf := make([]byte, hex.DecodedLen(len(key)))
			_, err := hex.Decode(keyBuf, key)
			key = keyBuf
			errorFatal("while converting hex key", err)
		}
	}

	file, err := os.Open(fileName)
	errorFatal("while opening input file", err)

	var buf bytes.Buffer
	err = bitsplit.AesGCMDecrypt(file, &buf, key)
	errorFatal("while decrypting", err)

	_ = file.Close()
	if !*aesDecForce && !*aesDecRewrite && osutil.FileExists(outputFileName) {
		askForRewrite(outputFileName)
	}
	file, err = os.Create(outputFileName)
	errorFatal("while creating output file", err)

	_, err = file.Write(buf.Bytes())
	errorFatal("while writing to output", err)
	_ = file.Close()
}

func DoKeygen(args []string) {
	keygen := flag.NewFlagSet("keygen", flag.ExitOnError)
	keygenForce := keygen.Bool("f", false, "use this flag to force rewriting")
	keygenHex := keygen.Bool("hex", false, "use this flag to generate key in hex representation")
	keyLength := keygen.Int("l", 32, "key length, default 32")

	keygen.Parse(args)
	keygenTail := keygen.Args()

	if len(keygenTail) < 1 {
		errLog.Fatal("output file not specified")
	}
	keyFileName := keygenTail[0]

	if osutil.FileExists(keyFileName) && !*keygenForce {
		askForRewrite(keyFileName)
	}
	key := make([]byte, *keyLength)
	rand.Seed(bitsplit.GetSeed())
	rand.Read(key)
	if *keygenHex {
		keyHex := make([]byte, hex.EncodedLen(len(key)))
		hex.Encode(keyHex, key)
		key = keyHex
	}
	err := ioutil.WriteFile(keyFileName, key, 0644)
	errorFatal("couldn't write key", err)
}

func RunCommandLine() {

	if len(os.Args) == 1 {
		stdLog.Println(fmt.Sprintf("This is bitsplit %s. For documentation visit github.com/imobulus/bitsplit or use --help", bitsplit.VERSION))
		os.Exit(0)
	}

	switch os.Args[1] {

	case "split": DoSplit(os.Args[2:])
	case "join":  DoJoin(os.Args[2:])
	case "keygen": DoKeygen(os.Args[2:])

	case "encrypt":
		if len(os.Args) == 2 {
			errLog.Fatal("encryption algorithm is not specified")
		}

		switch os.Args[2] {

		case "aes": DoEncryptAES(os.Args[3:])

		default:
			errLog.Fatal("unknown encryption type")
		}

	case "decrypt":
		if len(os.Args) == 2 {
			errLog.Fatal("decryption algorithm is not specified")
		}

		switch os.Args[2] {

		case "aes": DoDecryptAES(os.Args[3:])

		default:
			errLog.Fatal("unknown decryption type")
		}


	default:
		stdLog.Printf("Unknown command %s. Visit github.com/imobulus/bitsplit or use --help for help\n", os.Args[1])
	}
}

func main() {
	RunCommandLine()
}
