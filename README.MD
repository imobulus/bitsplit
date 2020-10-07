This is a simple command line tool to split files into random per-byte summons.
Random seed for `math/rand` is the sum of current time and random number from [random.org](https://random.org)

Splitting:
* Usage: `bitsplit split <flags> <input file> <output files>`
* `-k <int>` the number of summon files you wish to have, must be at least 2
* The first file name is mandatory. If additional file names are not given they are assigned by default. If they are given there must be at least `-k` of them

Joining:
* Usage: `bitsplit join <flags> <output file> <key files>`
* `-config <config file>` program will be initialized with config file, which should contain the output file name and names of key files. If this flag is present everything else will be ignored.
* Without `-config` the `<output file>` is mandatory

Encrypting via AES:
* Usage: `bitsplit encrypt aes <flags> (input file) (output file) (key file)`
* `-key <string>` key in hex format
* `-r` input file will be replaced with encrypted version. Output file is not provided with this flag
* `-f` force overwriting
* `-hex` save key in hex representation

Decrypting via AES:
* Usage: `bitsplit decrypt aes <flags> (input file) (output file) (key file)`
* `-key <string>` key in hex format. `(key file)` is not provided with this flag
* `-r` input file will be replaced with decrypted version. `(output file)` is not provided with this flag
* `-f` force overwriting
* `-hex` load key in hex representation