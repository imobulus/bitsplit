package bitsplit

import (
	"crypto/aes"
	"crypto/cipher"
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

const (
	VERSION= "v0.0.4"
)
var (
	warnLog = log.New(os.Stderr, "warning: ", 0)
)

type IOError struct {
	details  string
	contents error
}

func (err IOError) Error() string {
	return err.details + "\n" + err.contents.Error()
}

//---- useful functions ----
func GetSeed() int64 {
	seed := time.Now().UTC().UnixNano()

	addRandOrg := func() {
		resp, err := http.Get("http://www.random.org/integers/?num=4&min=0&max=65535&col=1&base=10&format=plain&rnd=new")
		if err != nil {
			warnLog.Println("no response from random.org\n" + err.Error())
			return
		}
		numBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			warnLog.Println("warning: reading response from random.org\n" + err.Error())
			return
		}

		numStrings := strings.Split(string(numBytes), "\n")
		toAdd := int64(0)
		for _, ns := range numStrings[:4] {
			n, err := strconv.ParseInt(ns, 10, 64)
			if err != nil {
				warnLog.Println("warning: cannot parse string to int\n" + err.Error())
				return
			}
			toAdd *= 65536
			toAdd += n
		}
		seed += toAdd
		//stdLog.Println("using numbers from random.org")
	}
	addRandOrg()

	return seed
}

func Split(file io.Reader, keys []io.Writer) error {
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return IOError{"while reading file contents", err}
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

		_, err = keys[0].Write([]byte{b - sum})
		if err != nil {
			return IOError{"while writing keys", err}
		}
        for i, key := range keys[1:] {
			_, err = key.Write([]byte{rands[i]})
			if err != nil {
				return IOError{"while writing keys", err}
			}
		}
	}
	return nil
}

func SplitIntoFiles(file io.Reader, keys []*os.File) error {
	keyWriters := make([]io.Writer, len(keys))
	for i, key := range keys {
		keyWriters[i] = key
	}
	return Split(file, keyWriters)
}

func Join(file io.Writer, keys []io.Reader) error {
	l := len(keys)
	if l < 2 {
		return fmt.Errorf("less than 2 key files provided")
	}
    contents := make([][]byte, l)
    var err error

    for i, key := range keys {
    	contents[i], err = ioutil.ReadAll(key)
    	if err != nil {
    		return IOError{"while reading key", err}
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
			return IOError{"writing result", err}
		}
	}
	return nil
}

func JoinFromFiles(file io.Writer, keys []*os.File) error {
	keyWriters := make([]io.Reader, len(keys))
	for i, key := range keys {
		keyWriters[i] = key
	}
	return Join(file, keyWriters)
}

func AesGCMEncrypt(file io.Reader, output io.Writer, key []byte) error {
	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		return IOError{"while reading contents of encrypted file", err}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return IOError{"while creating cipher block", err}
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return IOError{"while creating gcm encryption", err}
	}

	nonce := make([]byte, aesGCM.NonceSize())
	rand.Read(nonce)

	encryptedData := aesGCM.Seal(nonce, nonce, fileContents, nil)
	_, err = output.Write(encryptedData)
	if err != nil {
		return IOError{"while writing encrypted data", err}
	}
	return nil
}

func AesGCMDecrypt(file io.Reader, output io.Writer, key []byte) error {
    c, err := aes.NewCipher(key)
    if err != nil {
    	return IOError{"while creating decryption cipher", err}
	}

	aesGCM, err := cipher.NewGCM(c)
	if err != nil {
		return IOError{"while creating decryption GCM", err}
	}

	nonceSize := aesGCM.NonceSize()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return IOError{"while reading the file", err}
	}

	if len(data) < nonceSize {
		return fmt.Errorf("encrypted file must be bigger than nonce size")
	}

	nonce, data := data[:nonceSize], data[nonceSize:]
	decrypted, err := aesGCM.Open(nil, nonce, data, nil)
	if err != nil {
		return IOError{"while decrypting", err}
	}

	_, err = output.Write(decrypted)
	if err != nil {
		return IOError{"while writing output", err}
	}
	return nil
}
