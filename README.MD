This is a simple command line tool to split files into random per-byte summons.

Splitting:
* Usage: `bitsplit split <flags> <file names>`
* `-k <int>` the number of summon files you wish to have, must be at least 2
* The first file name is the file to split and is mandatory. If additional file names are not given they are assigned by default. If they are given there must be at least `-k` of them

Joining:
* Usage: `bitsplit join <flags> <file names>`
* `-config <config file name>` program will be initialized with config file, which should contain the output file name and names of key files. If this flag is present everything else will be ignored.
* If `-config` is not used or empty, the first file name is mandatory and is the output file, the next names are files to join
