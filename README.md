DBLYZER
==

Fast tool identifies technologies, favicon, domains,... on websites

![Alt Text](https://s3.gifyu.com/images/dblyzer.gif)

BUILD
--
~#: sudo apt install libprce3-dev

~#: go build -o releases/dblyzer cmd/main.go

RUN
--
Use [dbgrab](https://github.com/lochv/dbgrab) 's output file as input file.

~#: ./dblyzer


REF
--
https://github.com/glenn-brown/golang-pkg-pcre

https://github.com/rverton/webanalyze

