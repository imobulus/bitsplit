This go package provides functions for encryption and splitting arrays into rndom summons.
Generally, the use is pretty strainghtforward - extract data from `io.Reader` argument, make operations, write data to `io.Writer` argument(s) or vice versa.
Encryption functions also require a `[]byte` key wich should have particular length for each algorhithm (otherwise will return IOError).

`getSeed` is used to get random seed for `math/rand` as the sum of current time and random number from [random.org](https://random.org)

In `examples/cmdInterface` you can find command line tool based on this package. The usage is as follows

Splitting:
* Usage: `bitsplit split <flags> <input file> <output files>`
* `-k <int>` the number of summon files you wish to have, must be at least 2
* The first file name is mandatory. If additional file names are not given they are assigned by default. If they are given there must be at least `-k` of them

Joining:
* Usage: `bitsplit join <flags> <output file> <key files>`
* `-config <config file>` program will be initialized with config file, which should contain the output file name and names of key files. If this flag is present everything else will be ignored.
* Without `-config` the `<output file>` is mandatory

Keygen:
* Usage: `bitsplit keygen <flags> <key file>`
* `-l <int>` byte length of the key. Default 32
* `-f` force rewriting of `<key file>`
* `-hex` save key in hex representation

Encrypting via AES:
* Usage: `bitsplit encrypt aes <flags> (input file) (output file) (key file)`
* `-key <string>` key in hex format
* `-r` input file will be replaced with encrypted version. `(output file)` is not provided with this flag
* `-f` force overwriting
* `-hex` save key in hex representation
* `-reuse-key` checks if `(key file)` exists, and then uses the key from the file or generates new key and writes it to the file. Does nothing if `-key` is specified. Useful for encrypting multiple files. 

Decrypting via AES:
* Usage: `bitsplit decrypt aes <flags> (input file) (output file) (key file)`
* `-key <string>` key in hex format. `(key file)` is not provided with this flag
* `-r` input file will be replaced with decrypted version. `(output file)` is not provided with this flag
* `-f` force overwriting
* `-hex` load key in hex representation