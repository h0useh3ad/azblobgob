# AzBlobGob

## Overview
AzBlobGob is a CLI tool to enumerate and download Azure Blob contents

## Install

Clone from repo
```
$ go build
```

Tested with Go version 1.23.2

## Usage 
AzBlobGob requires the -account, -containers, and -dirprefixes flags at minimum.
```
./azblobgob -h
    _          ___   _         _       ___         _
   /_\    ___ | _ ) | |  ___  | |__   / __|  ___  | |__
  / _ \  |_ / | _ \ | | / _ \ | '_ \ | (_ | / _ \ | '_ \
 /_/ \_\ /__| |___/ |_| \___/ |_.__/  \___| \___/ |_.__/
					@h0useh3ad

Usage of ./azblobgob:
  -account string
    	Azure Blob Storage account name
  -containers string
    	Container names file (default: containers.txt) (default "containers.txt")
  -dest string
    	Directory to save downloaded blobs (default: provided account name in current directory)
  -dirprefixes string
    	Directory prefix name files (default "dirprefixes.txt")
  -socks string
    	SOCKS5 proxy address (e.g., 127.0.0.1:1080)
  -verbose
    	Enable verbose output
  -version
    	Display version information
```

### Account Name
The account name is the 'myaccount' name from the blob URI prefix 'https://myaccount.blob.core.windows.net/mycontainer/myblob'. Only provide the account name, not the full URI.

### Containers and Directory Prefixes
The containers and directory prefix files are new-line delimited files used to enumerate the Azure blobs. Included is a default containers file cloned from NetSpi's [MicroBurst](https://github.com/NetSPI/MicroBurst) [permutations.txt](https://github.com/NetSPI/MicroBurst/blob/master/Misc/permutations.txt) file.